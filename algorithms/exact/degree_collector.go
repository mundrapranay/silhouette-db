package exact

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mundrapranay/silhouette-db/algorithms/common"
	"github.com/mundrapranay/silhouette-db/pkg/client"
)

// DegreeCollector is a 2-round algorithm that collects vertex degrees.
// Round 1: Each worker publishes (vertex_id, degree) for its assigned vertices.
// Round 2: Each worker queries degrees of its neighbors and writes results to file.
type DegreeCollector struct {
	graphData *common.GraphData
	config    map[string]interface{}

	// Worker assignment
	workerID     string
	numWorkers   int
	vertexAssign map[int]string // vertex -> worker assignment
	myVertices   []int          // Vertices assigned to this worker

	// Results storage
	resultFile string
	mu         sync.Mutex
	results    []DegreeResult // Results to write to file
}

// DegreeResult represents a single result: (vertex, neighbor, neighbor_degree)
type DegreeResult struct {
	Vertex         int
	Neighbor       int
	NeighborDegree int
}

// NewDegreeCollector creates a new degree collector algorithm instance
func NewDegreeCollector() common.GraphAlgorithm {
	return &DegreeCollector{}
}

func (a *DegreeCollector) Name() string {
	return "degree-collector"
}

func (a *DegreeCollector) Type() common.AlgorithmType {
	return common.AlgorithmTypeExact
}

func (a *DegreeCollector) Initialize(ctx context.Context, graphData *common.GraphData, config map[string]interface{}) error {
	a.graphData = graphData
	a.config = config

	// Get worker configuration
	if workerID, ok := config["worker_id"].(string); ok {
		a.workerID = workerID
	} else {
		return fmt.Errorf("worker_id not found in config")
	}

	if numWorkers, ok := config["num_workers"].(int); ok {
		a.numWorkers = numWorkers
	} else {
		return fmt.Errorf("num_workers not found in config")
	}

	// Get result file path
	if resultFile, ok := config["result_file"].(string); ok {
		a.resultFile = resultFile
	} else {
		// Default result file
		a.resultFile = fmt.Sprintf("degree_collector_results_%s.txt", a.workerID)
	}

	// Assign vertices to workers
	a.vertexAssign = make(map[int]string)
	a.myVertices = []int{}

	// Get custom vertex assignment if provided
	var customAssign map[int]string
	if assign, ok := config["vertex_assignment"].(map[string]interface{}); ok {
		customAssign = make(map[int]string)
		for k, v := range assign {
			var vertexID int
			if _, err := fmt.Sscanf(k, "%d", &vertexID); err == nil {
				if workerStr, ok := v.(string); ok {
					customAssign[vertexID] = workerStr
				}
			}
		}
	}

	// Build adjacency map for degree computation
	adjacencyMap := make(map[int][]int)
	vertexSet := make(map[int]bool)

	for _, edge := range graphData.Edges {
		adjacencyMap[edge.U] = append(adjacencyMap[edge.U], edge.V)
		vertexSet[edge.U] = true
		vertexSet[edge.V] = true
	}

	// Assign vertices to workers
	for vertexID := range vertexSet {
		var assignedWorker string
		if customAssign != nil {
			if w, exists := customAssign[vertexID]; exists {
				assignedWorker = w
			} else {
				assignedWorker = common.GetVertexAssignment(vertexID, a.numWorkers, customAssign)
			}
		} else {
			assignedWorker = common.GetVertexAssignment(vertexID, a.numWorkers, nil)
		}

		a.vertexAssign[vertexID] = assignedWorker

		if assignedWorker == a.workerID {
			a.myVertices = append(a.myVertices, vertexID)
		}
	}

	return nil
}

func (a *DegreeCollector) Execute(ctx context.Context, dbClient *client.Client, numRounds int) (*common.AlgorithmResult, error) {
	if numRounds < 2 {
		return nil, fmt.Errorf("degree-collector requires at least 2 rounds")
	}

	// Round 1: Publish degrees for assigned vertices
	err := a.executeRound1(ctx, dbClient)
	if err != nil {
		return nil, fmt.Errorf("round 1 failed: %w", err)
	}

	// Round 2: Query neighbor degrees and write results
	err = a.executeRound2(ctx, dbClient)
	if err != nil {
		return nil, fmt.Errorf("round 2 failed: %w", err)
	}

	// Write results to file
	err = a.writeResults()
	if err != nil {
		return nil, fmt.Errorf("failed to write results: %w", err)
	}

	return a.GetResult(), nil
}

// executeRound1 publishes (vertex_id, degree) pairs for this worker's assigned vertices
func (a *DegreeCollector) executeRound1(ctx context.Context, dbClient *client.Client) error {
	roundID := uint64(1)

	// Start round 1
	err := dbClient.StartRound(ctx, roundID, int32(a.numWorkers))
	if err != nil {
		return fmt.Errorf("failed to start round 1: %w", err)
	}

	// Build adjacency map for degree computation
	adjacencyMap := make(map[int][]int)
	for _, edge := range a.graphData.Edges {
		adjacencyMap[edge.U] = append(adjacencyMap[edge.U], edge.V)
	}

	// Compute degrees for this worker's vertices
	pairs := make(map[string][]byte)
	for _, vertexID := range a.myVertices {
		degree := len(adjacencyMap[vertexID])
		key := fmt.Sprintf("vertex-%d", vertexID)

		// Encode degree as 8 bytes (uint64)
		value := make([]byte, 8)
		binary.LittleEndian.PutUint64(value, uint64(degree))
		pairs[key] = value
	}

	// Publish values (synchronization handled by silhouette-db)
	// When this returns, the worker's data is published
	// The last worker's call will complete the round and make data available
	err = dbClient.PublishValues(ctx, roundID, a.workerID, pairs)
	if err != nil {
		return fmt.Errorf("failed to publish values in round 1: %w", err)
	}

	// Wait for round to complete: try to query a key to verify round is ready
	// Since we don't know if we're the last worker, we poll by trying to initialize PIR client
	// The PIR client initialization will fail if the round isn't complete yet
	maxRetries := 100
	retryDelay := 50 // milliseconds
	for retry := 0; retry < maxRetries; retry++ {
		err = dbClient.InitializePIRClient(ctx, roundID)
		if err == nil {
			// Round is complete, PIR client initialized successfully
			break
		}
		if retry < maxRetries-1 {
			// Wait a bit before retrying (simple sleep, in production use context-aware wait)
			time.Sleep(time.Duration(retryDelay) * time.Millisecond)
		}
	}
	if err != nil {
		return fmt.Errorf("round 1 did not complete after %d retries: %w", maxRetries, err)
	}

	return nil
}

// executeRound2 queries degrees of neighbors and stores results
func (a *DegreeCollector) executeRound2(ctx context.Context, dbClient *client.Client) error {
	roundID := uint64(2)

	// Start round 2
	err := dbClient.StartRound(ctx, roundID, int32(a.numWorkers))
	if err != nil {
		return fmt.Errorf("failed to start round 2: %w", err)
	}

	// Build adjacency map to find neighbors
	adjacencyMap := make(map[int][]int)
	for _, edge := range a.graphData.Edges {
		adjacencyMap[edge.U] = append(adjacencyMap[edge.U], edge.V)
	}

	// For each of this worker's vertices, query degrees of its neighbors
	a.results = []DegreeResult{}
	for _, vertexID := range a.myVertices {
		neighbors := adjacencyMap[vertexID]
		for _, neighborID := range neighbors {
			// Query the neighbor's degree from round 1
			key := fmt.Sprintf("vertex-%d", neighborID)
			valueBytes, err := dbClient.GetValue(ctx, uint64(1), key)
			if err != nil {
				return fmt.Errorf("failed to get degree for vertex %d (neighbor of %d): %w", neighborID, vertexID, err)
			}

			// Decode degree
			if len(valueBytes) < 8 {
				return fmt.Errorf("invalid degree value for vertex %d: expected 8 bytes, got %d", neighborID, len(valueBytes))
			}

			neighborDegree := binary.LittleEndian.Uint64(valueBytes[:8])

			// Store result
			a.mu.Lock()
			a.results = append(a.results, DegreeResult{
				Vertex:         vertexID,
				Neighbor:       neighborID,
				NeighborDegree: int(neighborDegree),
			})
			a.mu.Unlock()
		}
	}

	// Publish empty values for round 2 (to complete synchronization)
	// Since we're just querying in round 2, we publish empty pairs
	err = dbClient.PublishValues(ctx, roundID, a.workerID, map[string][]byte{})
	if err != nil {
		return fmt.Errorf("failed to publish values in round 2: %w", err)
	}

	return nil
}

// writeResults writes the results to a file
func (a *DegreeCollector) writeResults() error {
	file, err := os.Create(a.resultFile)
	if err != nil {
		return fmt.Errorf("failed to create result file: %w", err)
	}
	defer file.Close()

	// Write header
	_, err = fmt.Fprintf(file, "# Degree Collector Results (Worker: %s)\n", a.workerID)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "# Format: vertex_id neighbor_id neighbor_degree\n")
	if err != nil {
		return err
	}

	// Write results
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, result := range a.results {
		_, err = fmt.Fprintf(file, "%d %d %d\n", result.Vertex, result.Neighbor, result.NeighborDegree)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *DegreeCollector) GetRoundData(roundID int) *common.RoundData {
	switch roundID {
	case 1:
		// Round 1: Publish degrees
		publishKeys := []string{}
		adjacencyMap := make(map[int][]int)
		for _, edge := range a.graphData.Edges {
			adjacencyMap[edge.U] = append(adjacencyMap[edge.U], edge.V)
		}
		for _, vertexID := range a.myVertices {
			publishKeys = append(publishKeys, fmt.Sprintf("vertex-%d", vertexID))
		}
		return &common.RoundData{
			RoundID:         roundID,
			ExpectedWorkers: int32(a.numWorkers),
			PublishKeys:     publishKeys,
			QueryKeys:       []string{}, // No queries in round 1
			Metadata: map[string]interface{}{
				"round_type": "publish",
			},
		}
	case 2:
		// Round 2: Query neighbor degrees
		queryKeys := []string{}
		adjacencyMap := make(map[int][]int)
		for _, edge := range a.graphData.Edges {
			adjacencyMap[edge.U] = append(adjacencyMap[edge.U], edge.V)
		}
		for _, vertexID := range a.myVertices {
			neighbors := adjacencyMap[vertexID]
			for _, neighborID := range neighbors {
				queryKeys = append(queryKeys, fmt.Sprintf("vertex-%d", neighborID))
			}
		}
		return &common.RoundData{
			RoundID:         roundID,
			ExpectedWorkers: int32(a.numWorkers),
			PublishKeys:     []string{}, // No publishing in round 2 (or empty publish)
			QueryKeys:       queryKeys,
			Metadata: map[string]interface{}{
				"round_type": "query",
			},
		}
	default:
		return &common.RoundData{
			RoundID:         roundID,
			ExpectedWorkers: int32(a.numWorkers),
			Metadata:        make(map[string]interface{}),
		}
	}
}

func (a *DegreeCollector) ProcessRound(roundID int, roundResults map[string]interface{}) error {
	// For degree-collector, we process results in executeRound2
	// This method can be used for additional processing if needed
	return nil
}

func (a *DegreeCollector) GetResult() *common.AlgorithmResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	return &common.AlgorithmResult{
		AlgorithmName:    a.Name(),
		NumRounds:        2,
		Converged:        true,
		ConvergenceRound: 2,
		Results: map[string]interface{}{
			"num_results":     len(a.results),
			"result_file":     a.resultFile,
			"my_vertices":     a.myVertices,
			"num_my_vertices": len(a.myVertices),
		},
		Metadata: map[string]interface{}{
			"worker_id":   a.workerID,
			"num_workers": a.numWorkers,
		},
	}
}

// Register the algorithm
func init() {
	Register("degree-collector", NewDegreeCollector)
}

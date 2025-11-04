// (moved executeRoundSingle below imports)
package ledp

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"sync"
	"time"

	"github.com/mundrapranay/silhouette-db/algorithms/common"
	"github.com/mundrapranay/silhouette-db/algorithms/noise"
	"github.com/mundrapranay/silhouette-db/pkg/client"
)

// KCoreVertex represents a vertex in the k-core decomposition algorithm
type KCoreVertex struct {
	id              int
	current_level   uint
	next_level      int
	permanent_zero  int
	round_threshold int
	neighbours      []int
}

// KCoreDecomposition implements the LEDP k-core decomposition algorithm
type KCoreDecomposition struct {
	graphData *common.GraphData
	config    map[string]interface{}

	// Worker info
	workerID     string
	numWorkers   int
	myVertices   []int
	vertexAssign map[int]string

	// Algorithm parameters
	n                       int
	psi                     float64
	epsilon                 float64
	factor                  float64
	lambda                  float64
	levels_per_group        float64
	number_of_rounds        int // Agreed upon in round 0
	maxPublicRoundThreshold int // From initial exchange

	// Vertex state (local computation only, not stored)
	vertices map[int]*KCoreVertex

	// Algorithm constants
	super_step1_geom_factor float64
	super_step2_geom_factor float64
	bias                    bool
	bias_factor             int
	noise                   bool

	// Results
	resultFile string
	mu         sync.Mutex
	results    map[int]float64 // vertex_id -> estimated core number
}

// NewKCoreDecomposition creates a new k-core decomposition algorithm instance
func NewKCoreDecomposition() common.GraphAlgorithm {
	return &KCoreDecomposition{}
}

func (a *KCoreDecomposition) Name() string {
	return "kcore-decomposition"
}

func (a *KCoreDecomposition) Type() common.AlgorithmType {
	return common.AlgorithmTypeLEDP
}

func (a *KCoreDecomposition) Initialize(ctx context.Context, graphData *common.GraphData, config map[string]interface{}) error {
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

	// Get algorithm parameters (preserve original naming)
	if n, ok := config["n"].(int); ok {
		a.n = n
	} else {
		return fmt.Errorf("n (number of vertices) not found in config")
	}

	if psi, ok := config["psi"].(float64); ok {
		a.psi = psi
	} else {
		return fmt.Errorf("psi not found in config")
	}

	if epsilon, ok := config["epsilon"].(float64); ok {
		a.epsilon = epsilon
	} else {
		return fmt.Errorf("epsilon not found in config")
	}

	if factor, ok := config["factor"].(float64); ok {
		a.factor = factor
	} else {
		return fmt.Errorf("factor not found in config")
	}

	if bias, ok := config["bias"].(bool); ok {
		a.bias = bias
	} else {
		a.bias = false
	}

	if biasFactor, ok := config["bias_factor"].(int); ok {
		a.bias_factor = biasFactor
	} else {
		a.bias_factor = 0
	}

	if noise, ok := config["noise"].(bool); ok {
		a.noise = noise
	} else {
		a.noise = true // Default to true for LEDP
	}

	// Compute levels_per_group (same formula as original)
	a.levels_per_group = math.Ceil(log_a_to_base_b(a.n, 1.0+a.psi)) / 4.0

	// Compute rounds_param (same formula as original)
	rounds_param := math.Ceil(4.0 * math.Pow(log_a_to_base_b(a.n, 1.0+a.psi), 1.2))
	a.number_of_rounds = int(rounds_param)

	// Compute geometric factors (same as original)
	a.super_step1_geom_factor = a.epsilon * a.factor
	a.super_step2_geom_factor = a.epsilon * (1.0 - a.factor)

	// Lambda for core number estimation (same as original KCoreLDPCoord)
	a.lambda = 0.5

	// Get result file
	if resultFile, ok := config["result_file"].(string); ok {
		a.resultFile = resultFile
	} else {
		a.resultFile = fmt.Sprintf("kcore_results_%s.txt", a.workerID)
	}

	// Initialize results map
	a.results = make(map[int]float64)

	// Assign vertices to workers
	a.vertexAssign = make(map[int]string)
	a.myVertices = []int{}

	// Get vertex assignment from config if provided
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

	// Build adjacency map and assign vertices
	adjacencyMap := make(map[int][]int)
	vertexSet := make(map[int]bool)

	for _, edge := range graphData.Edges {
		adjacencyMap[edge.U] = append(adjacencyMap[edge.U], edge.V)
		vertexSet[edge.U] = true
		vertexSet[edge.V] = true
	}

	// Initialize vertices and assign to workers
	a.vertices = make(map[int]*KCoreVertex)
	for vertexID := range vertexSet {
		var assignedWorker string
		if customAssign != nil {
			if w, exists := customAssign[vertexID]; exists {
				assignedWorker = w
			} else {
				// Compute deterministically if not in config
				assignedWorker = common.GetVertexAssignment(vertexID, a.numWorkers, customAssign)
			}
		} else {
			assignedWorker = common.GetVertexAssignment(vertexID, a.numWorkers, nil)
		}

		a.vertexAssign[vertexID] = assignedWorker

		if assignedWorker == a.workerID {
			neighbours := adjacencyMap[vertexID]
			a.vertices[vertexID] = &KCoreVertex{
				id:              vertexID,
				current_level:   0,
				next_level:      0,
				permanent_zero:  1,
				round_threshold: 0, // Will be computed in round 0
				neighbours:      neighbours,
			}
			a.myVertices = append(a.myVertices, vertexID)
		}
	}

	return nil
}

// executeRoundSingle performs a single get-then-set round:
// - Reads levels from round r-1 (or defaults to 0 for r=0)
// - Computes next levels based on neighbor counts and thresholds
// - Publishes updated levels to round r
// Algorithm: In each round, read the last value before changing it (simple get and set).
// Process a node based on: if current level == threshold, stop processing.
// If we've set it to not move in any previous round (permanent_zero == 0), it won't move in subsequent rounds.
func (a *KCoreDecomposition) executeRoundSingle(ctx context.Context, dbClient *client.Client, r int) error {
	roundID := uint64(r)

	log.Printf("[%s] Round %d: Starting round", a.workerID, r)

	// Start round r
	if err := dbClient.StartRound(ctx, roundID, int32(a.numWorkers)); err != nil {
		return fmt.Errorf("failed to start round %d: %w", roundID, err)
	}

	// Previous round for reading levels
	var prevLevelRoundID uint64
	if r <= 0 {
		prevLevelRoundID = 0
	} else {
		prevLevelRoundID = uint64(r - 1)
	}

	levelUpdates := make(map[string][]byte)
	processedCount := 0
	stoppedCount := 0
	increasedCount := 0

	for _, vertexID := range a.myVertices {
		vertex := a.vertices[vertexID]

		// Read current level from previous round (or default 0)
		// This is the "get" part: read the last value before changing it
		var currentLevel uint = 0
		if r > 0 {
			levelKey := fmt.Sprintf("level-%d", vertexID)
			if levelBytes, err := dbClient.GetValue(ctx, prevLevelRoundID, levelKey); err == nil {
				currentLevel = uint(bytesToFloat64(levelBytes))
			} else {
				log.Printf("[%s] Round %d: Vertex %d: Could not read level from round %d, defaulting to 0", a.workerID, r, vertexID, prevLevelRoundID)
			}
		}

		// Check if vertex should stop processing:
		// 1. If current level == threshold, stop processing
		// 2. If permanent_zero == 0 (was set to not move in previous round), don't move in subsequent rounds
		if int(currentLevel) == vertex.round_threshold {
			log.Printf("[%s] Round %d: Vertex %d: Stopping - current level %d == threshold %d", a.workerID, r, vertexID, currentLevel, vertex.round_threshold)
			vertex.permanent_zero = 0
			levelUpdates[fmt.Sprintf("level-%d", vertexID)] = float64ToBytes(float64(currentLevel))
			stoppedCount++
			continue
		}

		if vertex.permanent_zero == 0 {
			log.Printf("[%s] Round %d: Vertex %d: Stopping - permanent_zero is 0 (was set to not move previously), current level: %d", a.workerID, r, vertexID, currentLevel)
			levelUpdates[fmt.Sprintf("level-%d", vertexID)] = float64ToBytes(float64(currentLevel))
			stoppedCount++
			continue
		}

		// Count neighbors at same level from previous round
		neighborCount := 0
		for _, neighborID := range vertex.neighbours {
			neighborKey := fmt.Sprintf("level-%d", neighborID)
			var neighborLevel uint = 0
			if r > 0 {
				if b, err := dbClient.GetValue(ctx, prevLevelRoundID, neighborKey); err == nil {
					neighborLevel = uint(bytesToFloat64(b))
				}
			}
			if neighborLevel == currentLevel {
				neighborCount++
			}
		}

		// Apply noise (same logic as original)
		noisedNeighborCount := int64(neighborCount)
		if a.noise {
			scale := a.super_step2_geom_factor / (2.0 * float64(vertex.round_threshold))
			geomDist := noise.NewGeomDistribution(scale)
			noiseSampled := geomDist.TwoSidedGeometric()
			extraBias := int64(3.0 * (2.0 * math.Exp(scale)) / math.Pow((math.Exp(2.0*scale)-1.0), 3.0))
			noisedNeighborCount += noiseSampled
			noisedNeighborCount += extraBias
		}

		// Threshold using group index derived from current level
		vGroupIdx := uint(math.Floor(float64(currentLevel) / a.levels_per_group))
		threshold := math.Pow(1.0+a.psi, float64(vGroupIdx))

		newLevel := currentLevel
		if noisedNeighborCount > int64(threshold) {
			newLevel = currentLevel + 1
			increasedCount++
			log.Printf("[%s] Round %d: Vertex %d: Level increased %d -> %d (neighbor_count: %d, noised: %d, threshold: %.2f, group_idx: %d)",
				a.workerID, r, vertexID, currentLevel, newLevel, neighborCount, noisedNeighborCount, threshold, vGroupIdx)
		} else {
			// Set permanent_zero to 0 so it won't move in subsequent rounds
			vertex.permanent_zero = 0
			log.Printf("[%s] Round %d: Vertex %d: Not increasing - neighbor_count: %d, noised: %d, threshold: %.2f (setting permanent_zero=0)",
				a.workerID, r, vertexID, neighborCount, noisedNeighborCount, threshold)
		}

		// Publish updated level for this round (this is the "set" part)
		levelUpdates[fmt.Sprintf("level-%d", vertexID)] = float64ToBytes(float64(newLevel))
		vertex.current_level = newLevel
		processedCount++
	}

	log.Printf("[%s] Round %d: Processed %d vertices, %d increased, %d stopped", a.workerID, r, processedCount, increasedCount, stoppedCount)

	// Publish levels → OKVS for round r
	if err := dbClient.PublishValues(ctx, roundID, a.workerID, levelUpdates); err != nil {
		return fmt.Errorf("failed to publish level updates: %w", err)
	}

	log.Printf("[%s] Round %d: Published %d level updates", a.workerID, r, len(levelUpdates))

	// Wait for round completion
	maxRetries := 100
	retryDelay := 50 * time.Millisecond
	for retry := 0; retry < maxRetries; retry++ {
		if err := dbClient.InitializePIRClient(ctx, roundID); err == nil {
			break
		}
		if retry < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	log.Printf("[%s] Round %d: Completed", a.workerID, r)
	return nil
}

// Execute runs the k-core decomposition algorithm
func (a *KCoreDecomposition) Execute(ctx context.Context, dbClient *client.Client, numRounds int) (*common.AlgorithmResult, error) {
	startTime := time.Now()

	// Round 0: Initial exchange - determine number of rounds
	log.Printf("[%s] Starting Round 0: Initial exchange", a.workerID)
	if err := a.executeRound0(ctx, dbClient); err != nil {
		return nil, fmt.Errorf("round 0 (initial exchange) failed: %w", err)
	}
	log.Printf("[%s] Round 0 completed: maxPublicRoundThreshold=%d, number_of_rounds=%d",
		a.workerID, a.maxPublicRoundThreshold, a.number_of_rounds)

	// Execute algorithm rounds using single get-then-set per round
	// Round r reads levels from round r-1 (or 0s for r=0) and writes updated levels to round r
	// Algorithm: Each round reads last value, processes nodes, and posts updates (simple get and set)
	algorithmRounds := min(a.number_of_rounds-2, a.maxPublicRoundThreshold)
	log.Printf("[%s] Starting algorithm rounds: %d rounds to execute", a.workerID, algorithmRounds)

	for round := 0; round < algorithmRounds; round++ {
		if err := a.executeRoundSingle(ctx, dbClient, round); err != nil {
			return nil, fmt.Errorf("round %d (single update) failed: %w", round, err)
		}
	}
	log.Printf("[%s] Completed %d algorithm rounds", a.workerID, algorithmRounds)

	// Compute final core numbers
	if err := a.computeCoreNumbers(ctx, dbClient, algorithmRounds); err != nil {
		return nil, fmt.Errorf("failed to compute core numbers: %w", err)
	}

	// Write results to file
	if err := a.writeResults(); err != nil {
		return nil, fmt.Errorf("failed to write results: %w", err)
	}

	executionTime := time.Since(startTime)

	return &common.AlgorithmResult{
		AlgorithmName:    a.Name(),
		NumRounds:        algorithmRounds,
		Converged:        true,
		ConvergenceRound: algorithmRounds,
		Results: map[string]interface{}{
			"num_vertices":     len(a.myVertices),
			"result_file":      a.resultFile,
			"algorithm_rounds": algorithmRounds,
		},
		Metadata: map[string]interface{}{
			"worker_id":              a.workerID,
			"num_workers":            a.numWorkers,
			"execution_time_seconds": executionTime.Seconds(),
		},
	}, nil
}

// executeRound0 performs the initial exchange to determine number of rounds
func (a *KCoreDecomposition) executeRound0(ctx context.Context, dbClient *client.Client) error {
	roundID := uint64(0)

	log.Printf("[%s] Round 0: Starting initial exchange", a.workerID)

	// Start round 0
	if err := dbClient.StartRound(ctx, roundID, int32(a.numWorkers)); err != nil {
		return fmt.Errorf("failed to start round 0: %w", err)
	}

	// Compute noised degrees and round thresholds for assigned vertices
	degreePairs := make(map[string][]byte)
	maxWorkerThreshold := 0

	log.Printf("[%s] Round 0: Processing %d vertices", a.workerID, len(a.myVertices))

	for _, vertexID := range a.myVertices {
		vertex := a.vertices[vertexID]

		// Compute degree
		degree := len(vertex.neighbours)
		noised_degree := int64(degree)

		// Apply noise (same logic as original)
		if a.noise {
			geomDist := noise.NewGeomDistribution(a.super_step1_geom_factor / 2.0)
			noise_sampled := geomDist.TwoSidedGeometric()
			noised_degree += noise_sampled

			if a.bias {
				biasTerm := float64(a.bias_factor) * float64((2.0*math.Exp(a.super_step1_geom_factor))/(math.Exp(2.0*a.super_step1_geom_factor)-1.0))
				noised_degree -= int64(math.Min(biasTerm, float64(noised_degree)))
			}

			// Ensure degree is at least 2
			noised_degree += 1
		}

		// Compute round threshold (same formula as original)
		threshold := math.Ceil(log_a_to_base_b(int(noised_degree), 2.0)) * a.levels_per_group
		vertex.round_threshold = int(threshold) + 1

		log.Printf("[%s] Round 0: Vertex %d: degree=%d, noised_degree=%d, round_threshold=%d",
			a.workerID, vertexID, degree, noised_degree, vertex.round_threshold)

		// Store degree in OKVS
		degreeKey := fmt.Sprintf("degree-%d", vertexID)
		degreeValue := float64ToBytes(float64(noised_degree))
		degreePairs[degreeKey] = degreeValue

		// Track max threshold
		if vertex.round_threshold > maxWorkerThreshold {
			maxWorkerThreshold = vertex.round_threshold
		}
	}

	log.Printf("[%s] Round 0: Max worker threshold: %d", a.workerID, maxWorkerThreshold)

	// Publish degrees → OKVS
	if err := dbClient.PublishValues(ctx, roundID, a.workerID, degreePairs); err != nil {
		return fmt.Errorf("failed to publish degrees: %w", err)
	}

	// Wait for round completion (poll PIR client initialization)
	maxRetries := 100
	retryDelay := 50 * time.Millisecond
	for retry := 0; retry < maxRetries; retry++ {
		if err := dbClient.InitializePIRClient(ctx, roundID); err == nil {
			break
		}
		if retry < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	// Publish local max threshold so server can aggregate
	maxThresholdKey := fmt.Sprintf("max-threshold-%s", a.workerID)
	maxThresholdPairs := make(map[string][]byte)
	maxThresholdPairs[maxThresholdKey] = float64ToBytes(float64(maxWorkerThreshold))

	// Also publish max threshold as special key (last write wins, but all should be same)
	maxThresholdPairs["max-threshold"] = float64ToBytes(float64(maxWorkerThreshold))

	if err := dbClient.PublishValues(ctx, roundID, a.workerID, maxThresholdPairs); err != nil {
		// If publish fails, we'll compute locally
		a.maxPublicRoundThreshold = maxWorkerThreshold
		a.number_of_rounds = min(a.number_of_rounds-2, a.maxPublicRoundThreshold)
		return nil
	}

	// Wait for round completion to ensure all workers published
	maxRetries2 := 100
	retryDelay2 := 50 * time.Millisecond
	for retry2 := 0; retry2 < maxRetries2; retry2++ {
		if err2 := dbClient.InitializePIRClient(ctx, roundID); err2 == nil {
			break
		}
		if retry2 < maxRetries2-1 {
			time.Sleep(retryDelay2)
		}
	}

	// Query all max thresholds from other workers and compute global max
	// Query all worker-specific thresholds
	maxPublicRoundThreshold := maxWorkerThreshold
	for i := 0; i < a.numWorkers; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		if workerID == a.workerID {
			continue // Skip self
		}
		workerThresholdKey := fmt.Sprintf("max-threshold-%s", workerID)
		thresholdBytes, err := dbClient.GetValue(ctx, roundID, workerThresholdKey)
		if err == nil {
			threshold := int(bytesToFloat64(thresholdBytes))
			if threshold > maxPublicRoundThreshold {
				maxPublicRoundThreshold = threshold
			}
			log.Printf("[%s] Round 0: Worker %s threshold: %d", a.workerID, workerID, threshold)
		} else {
			log.Printf("[%s] Round 0: Could not read threshold from worker %s: %v", a.workerID, workerID, err)
		}
	}

	a.maxPublicRoundThreshold = maxPublicRoundThreshold
	// Compute round count (same formula as original: min(number_of_rounds-2, maxPublicRoundThreshold))
	original_number_of_rounds := a.number_of_rounds
	a.number_of_rounds = min(a.number_of_rounds-2, a.maxPublicRoundThreshold)

	log.Printf("[%s] Round 0: Global max threshold: %d, original number_of_rounds: %d, final number_of_rounds: %d",
		a.workerID, maxPublicRoundThreshold, original_number_of_rounds, a.number_of_rounds)

	// Initialize levels in OKVS (all start at 0.0)
	// This happens implicitly when levels are first queried/updated in round 1
	// We don't need to publish them now since all vertices start at level 0

	log.Printf("[%s] Round 0: Initial exchange completed", a.workerID)
	return nil
}

// executeRoundPublishIncreases executes the publish phase for level increases
// DEPRECATED: This function is no longer used. The algorithm now uses executeRoundSingle
// which performs a simple get-then-set in each round. Kept for reference only.
func (a *KCoreDecomposition) executeRoundPublishIncreases(ctx context.Context, dbClient *client.Client, algorithmRound int) error {
	// Deprecated in single-round mode - use executeRoundSingle instead
	// Keep for compatibility if needed by callers (unused now)
	roundID := uint64(algorithmRound)

	// Start round
	if err := dbClient.StartRound(ctx, roundID, int32(a.numWorkers)); err != nil {
		return fmt.Errorf("failed to start round %d: %w", roundID, err)
	}

	// Query neighbor levels from previous update round
	// For algorithm round 0, initial levels are all 0.0 (no previous round to query)
	// For algorithm round > 0, query from previous update round (where levels were last published)

	neighborLevels := make(map[int]uint)

	// Query levels of neighbors (via PIR from OKVS)
	for _, vertexID := range a.myVertices {
		vertex := a.vertices[vertexID]

		// Query current level for this vertex from the previous update round
		if algorithmRound == 0 {
			// Initial round: all levels start at 0
			vertex.current_level = 0
		} else {
			// Query level from the previous coordination round's update
			// Round r reads levels written in round r-1
			prevLevelRoundID := uint64(algorithmRound)
			levelKey := fmt.Sprintf("level-%d", vertexID)
			levelBytes, err := dbClient.GetValue(ctx, prevLevelRoundID, levelKey)
			if err == nil {
				vertex.current_level = uint(bytesToFloat64(levelBytes))
			} else {
				vertex.current_level = 0 // Default to 0 if not found
			}
		}

		// If threshold already reached, permanently stop processing this vertex
		if int(vertex.current_level) >= vertex.round_threshold {
			vertex.permanent_zero = 0
		}

		// Query neighbor levels if this vertex is still active this round
		if vertex.permanent_zero != 0 {
			for _, neighborID := range vertex.neighbours {
				neighborKey := fmt.Sprintf("level-%d", neighborID)
				if algorithmRound == 0 {
					// Initial levels are all 0.0 (no previous round to query from)
					neighborLevels[neighborID] = 0
				} else {
					// Query neighbor level from previous coordination round's update
					prevLevelRoundID := uint64(algorithmRound)
					neighborLevelBytes, err := dbClient.GetValue(ctx, prevLevelRoundID, neighborKey)
					if err == nil {
						neighborLevels[neighborID] = uint(bytesToFloat64(neighborLevelBytes))
					} else {
						neighborLevels[neighborID] = 0 // Default to 0 if not found
					}
				}
			}
		}
	}

	// Compute level increases (same logic as original workerKCore)
	levelIncreases := make(map[string][]byte)
	// group_index will be computed per-vertex from its current level (LDS group)

	for _, vertexID := range a.myVertices {
		vertex := a.vertices[vertexID]

		// Process vertices that haven't permanently stopped and haven't reached their threshold
		if vertex.permanent_zero != 0 && int(vertex.current_level) < vertex.round_threshold {
			// Count neighbors at same level
			neighbor_count := 0
			for _, neighborID := range vertex.neighbours {
				if neighborLevels[neighborID] == vertex.current_level {
					neighbor_count++
				}
			}

			// Apply noise (same logic as original)
			noised_neighbor_count := int64(neighbor_count)
			if a.noise {
				scale := a.super_step2_geom_factor / (2.0 * float64(vertex.round_threshold))
				geomDist := noise.NewGeomDistribution(scale)
				noise_sampled := geomDist.TwoSidedGeometric()
				extra_bias := int64(3.0 * (2.0 * math.Exp(scale)) / math.Pow((math.Exp(2.0*scale)-1.0), 3.0))
				noised_neighbor_count += noise_sampled
				noised_neighbor_count += extra_bias
			}

			// Compute threshold (same formula as original workerKCore)
			// Original: math.Pow((1+psi), group_index)
			// Use group_index derived from the vertex's current level (LDS group for level)
			vGroupIdx := uint(math.Floor(float64(vertex.current_level) / a.levels_per_group))
			threshold := math.Pow(1.0+a.psi, float64(vGroupIdx))
			increaseKey := fmt.Sprintf("level-increase-%d-round-%d", vertexID, algorithmRound+1)

			if noised_neighbor_count > int64(threshold) {
				vertex.next_level = 1
				levelIncreases[increaseKey] = float64ToBytes(1.0)
			} else {
				vertex.permanent_zero = 0
				levelIncreases[increaseKey] = float64ToBytes(0.0)
			}
		} else {
			// Vertex not active in this round: publish 0.0 to indicate no increase
			// This is important so that level updates in the next phase know this vertex exists
			increaseKey := fmt.Sprintf("level-increase-%d-round-%d", vertexID, algorithmRound+1)
			levelIncreases[increaseKey] = float64ToBytes(0.0)
		}
	}

	// Publish level increases → OKVS
	if err := dbClient.PublishValues(ctx, roundID, a.workerID, levelIncreases); err != nil {
		return fmt.Errorf("failed to publish level increases: %w", err)
	}

	// Wait for round completion
	maxRetries := 100
	retryDelay := 50 * time.Millisecond
	for retry := 0; retry < maxRetries; retry++ {
		if err := dbClient.InitializePIRClient(ctx, roundID); err == nil {
			break
		}
		if retry < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return nil
}

// executeRoundUpdateLevels executes the update phase for level updates
// DEPRECATED: This function is no longer used. The algorithm now uses executeRoundSingle
// which performs a simple get-then-set in each round. Kept for reference only.
func (a *KCoreDecomposition) executeRoundUpdateLevels(ctx context.Context, dbClient *client.Client, algorithmRound int) error {
	// Deprecated in single-round mode - use executeRoundSingle instead
	// Keep for compatibility if needed by callers (unused now)
	roundID := uint64(algorithmRound)

	// Start round
	if err := dbClient.StartRound(ctx, roundID, int32(a.numWorkers)); err != nil {
		return fmt.Errorf("failed to start round %d: %w", roundID, err)
	}

	// Query aggregated level increases from previous round
	prevRoundID := uint64(2*algorithmRound + 1) // Round where increases were published

	// Determine where to query current levels from:
	// - For algorithm round 0: initial levels are 0 (no round to query from)
	// - For algorithm round > 0: query from previous coordination round's update (round r-1)
	prevLevelRoundID := uint64(algorithmRound) // Round where levels were last updated

	levelUpdates := make(map[string][]byte)

	for _, vertexID := range a.myVertices {
		vertex := a.vertices[vertexID]

		// First, query current level from OKVS (from previous update round)
		var currentLevel uint = 0
		if algorithmRound == 0 {
			// Initial round: all levels start at 0
			currentLevel = 0
		} else {
			// Query level from previous update round
			levelKey := fmt.Sprintf("level-%d", vertexID)
			levelBytes, err := dbClient.GetValue(ctx, prevLevelRoundID, levelKey)
			if err == nil {
				currentLevel = uint(bytesToFloat64(levelBytes))
			} else {
				// If not found, default to 0
				currentLevel = 0
			}
		}

		// Query aggregated level increase from previous round
		// In the original algorithm, if ANY worker says increase, the level increases (MAX aggregation)
		// Since server uses last-write-wins, we need to query all workers' values and compute MAX
		// However, each vertex is only assigned to one worker, so only that worker publishes its increase
		// So we just query the aggregated value (which should be from the owning worker)
		increaseKey := fmt.Sprintf("level-increase-%d-round-%d", vertexID, algorithmRound+1)
		increaseBytes, err := dbClient.GetValue(ctx, prevRoundID, increaseKey)

		var newLevel uint = currentLevel
		if err == nil {
			increase := bytesToFloat64(increaseBytes)

			// If increase == 1.0, level increases (same as original: if any worker says increase, it increases)
			// Note: In original algorithm, coordinator uses MAX of all workers' nextLevels for each vertex
			// Since each vertex is only published by one worker (its owner), we just check if it's 1.0
			if increase >= 1.0 {
				newLevel = currentLevel + 1
			}
		}
		// If increase not found, keep current level (no change)

		// Update vertex state (for next round's publish phase)
		vertex.current_level = newLevel

		// Store updated level → OKVS (for next round)
		levelKey := fmt.Sprintf("level-%d", vertexID)
		levelUpdates[levelKey] = float64ToBytes(float64(newLevel))
	}

	// Publish updated levels → OKVS
	if err := dbClient.PublishValues(ctx, roundID, a.workerID, levelUpdates); err != nil {
		return fmt.Errorf("failed to publish level updates: %w", err)
	}

	// Wait for round completion
	maxRetries := 100
	retryDelay := 50 * time.Millisecond
	for retry := 0; retry < maxRetries; retry++ {
		if err := dbClient.InitializePIRClient(ctx, roundID); err == nil {
			break
		}
		if retry < maxRetries-1 {
			time.Sleep(retryDelay)
		}
	}

	return nil
}

// computeCoreNumbers computes final core numbers from levels (same formula as original estimateCoreNumbers)
func (a *KCoreDecomposition) computeCoreNumbers(ctx context.Context, dbClient *client.Client, algorithmRounds int) error {
	log.Printf("[%s] Computing final core numbers from %d algorithm rounds", a.workerID, algorithmRounds)

	// Query final levels from OKVS
	// Last round where levels were updated is the last executed round (algorithmRounds-1)
	var finalRoundID uint64
	if algorithmRounds <= 0 {
		finalRoundID = 0
	} else {
		finalRoundID = uint64(algorithmRounds - 1)
	}

	log.Printf("[%s] Querying final levels from round %d", a.workerID, finalRoundID)

	// Same constants as original estimateCoreNumbers
	two_plus_lambda := 2.0 + a.lambda
	one_plus_psi := 1.0 + a.psi

	computedCount := 0
	for _, vertexID := range a.myVertices {
		levelKey := fmt.Sprintf("level-%d", vertexID)
		levelBytes, err := dbClient.GetValue(ctx, finalRoundID, levelKey)

		var node_level float64
		if err == nil {
			// node_level is stored as float64, convert from uint
			node_level = bytesToFloat64(levelBytes)
		} else {
			node_level = 0.0 // Default to 0 if not found
			log.Printf("[%s] Vertex %d: Could not read final level, defaulting to 0.0", a.workerID, vertexID)
		}

		// Compute core number (same formula as original estimateCoreNumbers)
		// Original: frac_numerator := node_level + 1.0
		//          power := math.Max(math.Floor(float64(frac_numerator)/levels_per_group)-1.0, 0.0)
		//          core_numbers[i] = two_plus_lambda * math.Pow(one_plus_psi, power)
		frac_numerator := node_level + 1.0
		power := math.Max(math.Floor(frac_numerator/a.levels_per_group)-1.0, 0.0)
		core_number := two_plus_lambda * math.Pow(one_plus_psi, power)

		log.Printf("[%s] Vertex %d: final level=%.2f, core_number=%.4f", a.workerID, vertexID, node_level, core_number)

		a.mu.Lock()
		a.results[vertexID] = core_number
		a.mu.Unlock()
		computedCount++
	}

	log.Printf("[%s] Computed core numbers for %d vertices", a.workerID, computedCount)
	return nil
}

// writeResults writes the results to a file (same format as original)
func (a *KCoreDecomposition) writeResults() error {
	file, err := os.Create(a.resultFile)
	if err != nil {
		return fmt.Errorf("failed to create result file: %w", err)
	}
	defer file.Close()

	// Write results (same format as original KCoreLDPCoord)
	a.mu.Lock()
	defer a.mu.Unlock()

	for vertexID, coreNumber := range a.results {
		fmt.Fprintf(file, "%d: %.4f\n", vertexID, coreNumber)
	}

	return nil
}

func (a *KCoreDecomposition) GetRoundData(roundID int) *common.RoundData {
	// For k-core, rounds are determined dynamically
	return &common.RoundData{
		RoundID:         roundID,
		ExpectedWorkers: int32(a.numWorkers),
		PublishKeys:     []string{},
		QueryKeys:       []string{},
		Metadata:        make(map[string]interface{}),
	}
}

func (a *KCoreDecomposition) ProcessRound(roundID int, roundResults map[string]interface{}) error {
	// Results are processed in Execute method
	return nil
}

func (a *KCoreDecomposition) GetResult() *common.AlgorithmResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	return &common.AlgorithmResult{
		AlgorithmName:    a.Name(),
		NumRounds:        a.number_of_rounds,
		Converged:        true,
		ConvergenceRound: a.number_of_rounds,
		Results: map[string]interface{}{
			"num_results":     len(a.results),
			"result_file":     a.resultFile,
			"my_vertices":     a.myVertices,
			"num_my_vertices": len(a.myVertices),
		},
		Metadata: map[string]interface{}{
			"worker_id":                  a.workerID,
			"num_workers":                a.numWorkers,
			"max_public_round_threshold": a.maxPublicRoundThreshold,
		},
	}
}

// Helper functions (preserve original naming and logic)

// log_a_to_base_b computes log base b of a (same as original)
func log_a_to_base_b(a int, b float64) float64 {
	return math.Log2(float64(a)) / math.Log2(b)
}

// float64ToBytes converts float64 to 8-byte little-endian (for OKVS)
func float64ToBytes(f float64) []byte {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, math.Float64bits(f))
	return bytes
}

// bytesToFloat64 converts 8-byte little-endian to float64
func bytesToFloat64(bytes []byte) float64 {
	if len(bytes) < 8 {
		return 0.0
	}
	bits := binary.LittleEndian.Uint64(bytes[:8])
	return math.Float64frombits(bits)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Register the algorithm
func init() {
	Register("kcore-decomposition", NewKCoreDecomposition)
}

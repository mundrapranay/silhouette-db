package common

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
)

// LoadGraphData loads graph data from the configuration.
// If localTesting is true and workerID is provided, loads from partitioned files
// (e.g., data/1.txt for worker-0, data/2.txt for worker-1).
// Otherwise, loads from the specified file_path.
func LoadGraphData(config *GraphInputConfig, workerID string) (*GraphData, error) {
	graphData := &GraphData{
		Edges:            []Edge{},
		VertexProperties: make(map[int]map[string]interface{}),
		EdgeWeights:      make(map[int]float64),
	}

	// Load from file if specified
	if config.FilePath != "" {
		filePath := config.FilePath

		// Local testing mode: construct worker-specific file path
		if config.LocalTesting && workerID != "" {
			workerIndex, err := extractWorkerIndex(workerID)
			if err != nil {
				return nil, fmt.Errorf("invalid worker_id for local testing: %w", err)
			}
			// Construct path: {base_dir}/{worker_index+1}.txt
			// e.g., worker-0 -> data/1.txt, worker-1 -> data/2.txt
			filePath = fmt.Sprintf("%s/%d.txt", config.FilePath, workerIndex+1)
		}

		switch config.Format {
		case "edgelist", "edge_list":
			return loadEdgeListFromFile(filePath, config.Directed, graphData)
		case "adjacency_list":
			return loadAdjacencyListFromFile(filePath, graphData)
		default:
			return nil, fmt.Errorf("unsupported graph format: %s", config.Format)
		}
	}

	// Load from config edges if specified
	if len(config.Edges) > 0 {
		graphData.Edges = make([]Edge, len(config.Edges))
		vertexSet := make(map[int]bool)

		for i, e := range config.Edges {
			graphData.Edges[i] = Edge{U: e.U, V: e.V}
			vertexSet[e.U] = true
			vertexSet[e.V] = true

			if e.W != 0 {
				graphData.EdgeWeights[i] = e.W
			}
		}

		if config.NumVertices > 0 {
			graphData.NumVertices = config.NumVertices
		} else {
			graphData.NumVertices = len(vertexSet)
		}
		graphData.NumEdges = len(config.Edges)

		return graphData, nil
	}

	return nil, fmt.Errorf("no graph data provided: specify either file_path or edges")
}

// loadEdgeListFromFile loads an edge list from a file
func loadEdgeListFromFile(filePath string, directed bool, graphData *GraphData) (*GraphData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open graph file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ' '          // Space-separated or tab-separated
	reader.Comment = '#'        // Comments start with #
	reader.FieldsPerRecord = -1 // Allow variable fields

	vertexSet := make(map[int]bool)
	edges := []Edge{}
	edgeWeights := make(map[int]float64)
	edgeIndex := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read graph file: %w", err)
		}

		// Skip empty lines or comments
		if len(record) == 0 || record[0] == "" {
			continue
		}

		// Parse edge: u v [weight]
		if len(record) < 2 {
			return nil, fmt.Errorf("invalid edge format: need at least 2 values (u v), got: %v", record)
		}

		u, err := strconv.Atoi(record[0])
		if err != nil {
			return nil, fmt.Errorf("invalid vertex ID: %s", record[0])
		}

		v, err := strconv.Atoi(record[1])
		if err != nil {
			return nil, fmt.Errorf("invalid vertex ID: %s", record[1])
		}

		edges = append(edges, Edge{U: u, V: v})
		vertexSet[u] = true
		vertexSet[v] = true

		// Parse optional weight
		if len(record) >= 3 {
			weight, err := strconv.ParseFloat(record[2], 64)
			if err == nil {
				edgeWeights[edgeIndex] = weight
			}
		}

		// Add reverse edge if undirected
		if !directed {
			edges = append(edges, Edge{U: v, V: u})
			edgeIndex++
			if len(record) >= 3 {
				edgeWeights[edgeIndex] = edgeWeights[edgeIndex-1] // Same weight for reverse edge
			}
		}

		edgeIndex++
	}

	graphData.Edges = edges
	graphData.NumVertices = len(vertexSet)
	graphData.NumEdges = len(edges)
	graphData.EdgeWeights = edgeWeights

	return graphData, nil
}

// loadAdjacencyListFromFile loads an adjacency list from a file
func loadAdjacencyListFromFile(filePath string, graphData *GraphData) (*GraphData, error) {
	// TODO: Implement adjacency list loader
	return nil, fmt.Errorf("adjacency list format not yet implemented")
}

// GetVertexAssignment returns which worker should handle a vertex
func GetVertexAssignment(vertexID int, numWorkers int, customAssignment map[int]string) string {
	if customAssignment != nil {
		if workerID, exists := customAssignment[vertexID]; exists {
			return workerID
		}
	}

	// Default: round-robin assignment
	workerID := vertexID % numWorkers
	return fmt.Sprintf("worker-%d", workerID)
}

// extractWorkerIndex extracts the worker index from a worker ID string.
// Supports formats: "worker-0", "worker-1", etc.
// Returns the numeric index (0-based).
func extractWorkerIndex(workerID string) (int, error) {
	// Try to parse formats like "worker-0", "worker-1", etc.
	var index int
	n, err := fmt.Sscanf(workerID, "worker-%d", &index)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("failed to extract worker index from %s (expected format: worker-N)", workerID)
	}
	if index < 0 {
		return 0, fmt.Errorf("worker index must be non-negative, got: %d", index)
	}
	return index, nil
}

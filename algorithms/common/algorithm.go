package common

import (
	"context"
	"fmt"

	"github.com/mundrapranay/silhouette-db/pkg/client"
)

// AlgorithmType represents the type of algorithm
type AlgorithmType string

const (
	AlgorithmTypeExact AlgorithmType = "exact"
	AlgorithmTypeLEDP  AlgorithmType = "ledp"
)

// GraphAlgorithm is the interface that all graph algorithms must implement.
// Algorithms are round-based and synchronous, using silhouette-db for coordination.
type GraphAlgorithm interface {
	// Name returns the name of the algorithm
	Name() string

	// Type returns the algorithm type (exact or LEDP)
	Type() AlgorithmType

	// Initialize prepares the algorithm with graph data and configuration.
	// graphData contains the initial graph representation.
	// Returns an error if initialization fails.
	Initialize(ctx context.Context, graphData *GraphData, config map[string]interface{}) error

	// Execute runs the algorithm for a specified number of rounds.
	// The algorithm coordinates through the provided client connection.
	// Returns results and an error if execution fails.
	Execute(ctx context.Context, client *client.Client, numRounds int) (*AlgorithmResult, error)

	// GetRoundData retrieves data needed for a specific round.
	// Used by workers to know what data to publish/query.
	GetRoundData(roundID int) *RoundData

	// ProcessRound processes the results from a completed round.
	// Updates internal state based on aggregated results from all workers.
	ProcessRound(roundID int, roundResults map[string]interface{}) error

	// GetResult returns the final algorithm result after execution completes.
	GetResult() *AlgorithmResult
}

// GraphData represents the input graph for algorithms
type GraphData struct {
	// Number of vertices
	NumVertices int

	// Number of edges
	NumEdges int

	// Edges: list of (u, v) pairs where u and v are vertex IDs
	Edges []Edge

	// Optional: Vertex properties (vertex_id -> properties map)
	VertexProperties map[int]map[string]interface{}

	// Optional: Edge weights (edge_index -> weight)
	EdgeWeights map[int]float64
}

// Edge represents a single edge in the graph
type Edge struct {
	U int // Source vertex
	V int // Target vertex
}

// RoundData represents the data requirements for a round
type RoundData struct {
	RoundID         int
	ExpectedWorkers int32

	// Keys this worker should publish (worker-specific)
	PublishKeys []string

	// Keys this worker should query
	QueryKeys []string

	// Metadata for this round
	Metadata map[string]interface{}
}

// AlgorithmResult represents the final output of an algorithm execution
type AlgorithmResult struct {
	AlgorithmName    string
	NumRounds        int
	Converged        bool
	ConvergenceRound int // Round at which convergence occurred (if applicable)

	// Results: algorithm-specific data
	// Examples:
	// - For shortest paths: map[vertex_id]distance
	// - For PageRank: map[vertex_id]rank
	// - For clustering: map[vertex_id]cluster_id
	Results map[string]interface{}

	// Metadata: execution statistics, timing, etc.
	Metadata map[string]interface{}
}

// AlgorithmConfig represents the configuration for an algorithm
type AlgorithmConfig struct {
	// Algorithm name (must match an available algorithm)
	AlgorithmName string `yaml:"algorithm_name" json:"algorithm_name"`

	// Algorithm type
	AlgorithmType AlgorithmType `yaml:"algorithm_type" json:"algorithm_type"`

	// Silhouette-db server address
	ServerAddress string `yaml:"server_address" json:"server_address"`

	// Worker configuration
	WorkerConfig WorkerConfig `yaml:"worker_config" json:"worker_config"`

	// Algorithm-specific parameters
	Parameters map[string]interface{} `yaml:"parameters" json:"parameters"`

	// Graph input configuration
	GraphConfig GraphInputConfig `yaml:"graph_config" json:"graph_config"`
}

// WorkerConfig specifies how workers participate in the algorithm
type WorkerConfig struct {
	// Number of workers
	NumWorkers int `yaml:"num_workers" json:"num_workers"`

	// Worker ID (for single-worker execution, or worker index)
	WorkerID string `yaml:"worker_id" json:"worker_id"`

	// Assignment of vertices to workers (optional)
	// If not specified, vertices are assigned round-robin
	VertexAssignment map[int]string `yaml:"vertex_assignment" json:"vertex_assignment"`
}

// GraphInputConfig specifies how to load the graph
type GraphInputConfig struct {
	// Input format: "edgelist", "adjacency_list", "matrix", etc.
	Format string `yaml:"format" json:"format"`

	// Input file path (if loading from file)
	// For local_testing=false: Each worker uses this same path (deployment mode)
	// For local_testing=true: This is the base directory (e.g., "data"), worker-specific files are auto-selected
	FilePath string `yaml:"file_path" json:"file_path"`

	// Local testing mode: if true, load from partitioned files (data/1.txt, data/2.txt, etc.)
	// based on worker_id. If false, use file_path directly (deployment mode where each
	// worker has its own graph file with the same path/name).
	LocalTesting bool `yaml:"local_testing" json:"local_testing"`

	// Or: direct specification in config
	Edges []struct {
		U int     `yaml:"u" json:"u"`
		V int     `yaml:"v" json:"v"`
		W float64 `yaml:"w,omitempty" json:"w,omitempty"` // Optional weight
	} `yaml:"edges" json:"edges"`

	// Number of vertices (if not inferable from edges)
	NumVertices int `yaml:"num_vertices" json:"num_vertices"`

	// Whether graph is directed
	Directed bool `yaml:"directed" json:"directed"`
}

// Validate checks if the algorithm config is valid
func (c *AlgorithmConfig) Validate() error {
	if c.AlgorithmName == "" {
		return fmt.Errorf("algorithm_name is required")
	}

	if c.AlgorithmType != AlgorithmTypeExact && c.AlgorithmType != AlgorithmTypeLEDP {
		return fmt.Errorf("algorithm_type must be 'exact' or 'ledp', got: %s", c.AlgorithmType)
	}

	if c.ServerAddress == "" {
		return fmt.Errorf("server_address is required")
	}

	if c.WorkerConfig.NumWorkers <= 0 {
		return fmt.Errorf("num_workers must be > 0, got: %d", c.WorkerConfig.NumWorkers)
	}

	if c.WorkerConfig.WorkerID == "" {
		return fmt.Errorf("worker_id is required")
	}

	return nil
}

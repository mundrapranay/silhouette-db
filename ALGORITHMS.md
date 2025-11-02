# Graph Algorithms Framework

This document describes the graph algorithms framework built on top of `silhouette-db`.

## Overview

The algorithms framework provides a structure for implementing round-based synchronous graph algorithms that use `silhouette-db` as the coordination backend. Algorithms can be either:

- **Exact Algorithms**: Standard graph algorithms (e.g., shortest path, PageRank)
- **LEDP Algorithms**: Local Edge Differentially Private algorithms (e.g., private shortest path, private PageRank)

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│            Algorithm Runner (Entry Point)               │
│  - Loads config file                                    │
│  - Selects algorithm by type and name                   │
│  - Initializes algorithm with graph data                │
│  - Executes algorithm using silhouette-db               │
└─────────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────┐
│              Algorithm Interface                        │
│  - Initialize: Prepare algorithm with graph data        │
│  - Execute: Run algorithm for N rounds                  │
│  - GetRoundData: Get data for a round                   │
│  - ProcessRound: Process round results                  │
│  - GetResult: Get final algorithm result                │
└─────────────────────────────────────────────────────────┘
                         │
         ┌───────────────┴───────────────┐
         ▼                               ▼
┌──────────────────┐          ┌──────────────────┐
│  Exact Algorithms│          │  LEDP Algorithms │
│  - shortest_path │          │  - pagerank-ledp│
│  - pagerank      │          │  - sssp-ledp     │
│  - bfs           │          │  - ...          │
└──────────────────┘          └──────────────────┘
         │                               │
         └───────────────┬───────────────┘
                         ▼
┌─────────────────────────────────────────────────────────┐
│            silhouette-db Coordination Layer             │
│  - OKVS: Oblivious key-value storage                    │
│  - PIR: Private Information Retrieval                   │
│  - Raft: Distributed consensus                          │
└─────────────────────────────────────────────────────────┘
```

## Directory Structure

```
algorithms/
├── common/                    # Shared utilities and interfaces
│   ├── algorithm.go          # Algorithm interface and core types
│   ├── graph.go              # Graph loading utilities
│   └── config.go             # Configuration loading
├── exact/                     # Exact (non-private) algorithms
│   ├── registry.go           # Algorithm registry
│   ├── placeholder.go        # Example placeholder
│   ├── shortest_path.go     # TODO: Implement
│   ├── pagerank.go          # TODO: Implement
│   └── ...
└── ledp/                      # LEDP (private) algorithms
    ├── registry.go           # Algorithm registry
    ├── placeholder.go        # Example placeholder
    ├── pagerank_ledp.go     # TODO: Implement
    ├── sssp_ledp.go         # TODO: Implement
    └── ...
```

## Configuration File Format

Algorithms are configured via YAML files:

```yaml
# Required: Algorithm identification
algorithm_name: shortest_path
algorithm_type: exact  # or "ledp"

# Required: silhouette-db server address
server_address: "127.0.0.1:9090"

# Required: Worker configuration
worker_config:
  num_workers: 5
  worker_id: "worker-0"
  # Optional: Custom vertex-to-worker assignment
  # vertex_assignment:
  #   0: "worker-0"
  #   1: "worker-0"

# Required: Graph input configuration
graph_config:
  # Option 1: Load from file
  format: "edgelist"  # or "adjacency_list"
  file_path: "/path/to/graph.txt"
  
  # Option 2: Specify edges directly
  # edges:
  #   - u: 0
  #     v: 1
  #     w: 1.5  # Optional weight
  
  # Optional: Number of vertices (auto-detected if not specified)
  num_vertices: 100
  
  # Required: Whether graph is directed
  directed: false

# Required: Algorithm parameters
parameters:
  # Required: Number of rounds to execute
  num_rounds: 10
  
  # Algorithm-specific parameters
  # Example for shortest path:
  source_vertex: 0
  
  # Example for PageRank:
  damping_factor: 0.85
  tolerance: 1e-6
  
  # Example for LEDP algorithms:
  epsilon: 1.0  # Privacy parameter
  delta: 1e-5   # Privacy parameter
```

## Usage

### Building

```bash
# Build the algorithm runner
make build-algorithm-runner
```

### Running

```bash
# Run with config file
./bin/algorithm-runner -config configs/shortest_path.yaml

# With verbose logging
./bin/algorithm-runner -config configs/shortest_path.yaml -verbose
```

### Listing Available Algorithms

```bash
# List all algorithms (programmatically)
go run ./cmd/algorithm-runner -list-algorithms
```

## Implementing a New Algorithm

1. **Choose the algorithm type** (exact or LEDP)
2. **Create algorithm file** in `algorithms/exact/` or `algorithms/ledp/`
3. **Implement the `GraphAlgorithm` interface**:

```go
package exact

import (
	"context"
	"github.com/mundrapranay/silhouette-db/algorithms/common"
	"github.com/mundrapranay/silhouette-db/pkg/client"
)

type ShortestPathAlgorithm struct {
	// Algorithm state
	graphData *common.GraphData
	distances map[int]float64
	// ...
}

func NewShortestPathAlgorithm() common.GraphAlgorithm {
	return &ShortestPathAlgorithm{}
}

func (a *ShortestPathAlgorithm) Name() string {
	return "shortest_path"
}

func (a *ShortestPathAlgorithm) Type() common.AlgorithmType {
	return common.AlgorithmTypeExact
}

func (a *ShortestPathAlgorithm) Initialize(ctx context.Context, graphData *common.GraphData, config map[string]interface{}) error {
	a.graphData = graphData
	// Initialize algorithm state
	return nil
}

func (a *ShortestPathAlgorithm) Execute(ctx context.Context, client *client.Client, numRounds int) (*common.AlgorithmResult, error) {
	// Implement algorithm execution logic
	// - Start rounds
	// - Publish values
	// - Query values
	// - Process results
	return a.GetResult(), nil
}

// Implement other required methods...

// Register the algorithm
func init() {
	Register("shortest_path", NewShortestPathAlgorithm)
}
```

## Round-Based Execution Model

Algorithms execute in synchronous rounds:

1. **Round Start**: All workers coordinate to start round N
   ```go
   client.StartRound(ctx, roundID, numWorkers)
   ```

2. **Publish Phase**: Each worker publishes its local updates
   ```go
   pairs := map[string][]byte{
       "vertex-0": encodeValue(value0),
       "vertex-1": encodeValue(value1),
   }
   client.PublishValues(ctx, roundID, workerID, pairs)
   ```

3. **Query Phase**: Workers query for values they need
   ```go
   value, err := client.GetValue(ctx, roundID, "vertex-5")
   ```

4. **Process Phase**: Workers process round results and update state

5. **Repeat**: Continue until convergence or max rounds

## Algorithm Interface

All algorithms must implement:

- `Initialize`: Prepare algorithm with graph and config
- `Execute`: Run the algorithm for N rounds
- `GetRoundData`: Get data requirements for a round
- `ProcessRound`: Process aggregated round results
- `GetResult`: Return final algorithm result

## Next Steps

1. ✅ Directory structure created
2. ✅ Algorithm interface defined
3. ✅ Configuration system implemented
4. ✅ Entry point (algorithm-runner) created
5. ⏳ Implement actual algorithms:
   - Exact: shortest path, BFS, PageRank
   - LEDP: private shortest path, private PageRank


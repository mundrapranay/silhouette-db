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
│  - shortest_path │          │  - pagerank-ledp │
│  - pagerank      │          │  - sssp-ledp     │
│  - bfs           │          │  - ...           │
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
  # (Automatically added by graph generation script)
  # vertex_assignment:
  #   0: "worker-0"
  #   1: "worker-0"
  #   2: "worker-1"

# Required: Graph input configuration
graph_config:
  # Option 1: Load from file
  format: "edgelist"  # or "adjacency_list"
  
  # Local testing mode: if true, loads from partitioned files based on worker_id
  # (e.g., data/1.txt for worker-0, data/2.txt for worker-1)
  # If false, uses file_path directly (deployment mode where each worker
  # has its own graph file with the same path/name)
  local_testing: true  # Set to false for deployment
  
  # For local_testing=true: Base directory containing partitioned files (e.g., "data")
  # For local_testing=false: Full path to the graph file (same for all workers)
  file_path: "data"  # For local testing: directory containing 1.txt, 2.txt, etc.
  # file_path: "/path/to/graph.txt"  # For deployment: full path to graph file
  
  # Option 2: Specify edges directly in config (alternative to file_path)
  # edges:
  #   - u: 0
  #     v: 1
  #     w: 1.5  # Optional weight
  
  # Optional: Number of vertices (auto-detected if not specified)
  # num_vertices: 100
  
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

## Local Testing vs Deployment

The framework supports two modes for graph data loading:

### Local Testing Mode

For local development and testing, graphs are partitioned into multiple files (one per worker):

**Configuration:**
```yaml
graph_config:
  format: "edgelist"
  local_testing: true  # Enable local testing mode
  file_path: "data"    # Base directory containing partitioned files
  directed: false
```

**How it works:**
- Each worker loads from a partitioned file based on its `worker_id`
- Worker-0 loads from `data/1.txt`
- Worker-1 loads from `data/2.txt`
- Worker-N loads from `data/{N+1}.txt`
- All partitioned files are in the same directory (`data/`)

**Graph Generation:**
The graph generation script (`data-generation/generate_graph.py`) automatically:
1. Generates a random undirected graph
2. Assigns vertices to workers (deterministically, round-robin by default)
3. Partitions edges based on vertex ownership (edge goes to worker that owns source vertex)
4. Writes partitioned files to `data/1.txt`, `data/2.txt`, etc.
5. Updates the config file with `vertex_assignment` mapping

```bash
# Generate partitioned graph for local testing
python3 data-generation/generate_graph.py \
    --config configs/degree_collector.yaml \
    --num-vertices 20 \
    --num-edges 30 \
    --seed 42 \
    --output-dir data

# This creates:
# - data/1.txt (edges for worker-0)
# - data/2.txt (edges for worker-1)
# - data/3.txt (edges for worker-2)
# Also updates config with vertex_assignment like:
#   vertex_assignment:
#     '0': worker-0
#     '1': worker-1
#     '2': worker-2
#     ...
```

**Note:** For undirected graphs, each edge `(u, v)` is stored twice: as `(u, v)` and `(v, u)` in the partitioned files. The edge `(u, v)` goes to the worker that owns vertex `u`, and `(v, u)` goes to the worker that owns vertex `v`.

**Example:**
```bash
# Step 1: Generate partitioned graph
python3 data-generation/generate_graph.py \
    --config configs/degree_collector.yaml \
    --num-vertices 100 \
    --num-edges 200 \
    --seed 42

# Step 2: Run workers (each loads from its partition)
./bin/algorithm-runner -config configs/degree_collector_worker-0.yaml &
./bin/algorithm-runner -config configs/degree_collector_worker-1.yaml &
./bin/algorithm-runner -config configs/degree_collector_worker-2.yaml &
```

### Deployment Mode

For production deployment, each worker/server has its own complete graph file:

**Configuration:**
```yaml
graph_config:
  format: "edgelist"
  local_testing: false  # Deployment mode
  file_path: "/data/graphs/main_graph.txt"  # Full path to graph file
  directed: false
```

**How it works:**
- Each worker loads from the same file path
- All workers have access to the complete graph
- Graph files are stored on each server/worker's local filesystem
- Suitable for distributed deployment where each node has its own data

**Example:**
```bash
# Each worker loads from /data/graphs/main_graph.txt
# (file is stored locally on each worker's machine)
./bin/algorithm-runner -config configs/shortest_path_worker-0.yaml
```

### Vertex Assignment

The `vertex_assignment` configuration maps vertices to workers:

```yaml
worker_config:
  num_workers: 3
  worker_id: "worker-0"
  vertex_assignment:
    '0': worker-0  # Vertex 0 assigned to worker-0
    '1': worker-1  # Vertex 1 assigned to worker-1
    '2': worker-2  # Vertex 2 assigned to worker-2
    '3': worker-0  # Vertex 3 assigned to worker-0
    # ...
```

**Important Notes:**
- **Automatically generated**: The graph generation script (`generate_graph.py`) automatically computes and adds `vertex_assignment` to the config file
- **Consistency**: All workers use the same assignment (ensures correctness)
- **Deterministic**: Assignment is computed deterministically (round-robin by default)
- **Matching partition**: The assignment matches how edges are partitioned into files

**Without config assignment:**
If `vertex_assignment` is not provided, algorithms recompute it deterministically using the same rules (round-robin: `vertexID % numWorkers`), ensuring consistency.

For more details, see the [Graph Generation README](../data-generation/README.md).

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

## Testing Algorithms

For testing algorithms locally, use the automated test scripts:

```bash
# Test degree-collector algorithm (with local testing mode)
./scripts/test-degree-collector.sh

# Custom parameters
NUM_WORKERS=5 NUM_VERTICES=100 NUM_EDGES=200 ./scripts/test-degree-collector.sh
```

The test script:
1. Generates partitioned graph files (`data/1.txt`, `data/2.txt`, etc.)
2. Updates config with vertex assignments
3. Starts silhouette-db server
4. Runs all workers with appropriate configs
5. Verifies results

For more details, see the [Testing Guide](./testing.md).

## Next Steps

1. ✅ Directory structure created
2. ✅ Algorithm interface defined
3. ✅ Configuration system implemented
4. ✅ Entry point (algorithm-runner) created
5. ✅ Local testing support implemented
6. ✅ Graph generation and partitioning automated
7. ⏳ Implement actual algorithms:
   - Exact: shortest path, BFS, PageRank
   - LEDP: private shortest path, private PageRank


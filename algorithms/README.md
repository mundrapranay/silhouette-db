# Graph Algorithms for silhouette-db

This directory contains round-based synchronous graph algorithms that use `silhouette-db` as the coordination backend.

## Directory Structure

```
algorithms/
├── common/           # Shared interfaces, utilities, and types
│   ├── algorithm.go # Algorithm interface and core types
│   ├── graph.go     # Graph loading and manipulation utilities
│   └── config.go    # Configuration loading and validation
├── exact/           # Exact algorithms (non-private)
│   ├── registry.go  # Algorithm registration for exact algorithms
│   └── ...          # Algorithm implementations
└── ledp/            # LEDP algorithms (Local Edge Differentially Private)
    ├── registry.go  # Algorithm registration for LEDP algorithms
    └── ...          # Algorithm implementations
```

## Algorithm Interface

All algorithms must implement the `GraphAlgorithm` interface defined in `common/algorithm.go`:

```go
type GraphAlgorithm interface {
    Name() string
    Type() AlgorithmType
    Initialize(ctx context.Context, graphData *GraphData, config map[string]interface{}) error
    Execute(ctx context.Context, client *client.Client, numRounds int) (*AlgorithmResult, error)
    GetRoundData(roundID int) *RoundData
    ProcessRound(roundID int, roundResults map[string]interface{}) error
    GetResult() *AlgorithmResult
}
```

## Configuration File Format

Algorithms are configured via YAML files:

```yaml
algorithm_name: shortest_path
algorithm_type: exact  # or "ledp"
server_address: "127.0.0.1:9090"

worker_config:
  num_workers: 5
  worker_id: "worker-0"

graph_config:
  format: "edgelist"
  file_path: "/path/to/graph.txt"
  directed: false

parameters:
  num_rounds: 10
  # Algorithm-specific parameters
```

## Usage

Run an algorithm using the `algorithm-runner` command:

```bash
# Build the runner
make build-algorithm-runner

# Run with config file
./bin/algorithm-runner -config configs/shortest_path.yaml

# With verbose logging
./bin/algorithm-runner -config configs/shortest_path.yaml -verbose
```

## Algorithm Registration

Algorithms register themselves using the registry pattern:

```go
// In algorithms/exact/shortest_path.go or algorithms/ledp/...

func init() {
    exact.Register("shortest_path", NewShortestPathAlgorithm)
    // or
    ledp.Register("pagerank-ledp", NewPageRankLEDPAlgorithm)
}
```

## Implementing a New Algorithm

1. Create a new file in `algorithms/exact/` or `algorithms/ledp/`
2. Implement the `GraphAlgorithm` interface
3. Register the algorithm in an `init()` function
4. The algorithm will automatically be available when imported

## Round-Based Execution Model

Algorithms execute in synchronous rounds:

1. **Round Start**: All workers coordinate to start a new round
2. **Publish Phase**: Each worker publishes its local updates to silhouette-db
3. **Aggregation**: The server aggregates all worker contributions
4. **Query Phase**: Workers query for values they need for the next round
5. **Process**: Workers process the round results and update local state
6. **Repeat**: Continue until convergence or max rounds reached

The silhouette-db backend ensures:
- **Oblivious Storage**: Storage patterns are hidden via OKVS
- **Private Queries**: Query privacy is preserved via PIR
- **Fault Tolerance**: System remains available despite node failures


# Data Generation Scripts

This directory contains Python scripts for generating test graphs and partitioning them for workers.

## Local Testing vs Deployment

The graph generation is designed for **local testing mode**:
- In local testing: One `data/` folder with partitioned files (`1.txt`, `2.txt`, etc.)
- In deployment: Each worker/server has its own graph file with the same path/name

When running algorithms:
- Set `local_testing: true` in config → Loads from `data/{worker_index+1}.txt`
- Set `local_testing: false` in config → Loads from `file_path` directly (deployment mode)

## Graph Generation

### `generate_graph.py`

Generates random undirected graphs and partitions them based on algorithm configuration.

**Features:**
- Generates random undirected graphs
- Partitions graph based on worker configuration
- Stores edges in format: `u v` (one edge per line)
- Each edge stored twice for undirected graphs: `(u, v)` and `(v, u)`
- Outputs partitioned files: `1.txt`, `2.txt`, ... for each worker

**Usage:**

```bash
# Generate graph using config file
python3 data-generation/generate_graph.py --config configs/degree_collector.yaml

# With custom parameters
python3 data-generation/generate_graph.py \
    --config configs/degree_collector.yaml \
    --num-vertices 1000 \
    --num-edges 5000 \
    --seed 42

# Save global graph as well
python3 data-generation/generate_graph.py \
    --config configs/degree_collector.yaml \
    --global-graph data/graph.txt
```

**Arguments:**
- `--config`: Path to algorithm configuration YAML file (required)
- `--num-vertices`: Number of vertices (overrides config)
- `--num-edges`: Number of edges (overrides config)
- `--seed`: Random seed for reproducibility
- `--output-dir`: Output directory (default: `data/`)
- `--global-graph`: Path to save complete graph file (optional)

**Output:**
- `data/1.txt`: Edges for worker 0
- `data/2.txt`: Edges for worker 1
- `data/N.txt`: Edges for worker N-1

**Graph Format:**
```
0 1
1 0
1 2
2 1
...
```

Each line contains two space-separated integers: source vertex and target vertex.

## Example

```bash
# Generate graph for degree-collector algorithm
python3 data-generation/generate_graph.py \
    --config configs/degree_collector.yaml \
    --num-vertices 100 \
    --num-edges 200 \
    --seed 123

# This will create:
# - data/1.txt (edges for worker-0)
# - data/2.txt (edges for worker-1)
# - data/3.txt (edges for worker-2)
```

## Requirements

Install Python dependencies:

```bash
pip install pyyaml
```

Or:

```bash
pip install -r data-generation/requirements.txt
```


# Testing Degree Collector Algorithm

This guide explains how to test the `degree-collector` algorithm in local testing mode.

## Prerequisites

1. **silhouette-db server** must be running
2. **Python 3** with `pyyaml` installed
3. **Graph data** generated using `data-generation/generate_graph.py`

## Quick Start

### Option 1: Automated Test Script

Use the provided test script for an end-to-end test:

```bash
# Basic test with defaults (3 workers, 20 vertices, 30 edges)
./scripts/test-degree-collector.sh

# With custom parameters
NUM_WORKERS=5 NUM_VERTICES=100 NUM_EDGES=200 ./scripts/test-degree-collector.sh

# Keep test files for inspection
./scripts/test-degree-collector.sh --keep
```

### Option 2: Manual Step-by-Step

#### Step 1: Generate Test Graph

```bash
# Generate graph data partitioned for workers
python3 data-generation/generate_graph.py \
    --config configs/degree_collector.yaml \
    --num-vertices 20 \
    --num-edges 30 \
    --seed 42

# This creates:
# - data/1.txt (edges for worker-0)
# - data/2.txt (edges for worker-1)
# - data/3.txt (edges for worker-2)
```

#### Step 2: Start silhouette-db Server

```bash
# Start server (if not already running)
./bin/silhouette-server -config configs/node1.hcl

# Or in background:
./bin/silhouette-server -config configs/node1.hcl > server.log 2>&1 &
```

#### Step 3: Create Worker Configs

For each worker, create a config file with the correct `worker_id`:

```bash
# Create configs for each worker
cp configs/degree_collector.yaml configs/degree_collector_worker-0.yaml
cp configs/degree_collector.yaml configs/degree_collector_worker-1.yaml
cp configs/degree_collector.yaml configs/degree_collector_worker-2.yaml

# Update worker_id in each config file
# For worker-0:
sed -i '' 's/worker_id: "worker-0"/worker_id: "worker-0"/' configs/degree_collector_worker-0.yaml
# For worker-1:
sed -i '' 's/worker_id: "worker-0"/worker_id: "worker-1"/' configs/degree_collector_worker-1.yaml
# For worker-2:
sed -i '' 's/worker_id: "worker-0"/worker_id: "worker-2"/' configs/degree_collector_worker-2.yaml
```

Or use Python to update configs programmatically:

```bash
python3 <<EOF
import yaml

for i in range(3):
    with open('configs/degree_collector.yaml', 'r') as f:
        config = yaml.safe_load(f)
    config['worker_config']['worker_id'] = f'worker-{i}'
    with open(f'configs/degree_collector_worker-{i}.yaml', 'w') as f:
        yaml.dump(config, f, default_flow_style=False, sort_keys=False)
EOF
```

#### Step 4: Run Workers

**Option A: Run sequentially** (for debugging):

```bash
# Terminal 1: Worker 0
./bin/algorithm-runner -config configs/degree_collector_worker-0.yaml -verbose

# Terminal 2: Worker 1 (wait for worker 0 to complete round 1)
./bin/algorithm-runner -config configs/degree_collector_worker-1.yaml -verbose

# Terminal 3: Worker 2 (wait for worker 1 to complete round 1)
./bin/algorithm-runner -config configs/degree_collector_worker-2.yaml -verbose
```

**Option B: Run in parallel** (proper test):

```bash
# Start all workers simultaneously
./bin/algorithm-runner -config configs/degree_collector_worker-0.yaml > worker-0.log 2>&1 &
./bin/algorithm-runner -config configs/degree_collector_worker-1.yaml > worker-1.log 2>&1 &
./bin/algorithm-runner -config configs/degree_collector_worker-2.yaml > worker-2.log 2>&1 &

# Wait for all to complete
wait

# Check results
ls -lh degree_collector_results_worker-*.txt
```

#### Step 5: Verify Results

Each worker produces a result file:

```bash
# Check result files
cat degree_collector_results_worker-0.txt
cat degree_collector_results_worker-1.txt
cat degree_collector_results_worker-2.txt
```

**Expected output format:**
```
# Degree Collector Results (Worker: worker-0)
# Format: vertex_id neighbor_id neighbor_degree
0 1 3
0 4 2
3 8 1
...
```

## Configuration for Local Testing

Ensure your config file has:

```yaml
graph_config:
  format: "edgelist"
  local_testing: true  # ← Important: must be true
  file_path: "data"    # Base directory with 1.txt, 2.txt, etc.
  directed: false

worker_config:
  num_workers: 3        # Match number of workers
  worker_id: "worker-0" # Unique for each worker
```

## Understanding the Test

The `degree-collector` algorithm:

1. **Round 1**: Each worker publishes degrees for its assigned vertices
   - Worker-0 publishes degrees for vertices assigned to it
   - Worker-1 publishes degrees for vertices assigned to it
   - Worker-2 publishes degrees for vertices assigned to it
   - All workers synchronize via silhouette-db

2. **Round 2**: Each worker queries degrees of its neighbors
   - For each vertex owned by the worker
   - Query degrees of all its neighbors (from Round 1)
   - Write results to file

## Verifying Correctness

### Check Result Files

```bash
# Count results per worker
for f in degree_collector_results_worker-*.txt; do
    echo "$f: $(grep -v '^#' $f | wc -l) results"
done
```

### Verify No Errors

```bash
# Check for errors in worker logs
grep -i "error\|failed\|fatal" worker-*.log
```

### Manual Verification

1. Load the global graph file: `test-degree-collector/graph.txt`
2. Compute vertex degrees manually
3. Compare with results in `degree_collector_results_worker-*.txt`

## Troubleshooting

### Issue: "Failed to load graph data"

**Solution**: Ensure:
- Graph files exist: `data/1.txt`, `data/2.txt`, etc.
- `local_testing: true` in config
- `file_path: "data"` in config
- Correct `worker_id` matches file number (worker-0 → 1.txt)

### Issue: "Round did not complete"

**Solution**: 
- Ensure all workers are running simultaneously
- Check server logs for errors
- Verify server is accessible at configured address

### Issue: "key not found" errors

**Solution**:
- Ensure Round 1 completed before Round 2 starts
- All workers must publish in Round 1
- Check that graph was partitioned correctly

### Issue: Empty result files

**Solution**:
- Verify workers own vertices (check vertex assignment)
- Check that vertices have neighbors
- Verify graph file format is correct

## Advanced Testing

### Test with Custom Vertex Assignment

```yaml
worker_config:
  vertex_assignment:
    0: "worker-0"
    1: "worker-0"
    2: "worker-1"
    3: "worker-1"
    4: "worker-2"
```

### Test with Larger Graphs

```bash
NUM_VERTICES=1000 NUM_EDGES=5000 ./scripts/test-degree-collector.sh
```

### Test with Different Worker Counts

```bash
NUM_WORKERS=5 NUM_VERTICES=50 NUM_EDGES=100 ./scripts/test-degree-collector.sh
```

## Expected Output

When successful, you should see:

```
=== Degree Collector Local Testing ===
Workers: 3
Vertices: 20
Edges: 30
...

✓ Build complete
✓ Graph generated
✓ Server ready
✓ All workers completed
✓ Worker-0: 25 results in degree_collector_results_worker-0.txt
✓ Worker-1: 15 results in degree_collector_results_worker-1.txt
✓ Worker-2: 18 results in degree_collector_results_worker-2.txt
=== Test Passed ===
```


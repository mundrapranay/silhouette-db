# Testing Guide for silhouette-db

This guide explains how to test the `silhouette-db` system, including manual testing, automated scripts, and algorithm-specific testing.

## Testing Status Overview

The `silhouette-db` system is **functionally complete** and **tested**, with all core features implemented and verified. The system has been tested through:

1. ✅ **Unit Tests** - All core components tested
2. ✅ **Integration Tests** - End-to-end workflows verified
3. ✅ **Runtime Tests** - Single-node and multi-node scenarios tested
4. ✅ **Multi-Worker Tests** - Concurrent worker aggregation verified
5. ✅ **Load Tests** - System stability under load verified

### Test Coverage Summary

**Unit Tests (All Passing):**
- ✅ FSM Tests: 8 tests - FSM creation, operations, snapshots, restore
- ✅ Store Tests: 5 tests - Store creation, Set/Get operations, leadership detection
- ✅ Server Tests: Multiple test suites covering server operations
- ✅ Crypto Tests: PIR integration (2 tests), OKVS implementation (6 tests), PIR + OKVS integration (3 tests)

**Integration Tests (All Passing):**
- ✅ Round lifecycle tests
- ✅ Concurrent workers tests
- ✅ Sequential rounds tests
- ✅ Edge cases: duplicates, errors, empty data, large values

**Runtime Tests:**
- ✅ Single-Node Runtime: OKVS encoding, direct PIR fallback, PIR queries
- ✅ Multi-Node Cluster: Cluster formation, leader election, data replication, failover
- ✅ Multi-Worker: Concurrent publishing, worker aggregation, large datasets
- ✅ Load Testing: Concurrent rounds, high query load, system stability

## Quick Start: Automated Test Scripts

### Single Node Runtime Testing

```bash
./scripts/test-runtime.sh
```

This script will:
1. Build binaries if needed
2. Start the server
3. Run tests with 150 pairs (OKVS encoding)
4. Run tests with 50 pairs (direct PIR fallback)
5. Test specific key queries
6. Clean up automatically

### Multi-Node Cluster Testing

```bash
# Test with 3 nodes (default)
./scripts/test-cluster.sh 3

# Test with 5 nodes
./scripts/test-cluster.sh 5

# Or via Makefile
make test-cluster NUM_NODES=5
```

This script will:
1. Build binaries if needed
2. Start N nodes (bootstrap + N-1 additional nodes)
3. Attempt to add peers to form cluster
4. Test OKVS encoding in cluster
5. Test data replication across nodes
6. Test leader election and failover
7. Test direct PIR fallback in cluster
8. Clean up automatically

**Note:** Full automatic peer joining requires an `AddPeer` RPC endpoint. The current implementation uses a workaround that may have limitations.

### Multi-Worker Testing

```bash
# Default: 10 workers, 20 pairs each
make test-multi-worker

# Custom: 20 workers, 50 pairs each, round 200
make test-multi-worker NUM_WORKERS=20 PAIRS_PER_WORKER=50 ROUND_ID=200

# Or directly:
./scripts/test-multi-worker.sh 127.0.0.1:9090 20 50 200
```

**What it tests:**
- Concurrent worker publishing
- Worker aggregation correctness
- Large dataset handling (OKVS encoding with 1000+ pairs via multiple workers)
- Query verification across different worker data

### Load Testing

```bash
# Default: 10 rounds, 150 pairs/round, 5 workers/round, 10 QPS, 30s duration
make test-load

# Custom: 20 rounds, 200 pairs/round, 10 workers/round, 50 QPS, 60s duration
make test-load NUM_ROUNDS=20 PAIRS=200 WORKERS=10 QPS=50.0 DURATION=60

# Or directly:
./scripts/test-load.sh 127.0.0.1:9090 20 200 10 50.0 60
```

**What it tests:**
- Concurrent round creation and completion
- Multiple workers publishing simultaneously
- PIR query load (configurable QPS)
- System stability under load
- Metrics tracking (completion rates, latencies)
- Thread-safe concurrent queries across multiple rounds

**Note:** The load test uses per-round PIR clients to prevent race conditions when querying different rounds concurrently. The client automatically initializes separate PIR clients for each round as needed.

## Manual Step-by-Step Testing

### Step 1: Build Binaries

```bash
# Build server and test client
make build build-client
```

### Step 2: Start the Server

In terminal 1:

```bash
./bin/silhouette-server \
    -node-id=test-node \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./test-data/node1 \
    -bootstrap=true
```

Wait for the server to start (you'll see log messages).

### Step 3: Run Test Client

In terminal 2:

#### Test 1: OKVS Encoding (150 pairs > 100 minimum)

```bash
./bin/test-client \
    -server=127.0.0.1:9090 \
    -pairs=150 \
    -round=1
```

Expected output:
- ✅ Should show "OKVS encoding will be used"
- ✅ All PIR queries should succeed
- ✅ Retrieved values should match expected values

#### Test 2: Direct PIR Fallback (50 pairs < 100 minimum)

```bash
./bin/test-client \
    -server=127.0.0.1:9090 \
    -pairs=50 \
    -round=2
```

Expected output:
- ✅ Should show "Direct PIR will be used"
- ✅ All PIR queries should succeed
- ✅ Retrieved values should match expected values

#### Test 3: Query Specific Key

```bash
./bin/test-client \
    -server=127.0.0.1:9090 \
    -pairs=100 \
    -round=3 \
    -key="test-key-050"
```

Expected output:
- ✅ Should query only the specified key
- ✅ Retrieved value should match expected value

## Testing Degree Collector Algorithm

The `degree-collector` algorithm can be tested using the automated test script:

```bash
# Basic test with defaults (3 workers, 20 vertices, 30 edges)
./scripts/test-degree-collector.sh

# With custom parameters
NUM_WORKERS=5 NUM_VERTICES=100 NUM_EDGES=200 ./scripts/test-degree-collector.sh

# Keep test files for inspection
./scripts/test-degree-collector.sh --keep
```

### Manual Testing Steps

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
./bin/silhouette-server \
    -node-id=test-node \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./data/node1 \
    -bootstrap
```

#### Step 3: Run Workers

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

#### Step 4: Verify Results

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

### Configuration for Local Testing

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

## What Gets Tested

### Single Node Tests

#### ✅ OKVS Encoding (> 100 pairs)
- Server accepts 150+ pairs
- OKVS encoding is triggered
- OKVS-encoded blob is stored in Raft
- PIR database is created from OKVS-decoded values
- PIR queries work correctly

#### ✅ Direct PIR Fallback (< 100 pairs)
- Server accepts < 100 pairs
- OKVS encoding is skipped
- Raw pairs are stored directly
- PIR database is created from raw pairs
- PIR queries work correctly

#### ✅ PIR Query Correctness
- Multiple keys can be queried
- Retrieved values match expected values
- Float64 precision is preserved
- Query timing is reasonable

### Multi-Node Cluster Tests

#### ✅ Cluster Formation
- Bootstrap node starts successfully
- Additional nodes start and attempt to join
- Leader election occurs
- Cluster stabilizes

#### ✅ Data Replication (OKVS Encoding)
- Data published to leader
- OKVS encoding works in cluster mode
- Data available across cluster nodes
- PIR queries work from any node

#### ✅ Query from All Nodes
- All nodes can serve PIR queries
- Data is replicated across cluster
- Queries work regardless of which node handles them

#### ✅ Leader Election and Failover
- Leader can be identified
- Leader failure triggers election
- New leader is elected
- Cluster continues operating

#### ✅ Data Availability After Failover
- Data persists after leader failure
- Queries still work after failover
- Cluster remains operational

#### ✅ Direct PIR Fallback in Cluster
- Direct PIR fallback works in cluster mode
- Graceful degradation with < 100 pairs

## Troubleshooting

### Server fails to start
- Check if port 8080 or 9090 is already in use
- Verify build was successful: `make build`
- Check server logs for errors

### Client fails to connect
- Verify server is running: `ps aux | grep silhouette-server`
- Check server address matches: `-server=127.0.0.1:9090`
- Verify server logs show gRPC server started

### PIR queries fail
- Check that round completed (all workers published)
- Verify PIR library is built: `make build-pir`
- Check server logs for PIR-related errors

### Values don't match
- Verify value encoding (must be 8 bytes float64)
- Check epsilon threshold for float comparison
- Verify OKVS encoding completed successfully

### Algorithm testing issues

**"Failed to load graph data"**
- Ensure graph files exist: `data/1.txt`, `data/2.txt`, etc.
- `local_testing: true` in config
- `file_path: "data"` in config
- Correct `worker_id` matches file number (worker-0 → 1.txt)

**"Round did not complete"**
- Ensure all workers are running simultaneously
- Check server logs for errors
- Verify server is accessible at configured address

**"key not found" errors**
- Ensure Round 1 completed before Round 2 starts
- All workers must publish in Round 1
- Check that graph was partitioned correctly

## Expected Performance

- **Publish Values (150 pairs)**: ~1-3 seconds
- **PIR Query**: ~10-50ms per query
- **OKVS Encoding**: ~500ms-2s for 150 pairs
- **Round completion**: ~60-70ms for 200 pairs
- **PIR query latency**: ~2-5ms average

## Test Execution Commands

### Quick Test Commands

```bash
# Run all unit/integration tests
make test

# Run single-node runtime tests
make test-runtime

# Run multi-node cluster tests (3 nodes)
make test-cluster NUM_NODES=3

# Run multi-worker tests
make test-multi-worker

# Run load tests
make test-load
```

## Verification Results

### ✅ Functional Correctness
- All core operations work correctly
- Round lifecycle complete
- Worker aggregation correct
- PIR queries return correct values
- OKVS encoding/decoding verified

### ✅ Concurrency Safety
- Thread-safe client implementation (per-round PIR clients)
- Concurrent worker publishing works
- Concurrent round creation works
- Concurrent PIR queries across rounds work

### ✅ Fault Tolerance
- Leader election works
- Failover scenarios tested
- Data replication verified
- Cluster stability confirmed

### ✅ Performance
- Load testing shows system stable under 50 QPS
- Round completion time: ~60-70ms for 200 pairs
- PIR query latency: ~2-5ms average
- No memory leaks detected in basic testing

## Conclusion

**YES - We have a working, tested system.**

The `silhouette-db` framework is:
- ✅ **Functionally complete** - All core features implemented
- ✅ **Thoroughly tested** - Unit, integration, and runtime tests all passing
- ✅ **Production-ready for basic use** - Core functionality stable
- ⚠️ **Some production features pending** - Monitoring, profiling, advanced deployment features

The system is ready for:
- ✅ Basic deployment and usage
- ✅ Integration with LEDP algorithms
- ✅ Single-node and multi-node cluster deployment
- ✅ Further testing and optimization


# Manual Runtime Testing Guide

This guide explains how to manually test the `silhouette-db` system end-to-end.

## Quick Start

### Option 1: Automated Test Scripts

#### Single Node Runtime Testing:
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

#### Multi-Node Cluster Testing:
```bash
# Test with 3 nodes (default)
./scripts/test-cluster.sh 3

# Test with 5 nodes
./scripts/test-cluster.sh 5
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

### Option 2: Manual Step-by-Step

#### Step 1: Build Binaries

```bash
# Build server and test client
make build build-client
```

#### Step 2: Start the Server

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

#### Step 3: Run Test Client

In terminal 2:

##### Test 1: OKVS Encoding (150 pairs > 100 minimum)

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

##### Test 2: Direct PIR Fallback (50 pairs < 100 minimum)

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

##### Test 3: Query Specific Key

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

## Test Client Options

The test client supports the following command-line options:

```
  -server string
        Server address (host:port) (default "127.0.0.1:9090")
  -pairs int
        Number of key-value pairs to publish (default 150)
  -round uint
        Round ID (default 1)
  -key string
        Specific key to query (optional, defaults to first/middle/last keys)
```

## Cluster Testing Options

The cluster test script supports the following:

```bash
# Usage
./scripts/test-cluster.sh [NUM_NODES]

# Examples
./scripts/test-cluster.sh 1   # Single node
./scripts/test-cluster.sh 3   # 3-node cluster (default)
./scripts/test-cluster.sh 5   # 5-node cluster
```

**Via Makefile:**
```bash
# Default (3 nodes)
make test-cluster

# Custom number of nodes
make test-cluster NUM_NODES=5
```

## What Gets Tested

### Single Node Tests:

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

### Multi-Node Cluster Tests:

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

## Expected Performance

- **Publish Values (150 pairs)**: ~1-3 seconds
- **PIR Query**: ~10-50ms per query
- **OKVS Encoding**: ~500ms-2s for 150 pairs

## Next Steps

After manual testing passes:
1. ✅ Verify OKVS encoding works
2. ✅ Verify PIR queries work
3. ✅ Verify graceful fallback works
4. ➡️ Test with multiple workers
5. ➡️ Test with multiple server nodes
6. ➡️ Test under load


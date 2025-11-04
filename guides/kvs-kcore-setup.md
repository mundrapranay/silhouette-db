# Running K-Core Decomposition with KVS Backend

## Quick Start

To run the k-core decomposition algorithm with KVS (simple key-value store) as the storage backend:

```bash
STORAGE_BACKEND=kvs ./scripts/test-kcore-decomposition.sh
```

## Method 1: Environment Variable (Recommended)

The test script respects the `STORAGE_BACKEND` environment variable:

```bash
# Set environment variable and run
export STORAGE_BACKEND=kvs
./scripts/test-kcore-decomposition.sh

# Or run in one line
STORAGE_BACKEND=kvs ./scripts/test-kcore-decomposition.sh
```

## Method 2: Manual Server Startup

If you want to start the server manually with KVS backend:

```bash
# Start server with KVS backend
./bin/silhouette-server \
    -node-id=test-node \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./test-kcore-decomposition/node1 \
    -bootstrap=true \
    -storage-backend=kvs
```

Then run the algorithm runner separately with the worker configuration.

## Method 3: Modify Test Script

You can also modify the script to default to KVS by changing line 66 in `scripts/test-kcore-decomposition.sh`:

```bash
# Change from:
STORAGE_BACKEND=${STORAGE_BACKEND:-okvs}

# To:
STORAGE_BACKEND=${STORAGE_BACKEND:-kvs}
```

## Differences: KVS vs OKVS

| Feature | KVS | OKVS |
|---------|-----|------|
| **Minimum pairs** | Any number | 100+ pairs required |
| **Performance** | Faster | Slower (encoding overhead) |
| **Oblivious storage** | ❌ No | ✅ Yes |
| **CGO required** | ❌ No | ✅ Yes |
| **Use case** | Testing, development | Production (privacy-sensitive) |

## Benefits of KVS for K-Core

- **Faster execution**: No encoding/decoding overhead
- **Works with any number of vertices**: No 100+ pair requirement
- **Easier debugging**: Direct JSON storage format
- **No CGO dependency**: Pure Go implementation

## Verification

After running with KVS, check the server logs to confirm:

```bash
# Look for this line in the server log:
# "Using KVS (simple key-value store) backend"
tail -f test-kcore-decomposition/server.log | grep -i "storage backend"
```

You should see:
```
Using KVS (simple key-value store) backend
```

## Notes

- KVS provides **query privacy** via PIR (same as OKVS)
- KVS does **not** provide **storage privacy** (storage format reveals which keys exist)
- For production use with privacy requirements, use OKVS instead
- KVS is ideal for testing and development


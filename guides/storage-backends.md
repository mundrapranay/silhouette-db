# Storage Backends Guide

This guide explains the storage backend options available in `silhouette-db` and when to use each one.

## Overview

`silhouette-db` supports two storage backends for encoding key-value pairs:

1. **OKVS (Oblivious Key-Value Store)**: Provides oblivious storage properties
2. **KVS (Simple Key-Value Store)**: Fast, simple storage without oblivious properties

Both backends work seamlessly with PIR (Private Information Retrieval) for query privacy, but they differ in their storage encoding and privacy guarantees.

## Storage Backend Comparison

| Feature | OKVS | KVS |
|---------|------|-----|
| **Oblivious Storage** | ✅ Yes | ❌ No |
| **Minimum Pairs** | 100+ pairs required | Any number |
| **Encoding Overhead** | ~10-20% | None (just JSON serialization) |
| **Performance** | Slower (encoding/decoding) | Faster (direct map lookup) |
| **Use Case** | Privacy-sensitive applications | Testing, development, non-private algorithms |
| **CGO Required** | ✅ Yes (Rust FFI) | ❌ No |
| **Storage Format** | RB-OKVS encoded blob | JSON-serialized map |

## OKVS (Oblivious Key-Value Store)

### Overview

OKVS provides **oblivious storage**, meaning the encoded blob reveals no information about which keys are stored. This is useful for privacy-sensitive applications where you want to hide storage patterns.

### Implementation

- **Algorithm**: RB-OKVS (Random Band Matrix OKVS)
- **Library**: `third_party/rb-okvs/` (Rust implementation)
- **FFI Wrapper**: `third_party/rb-okvs-ffi/` (C-compatible interface)
- **Go Bindings**: `internal/crypto/okvs_impl.go` (cgo integration)

### Requirements

- **Minimum 100 pairs**: RB-OKVS requires at least 100 key-value pairs for reliable operation
- **Fixed-size values**: Values must be exactly 8 bytes (float64, little-endian)
- **Key hashing**: Keys are hashed to 8-byte `OkvsKey` using BLAKE2b512
- **CGO enabled**: Requires CGO for Rust FFI integration

### Properties

- **Obliviousness**: The encoded blob reveals nothing about stored keys
- **Decodability**: Any key can be decoded from the blob (with high probability)
- **Compactness**: Size is ~1.1-1.2x the original data size
- **Privacy**: Hides which keys are stored in the database

### Usage

```bash
# Start server with OKVS backend (default)
./bin/silhouette-server \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./data/node1 \
    -bootstrap \
    -storage-backend=okvs
```

### When to Use OKVS

- ✅ Privacy-sensitive applications requiring oblivious storage
- ✅ Production deployments where storage privacy is important
- ✅ Algorithms with 100+ key-value pairs per round
- ✅ When you need to hide which keys are stored

## KVS (Simple Key-Value Store)

### Overview

KVS provides **simple, fast storage** without oblivious properties. It's ideal for testing, development, and algorithms where storage privacy is not required.

### Implementation

- **Format**: JSON-serialized map with base64-encoded values
- **Go Implementation**: `internal/crypto/kvs.go` (pure Go, no CGO)
- **Lookup**: O(1) map lookup after deserialization

### Requirements

- **No minimum pairs**: Works with any number of pairs (1, 10, 1000, etc.)
- **Any value size**: Values can be any size (not limited to 8 bytes)
- **No CGO**: Pure Go implementation, no external dependencies
- **Simple serialization**: JSON + base64 encoding

### Properties

- **Speed**: Fast encoding/decoding (just JSON serialization)
- **Flexibility**: Works with any number of pairs and value sizes
- **Simplicity**: Easy to understand and debug
- **No obliviousness**: Storage format reveals which keys are stored

### Usage

```bash
# Start server with KVS backend
./bin/silhouette-server \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./data/node1 \
    -bootstrap \
    -storage-backend=kvs
```

### When to Use KVS

- ✅ Testing and development
- ✅ Algorithms with fewer than 100 pairs
- ✅ Non-privacy-sensitive applications
- ✅ When performance is critical
- ✅ When CGO is not available or desired
- ✅ Exact algorithms (non-private)

## Choosing the Right Backend

### Decision Tree

```
Do you need oblivious storage?
├─ Yes → Use OKVS (if you have 100+ pairs)
│         └─ If <100 pairs → Use KVS (OKVS not available)
└─ No → Use KVS (faster, simpler)
```

### Recommendations

1. **Privacy-sensitive production**: Use OKVS
2. **Testing/development**: Use KVS
3. **<100 pairs**: Use KVS (OKVS requires 100+)
4. **Performance-critical**: Use KVS
5. **No CGO available**: Use KVS

## Server Configuration

### Command-Line Flag

The server accepts a `--storage-backend` flag to choose the backend:

```bash
-storage-backend=okvs  # Use OKVS (default)
-storage-backend=kvs   # Use KVS
```

### Default Behavior

If not specified, the server defaults to **OKVS** backend.

### Environment Variable

Test scripts support the `STORAGE_BACKEND` environment variable:

```bash
# Test with KVS backend
STORAGE_BACKEND=kvs ./scripts/test-degree-collector.sh

# Test with OKVS backend
STORAGE_BACKEND=okvs ./scripts/test-kcore-decomposition.sh
```

## Integration with PIR

Both backends work seamlessly with PIR (Private Information Retrieval) for query privacy:

1. **Storage Backend**: Encodes/decodes key-value pairs (OKVS or KVS)
2. **PIR Layer**: Provides query privacy (hides which key is queried)

### Data Flow

```
Workers → Publish Key-Value Pairs
    ↓
Server → Storage Backend (OKVS or KVS)
    ↓
    Encode → Storage Blob
    Decode → Key-Value Pairs for PIR
    ↓
PIR Server → Create Database from Decoded Pairs
    ↓
Queries → Private Retrieval via PIR
```

### Privacy Guarantees

- **OKVS + PIR**: Both storage and query privacy
- **KVS + PIR**: Query privacy only (storage patterns visible)

## Testing

### KVS Tests

```bash
# Run KVS unit tests
make test-kvs

# Run KVS integration tests
make test-kvs-integration

# Run KVS benchmarks
make bench-kvs
```

### OKVS Tests

```bash
# Run OKVS unit tests (requires cgo)
make test-okvs-unit

# Run OKVS integration tests (requires cgo)
make test-okvs-integration

# Run OKVS benchmarks (requires cgo)
make bench-okvs
```

### Comparison Tests

```bash
# Compare KVS vs OKVS performance
make bench-kvs-vs-okvs

# Run end-to-end tests with both backends
make test-e2e-backends
```

## Performance Considerations

### Encoding Performance

- **OKVS**: Slower encoding (~10-20% overhead), requires 100+ pairs
- **KVS**: Fast encoding (just JSON serialization), any number of pairs

### Decoding Performance

- **OKVS**: Slower decoding (FFI overhead), but oblivious
- **KVS**: Fast decoding (O(1) map lookup), but not oblivious

### Memory Usage

- **OKVS**: Compact storage (~1.1-1.2x original size)
- **KVS**: Larger storage (JSON + base64 overhead)

### When Performance Matters

- **Many small rounds**: Use KVS (faster encoding/decoding)
- **Large datasets**: Use OKVS (more compact storage)
- **Real-time algorithms**: Use KVS (lower latency)
- **Batch processing**: Use OKVS (better storage efficiency)

## Migration

### Switching Between Backends

You can switch backends at any time by restarting the server with a different `--storage-backend` flag. Note that:

- Existing data in Raft is stored in the backend format used at publish time
- Queries use the same backend format as the round's storage
- You cannot decode OKVS data with KVS decoder (or vice versa)

### Best Practices

1. **Consistency**: Use the same backend for all rounds in a single algorithm execution
2. **Testing**: Test with both backends to ensure compatibility
3. **Documentation**: Document which backend is used in your deployment
4. **Monitoring**: Monitor performance differences between backends

## Troubleshooting

### Common Issues

1. **"OKVS requires at least 100 pairs"**
   - Solution: Use KVS backend or increase the number of pairs

2. **"CGO not available"**
   - Solution: Use KVS backend (doesn't require CGO)

3. **"Performance too slow"**
   - Solution: Try KVS backend for faster encoding/decoding

4. **"Storage format incompatible"**
   - Solution: Ensure you use the same backend for encoding and decoding

## References

- [OKVS Integration Plan](./okvs-integration-plan.md) - Detailed OKVS implementation guide
- [PIR Integration Guide](./pir-integration.md) - PIR integration documentation
- [KVS Integration Plan](../KVS_INTEGRATION_PLAN.md) - KVS implementation details
- [Complete Guide](./guide.md) - Full system architecture


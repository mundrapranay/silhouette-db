# FrodoPIR Integration Guide

This guide explains how to integrate and use FrodoPIR in the `silhouette-db` project.

## Overview

FrodoPIR provides Private Information Retrieval (PIR) functionality, allowing clients to query a server database without revealing which item they're querying. The integration consists of three layers:

1. **Rust FFI Wrapper** (`third_party/frodo-pir-ffi/`) - C-compatible interface to FrodoPIR
2. **Go cgo Bindings** (`internal/crypto/pir.go`) - Go interface to the Rust FFI
3. **Server/Client Integration** - Usage in the gRPC server and client library

## Building

### Prerequisites

1. **Rust and Cargo** (>= 1.61.0)
   ```bash
   curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
   ```

2. **cbindgen** (for generating C headers)
   ```bash
   cargo install cbindgen
   ```

### Building the Rust FFI Library

```bash
# Build the static library
make build-pir

# Or manually:
cd third_party/frodo-pir-ffi
cargo build --release
```

This creates:
- `target/release/libfrodopirffi.a` - Static library (17MB)
- `target/release/libfrodopirffi.dylib` - Dynamic library (446KB, macOS)
- `frodopir_ffi.h` - C header file (5.5KB)

### Building the Go Code

```bash
# Build with cgo support
go build -tags cgo ./...

# Or using the Makefile
make build
```

## Architecture

### Server Side

The server maintains a FrodoPIR **Shard** for each round:

1. **Publish Phase**: Workers submit key-value pairs
2. **Aggregation**: Server collects all pairs
3. **FrodoPIR Setup**: 
   - Convert pairs to base64-encoded strings
   - Create FrodoPIR Shard with these strings
   - Extract BaseParams (serialized) for client distribution
4. **Storage**: Store both OKVS blob and FrodoPIR shard
5. **Query Phase**: Use shard to process PIR queries

### Client Side

The client needs:

1. **BaseParams**: Downloaded from server (serialized)
2. **Key-to-Index Mapping**: Maps keys to database row indices
3. **Query Generation**: Creates PIR query for specific index
4. **Response Decoding**: Decodes server response using QueryParams

## Usage

### Server Side

```go
import "github.com/mundrapranay/silhouette-db/internal/crypto"

// After collecting all key-value pairs in a round
pairs := map[string][]byte{
    "key1": []byte("value1"),
    "key2": []byte("value2"),
    // ...
}

// Create FrodoPIR server
lweDim := 512        // LWE dimension (512, 1024, or 1572)
elemSize := 8192     // Element size in bits
plaintextBits := 10 // Plaintext bits (10 or 9)

pirServer, baseParams, err := crypto.NewFrodoPIRServer(pairs, lweDim, elemSize, plaintextBits)
if err != nil {
    return err
}
defer pirServer.Close()

// Store baseParams for client distribution
// baseParams is []byte - serialize and store it

// Process queries
response, err := pirServer.ProcessQuery(nil, queryBytes)
if err != nil {
    return err
}
```

### Client Side

```go
import "github.com/mundrapranay/silhouette-db/internal/crypto"

// Get baseParams from server (deserialized)
baseParams := []byte{...} // Downloaded from server

// Create key-to-index mapping
// This should match the server's ordering
keyToIndex := map[string]int{
    "key1": 0,
    "key2": 1,
    // ...
}

// Create FrodoPIR client
pirClient, err := crypto.NewFrodoPIRClient(baseParams, keyToIndex)
if err != nil {
    return err
}
defer pirClient.Close()

// Generate query for a key
query, queryParams, err := pirClient.GenerateQuery("key1")
if err != nil {
    return err
}

// Send query to server (using gRPC client)
response, err := grpcClient.GetValue(ctx, roundID, query)
if err != nil {
    return err
}

// Decode response
value, err := pirClient.DecodeResponse(response.PirResponse, queryParams)
if err != nil {
    return err
}
```

## Key-to-Index Mapping

FrodoPIR queries by **index** (0, 1, 2, ...), not by key. The client needs a mapping from keys to indices.

**Options:**

1. **Server provides mapping** (simplest):
   - Server creates mapping when initializing shard
   - Server exposes mapping via API endpoint
   - Client downloads mapping along with BaseParams

2. **OKVS for mapping** (more private):
   - Use OKVS to encode key→index mapping
   - Client queries OKVS using PIR to find index
   - Then queries main database using found index

3. **Separate index service** (most flexible):
   - Dedicated service for key→index lookups
   - Can use different privacy-preserving techniques

**Current Implementation:** The mapping must be provided manually. In production, implement one of the options above.

## Parameters

### LWE Dimension (`lweDim`)

- **512**: Small databases (< 1K elements)
- **1024**: Medium databases (1K-100K elements)
- **1572**: Large databases (> 100K elements)

Choose based on database size and security requirements.

### Element Size (`elemSize`)

Size of each database element in **bits**. Should match the size of your values (padded to bits).

### Plaintext Bits (`plaintextBits`)

- **10 bits**: For databases with 16 ≤ log₂(m) ≤ 18
- **9 bits**: For databases with log₂(m) ≤ 20

Where `m` is the number of database elements.

## Memory Management

All memory allocated by FFI functions must be freed:

```go
// Buffers returned by FFI are automatically freed by the Go wrapper
// But shard/client handles must be explicitly closed
defer pirServer.Close()
defer pirClient.Close()
```

## Error Handling

The FFI wrapper returns error codes:

- `FrodoPIRResultSuccess` (0) - Success
- `FrodoPIRResultInvalidInput` (1) - Invalid parameters
- `FrodoPIRResultSerializationError` (2) - Serialization failed
- `FrodoPIRResultDeserializationError` (3) - Deserialization failed
- `FrodoPIRResultQueryParamsReused` (4) - QueryParams already used (non-retryable)
- `FrodoPIRResultOverflownAdd` (5) - Overflow in addition during query generation (retryable)
- `FrodoPIRResultNotFound` (6) - Resource not found
- `FrodoPIRResultUnknownError` (99) - Unknown error

### Overflow Error and Retry Logic

**Known Limitation:** FrodoPIR query generation can occasionally fail with an `OverflownAdd` error (error code 5). This occurs when random values in the query parameters are very close to `u32::MAX`, and adding the query indicator causes an arithmetic overflow.

**Retry Logic:** The `GenerateQuery` method automatically implements retry logic for overflow errors:
- Retries up to 3 times by default
- Each retry creates new random query parameters
- Since the overflow is probabilistic, retries usually succeed
- Non-retryable errors (invalid input, params reused) fail immediately

**Why This Happens:**
- FrodoPIR uses randomness to generate secure query parameters
- The query indicator is added to random values in the parameter vector
- If these values are close to the maximum `u32` value, overflow can occur
- This is a fundamental limitation of the protocol's arithmetic operations

**Mitigation:**
- The retry logic handles this automatically in most cases
- If retries fail, consider adjusting PIR parameters (larger `lweDim`, different `plaintextBits`)
- In production, monitor retry rates and adjust parameters if needed

## Testing

```bash
# Test Rust FFI library
make test-pir

# Test Go bindings
go test -tags cgo ./internal/crypto/...

# Run all tests
make test

# Run PIR benchmarks
make bench-pir
```

## Performance Benchmarks

Running `make bench-pir` performs several PIR operation benchmarks. Here's how to interpret the results:

### Benchmark Results Example

```
BenchmarkPIR_ShardCreation-11         94    12432091 ns/op    3867092 B/op    412 allocs/op
|                                      |            |              |          |  
|                                      │            │              │          └─ Number of allocations per operation
|                                      │            │              └─────────── Bytes allocated per operation
|                                      │            └────────────────────────── Nanoseconds per operation
|                                      └────────────────────────────────────── Number of iterations run
└─────────────────────────────────────────────────────────────── Benchmark name

BenchmarkPIR_QueryGeneration-11     1375      879287 ns/op      9528 B/op     13 allocs/op
BenchmarkPIR_QueryProcessing-11   157794        7654 ns/op      6928 B/op      4 allocs/op
BenchmarkPIR_EndToEnd-11            1107     1077417 ns/op     18529 B/op     21 allocs/op
```

### Understanding the Metrics

Each benchmark line shows:
- **Benchmark Name**: Operation being measured
- **Iterations**: Number of times the operation ran (e.g., `94`, `1375`)
- **Time per Operation**: `ns/op` - nanoseconds per operation
- **Memory per Operation**: `B/op` - bytes allocated per operation
- **Allocations per Operation**: `allocs/op` - number of memory allocations

### Performance Characteristics

**1. Shard Creation (`BenchmarkPIR_ShardCreation`)**
- **Purpose**: Measures time to create a FrodoPIR server from key-value pairs
- **Typical Performance**: ~12-13ms per shard creation
- **Memory**: ~3.8MB per shard (for 100 elements)
- **When This Matters**: Server initialization, round setup
- **Optimization**: Happens once per round, not per query

**2. Query Generation (`BenchmarkPIR_QueryGeneration`)**
- **Purpose**: Measures time to generate a PIR query for a key
- **Typical Performance**: ~0.8-0.9ms per query generation
- **Memory**: ~9.5KB per query
- **When This Matters**: Client-side query creation before sending to server
- **Note**: Includes retry logic overhead for overflow errors (if any occur)

**3. Query Processing (`BenchmarkPIR_QueryProcessing`)**
- **Purpose**: Measures server-side query processing time
- **Typical Performance**: ~7-8μs per query (very fast!)
- **Memory**: ~6.9KB per query
- **When This Matters**: Server query handling latency
- **Note**: This is the fastest operation - server can handle many queries per second

**4. End-to-End (`BenchmarkPIR_EndToEnd`)**
- **Purpose**: Measures complete workflow: query generation + processing + decoding
- **Typical Performance**: ~1.0-1.1ms per complete query
- **Memory**: ~18.5KB per query
- **When This Matters**: Total client-to-server round-trip time
- **Breakdown**: Query generation (~0.9ms) + Processing (~0.008ms) + Decoding (~0.1ms)

### Performance Insights

**Query Processing is Fast**
- Server-side processing is extremely fast (~8μs)
- This allows high-throughput query handling
- Bottleneck is typically network, not computation

**Query Generation Dominates Latency**
- Query generation takes ~90% of total time (~0.9ms out of ~1.1ms)
- This is client-side work (happens before network call)
- Acceptable for most use cases

**Memory Usage**
- Shard creation allocates significant memory (~3.8MB for 100 elements)
- This is stored server-side, shared across all queries for a round
- Per-query memory is relatively small (~10-20KB)

**Scalability Considerations**
- Processing time per query is constant (O(1)) - very scalable
- Shard creation time increases with database size
- Memory grows linearly with number of elements

### Benchmarking Your Own Data

To benchmark with your specific parameters:

```bash
# Run specific benchmark
go test -tags cgo -bench=BenchmarkPIR_ShardCreation -benchmem ./internal/server/...

# Run with custom parameters (modify test file)
# Adjust database size, element size, or LWE dimension
```

**Factors Affecting Performance:**
- **Database Size (`m`)**: Larger databases increase shard creation time
- **Element Size (`elemSize`)**: Larger elements use more memory
- **LWE Dimension (`lweDim`)**: Larger dimensions increase security but slow operations
- **Plaintext Bits (`plaintextBits`)**: Affects query size and processing time

## Troubleshooting

### Build Issues

**"cannot find frodopir_ffi.h"**
- Ensure `third_party/frodo-pir-ffi/frodopir_ffi.h` exists
- Run `make build-pir` to generate it

**"cannot find libfrodopirffi.a"**
- Ensure `third_party/frodo-pir-ffi/target/release/libfrodopirffi.a` exists
- Run `make build-pir` to build it

**"undefined reference" errors**
- Ensure `-lfrodopirffi` is in LDFLAGS
- Check library path in cgo directives

### Runtime Issues

**"failed to create shard: error code X"**
- Check parameters (lweDim, elemSize, plaintextBits)
- Verify database elements are valid base64 strings
- Ensure database is not empty

**"key not found"**
- Verify key-to-index mapping is correct
- Ensure mapping matches server's ordering

**"QueryParams already used"**
- Each QueryParams can only be used once
- Create new QueryParams for each query

**"Overflow in addition" (error code 5)**
- This is a probabilistic error that occurs during query generation
- The client automatically retries up to 3 times
- If retries fail, consider adjusting PIR parameters
- See "Overflow Error and Retry Logic" section above for details

## Next Steps

1. ✅ Rust FFI wrapper created
2. ✅ Go cgo bindings implemented
3. ⏳ Server integration (store shards per round)
4. ⏳ Client integration (key-to-index mapping)
5. ⏳ End-to-end testing
6. ⏳ Performance benchmarking

## References

- [FrodoPIR Paper](https://eprint.iacr.org/2022/981)
- [FrodoPIR Repository](https://github.com/brave-experiments/frodo-pir)
- [Go cgo Documentation](https://pkg.go.dev/cmd/cgo)


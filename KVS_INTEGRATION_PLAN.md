# Simple KV Store Integration Plan

## Overview

Add a simple key-value store option alongside OKVS, allowing users to choose between:
- **OKVS (Oblivious Key-Value Store)**: Provides oblivious storage but requires 100+ pairs and has encoding overhead
- **KVS (Simple Key-Value Store)**: Direct storage without oblivious properties, but faster and works with any number of pairs

## Architecture

### Current State
- `OKVSEncoder` interface defines encoding of key-value pairs
- Server uses `crypto.OKVSEncoder` to encode pairs before PIR
- Currently uses `RBOKVSEncoder` (requires 100+ pairs) or falls back to direct PIR (<100 pairs)
- Server automatically chooses OKVS if pairs >= 100

### Target State
- Add `KVSEncoder` that implements `OKVSEncoder` interface (simple wrapper)
- Add `KVSDecoder` that implements `OKVSDecoder` interface
- Add configuration option to choose storage backend: `"okvs"` or `"kvs"`
- Server respects config choice instead of auto-selecting
- Both paths work with PIR for query privacy

## Implementation Plan

### Phase 1: Create Simple KV Store Implementation

**Files to create:**
- `internal/crypto/kvs.go` - Simple KV encoder/decoder implementation

**Key Components:**
1. **KVSEncoder**: Implements `OKVSEncoder` interface
   - Stores pairs as a serialized map (e.g., JSON, msgpack, or simple binary format)
   - No encoding overhead (just serialization)
   - Works with any number of pairs (no minimum requirement)

2. **KVSDecoder**: Implements `OKVSDecoder` interface
   - Deserializes the blob and looks up values by key
   - Fast O(1) lookup using Go map

3. **Serialization Format:**
   - Option 1: JSON (simple, readable, but larger)
   - Option 2: msgpack (binary, more compact)
   - Option 3: Custom binary format (most compact, fastest)
   - **Recommendation**: Start with JSON for simplicity, can optimize later

### Phase 2: Update Server Configuration

**Files to modify:**
- `cmd/silhouette-server/main.go` - Add config flag/option
- `internal/server/server.go` - Respect storage backend choice

**Changes:**
1. Add `--storage-backend` flag (options: `okvs`, `kvs`, default: `okvs`)
2. Server constructor accepts storage backend choice
3. Create appropriate encoder based on choice
4. Remove auto-selection logic (>=100 pairs check)

### Phase 3: Update Algorithm Configuration

**Files to modify:**
- `algorithms/common/algorithm.go` - Add storage backend to config
- Algorithm config files - Add `storage_backend` option

**Changes:**
1. Add `StorageBackend string` field to algorithm config
2. Pass to server/client if needed, or keep server-level

### Phase 4: Tests

**Files to create:**
- `internal/crypto/kvs_test.go` - Unit tests for KVS encoder/decoder

**Test Cases:**
1. **Encoding Tests:**
   - Encode empty map (should work)
   - Encode single pair
   - Encode 10 pairs
   - Encode 1000 pairs
   - Encode with special characters in keys
   - Encode with various value sizes

2. **Decoding Tests:**
   - Decode existing key
   - Decode non-existent key (should return error)
   - Decode all keys from encoded blob
   - Round-trip: encode then decode all pairs

3. **Integration Tests:**
   - Server with KVS encoder
   - Server with OKVS encoder
   - Compare behavior (both should work identically from client perspective)

### Phase 5: Benchmarks

**Files to create:**
- `internal/crypto/kvs_bench_test.go` - Benchmark KVS vs OKVS

**Benchmark Cases:**
1. **Encoding Performance:**
   - Encode 10 pairs (OKVS should fail, KVS should work)
   - Encode 100 pairs (both should work)
   - Encode 1000 pairs (both should work)
   - Compare encoding time, memory usage

2. **Decoding Performance:**
   - Decode single key from 100 pairs
   - Decode single key from 1000 pairs
   - Decode all keys from 100 pairs
   - Compare decoding time

3. **End-to-End Performance:**
   - Publish 100 pairs, query one (KVS path)
   - Publish 100 pairs, query one (OKVS path)
   - Compare total round time

### Phase 6: Integration & End-to-End Testing

**Files to create/modify:**
- `scripts/test-kvs.sh` - Test script for KVS mode
- `scripts/test-okvs.sh` - Test script for OKVS mode
- Update existing test scripts to support both modes

**Test Scenarios:**
1. **Single Worker Test:**
   - Run degree-collector with KVS
   - Run degree-collector with OKVS
   - Verify same results

2. **Multi-Worker Test:**
   - Run k-core decomposition with KVS
   - Run k-core decomposition with OKVS
   - Verify same results

3. **Performance Comparison:**
   - Run same algorithm with both backends
   - Compare execution time
   - Compare memory usage

## Implementation Details

### KVS Encoder/Decoder Implementation

```go
// KVSEncoder implements OKVSEncoder using simple key-value storage
type KVSEncoder struct{}

// KVSDecoder implements OKVSDecoder using simple key-value storage
type KVSDecoder struct {
    pairs map[string][]byte
}

// Encode serializes the map to JSON
func (e *KVSEncoder) Encode(pairs map[string][]byte) ([]byte, error) {
    // Serialize to JSON or binary format
    // Return blob
}

// Decode deserializes and looks up value
func (d *KVSDecoder) Decode(blob []byte, key string) ([]byte, error) {
    // Deserialize blob
    // Look up key in map
    // Return value or error
}
```

### Server Configuration

```go
type Server struct {
    // ...
    storageBackend string // "okvs" or "kvs"
    okvsEncoder crypto.OKVSEncoder
    // ...
}

func NewServer(s *store.Store, storageBackend string) *Server {
    var encoder crypto.OKVSEncoder
    switch storageBackend {
    case "kvs":
        encoder = crypto.NewKVSEncoder()
    case "okvs":
        encoder = crypto.NewRBOKVSEncoder()
    default:
        encoder = crypto.NewRBOKVSEncoder() // Default
    }
    // ...
}
```

## File Structure

```
internal/crypto/
├── okvs.go              # Interface definitions (unchanged)
├── okvs_impl.go         # RB-OKVS implementation (unchanged)
├── kvs.go               # NEW: Simple KV store implementation
├── kvs_test.go          # NEW: KVS unit tests
├── kvs_bench_test.go    # NEW: KVS benchmarks
└── ...

cmd/silhouette-server/
└── main.go              # MODIFY: Add storage backend flag

internal/server/
└── server.go            # MODIFY: Accept storage backend choice

scripts/
├── test-kvs.sh          # NEW: Test with KVS
└── test-okvs.sh         # NEW: Test with OKVS
```

## Testing Strategy

### Unit Tests
- ✅ KVS encoder/decoder correctness
- ✅ Edge cases (empty, single pair, large datasets)
- ✅ Error handling (invalid keys, corrupted data)

### Integration Tests
- ✅ Server with KVS encoder
- ✅ Server with OKVS encoder
- ✅ PIR queries work with both backends

### End-to-End Tests
- ✅ Algorithms work with KVS
- ✅ Algorithms work with OKVS
- ✅ Results are identical (given same inputs)

### Benchmarks
- ✅ Encoding performance comparison
- ✅ Decoding performance comparison
- ✅ Memory usage comparison
- ✅ End-to-end round time comparison

## Success Criteria

1. ✅ KVS encoder/decoder implemented and tested
2. ✅ Server accepts storage backend choice
3. ✅ Both KVS and OKVS work with existing algorithms
4. ✅ Benchmarks show performance characteristics
5. ✅ End-to-end tests pass for both backends
6. ✅ Documentation updated

## Timeline

1. **Phase 1**: Create KVS implementation (1-2 hours)
2. **Phase 2**: Update server configuration (30 min)
3. **Phase 3**: Update algorithm configs (30 min)
4. **Phase 4**: Write tests (1-2 hours)
5. **Phase 5**: Write benchmarks (1 hour)
6. **Phase 6**: Integration & E2E testing (1-2 hours)

**Total**: ~5-8 hours

## Next Steps

1. Implement KVS encoder/decoder
2. Write unit tests
3. Update server to support backend choice
4. Write benchmarks
5. Run end-to-end tests
6. Document usage


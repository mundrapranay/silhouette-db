# OKVS Integration Plan

## Overview

This document outlines the plan for integrating RB-OKVS (Random Band Matrix OKVS) into the `silhouette-db` framework, following the successful FrodoPIR integration pattern.

## Current Status

**Current Implementation:**
- ✅ Mock OKVS encoder exists (`internal/crypto/okvs.go`)
- ✅ OKVS interface defined (`OKVSEncoder`, `OKVSDecoder`)
- ✅ Server code uses OKVS encoder (line 134 in `server.go`)
- ✅ RB-OKVS implementation found and added as submodule (`third_party/rb-okvs`)
- ✅ RB-OKVS library tested and verified for float64 use case
- ✅ All float64 tests passing (7 tests)
- ❌ FFI wrapper not yet created
- ❌ Go cgo bindings not yet implemented
- ❌ OKVS encoding result stored but not used with PIR (currently using raw key-value pairs)

**Architecture:**
- Currently, FrodoPIR works directly with raw key-value pairs
- OKVS blob is created and stored but not used in PIR queries
- Need to integrate OKVS encoding **before** PIR to achieve complete obliviousness

## Research Results ✅ COMPLETED

### 1. RB-OKVS Implementation Source

**Status: ✅ FOUND**

- **Repository**: `https://github.com/felicityin/rb-okvs`
- **Language**: Rust ✅ (matches FrodoPIR pattern)
- **Location**: Added as git submodule in `third_party/rb-okvs`
- **Maintainability**: Active Rust project with proper structure

### 2. Library Details

- **API**: Uses trait-based design (`Okvs`, `OkvsK`, `OkvsV` traits)
- **Key Type**: `OkvsKey<const N: usize = 8>` (fixed-size 8-byte keys by default)
- **Value Type**: `OkvsValue<const N: usize>` (fixed-size values)
- **Main Struct**: `RbOkvs` (Random Band OKVS implementation)
- **Methods**: `encode()` and `decode()`

### 3. Test Results

**Test Suite**: `third_party/rb-okvs/tests/float64_use_case.rs`

- ✅ All 7 tests passing
- ✅ Verified string keys → float64 values conversion
- ✅ Key hashing using BLAKE2b512 to 8-byte `OkvsKey`
- ✅ Float64 to `OkvsValue<8>` conversion (little-endian)
- ✅ Precision preservation verified
- ✅ Special f64 values (NaN, Infinity) handled correctly
- ✅ Library requires 100+ pairs for reliable operation (documented in tests)

### 4. Library Limitations Identified

- Requires minimum ~100 key-value pairs for reliable operation
- Fixed-size values required (we use 8 bytes for f64)
- String keys must be hashed to fixed-size `OkvsKey`
- Encoding size overhead: ~10-20% (epsilon = 0.1)

## Implementation Strategy

Based on successful FrodoPIR integration, we'll follow the same pattern:

### Phase 1: Research and Selection ✅ COMPLETED

1. **✅ RB-OKVS Implementation Found:**
   - Repository: `felicityin/rb-okvs` on GitHub
   - Language: Rust (matches FrodoPIR pattern)
   - Added as git submodule: `third_party/rb-okvs`

2. **✅ Library Testing:**
   - Created comprehensive test suite for float64 use case
   - All 7 tests passing
   - Verified key hashing, encoding, decoding, precision
   - Documented library limitations (100+ pairs required)

### Phase 2: FFI Wrapper Creation ⚠️ NEXT STEP

Similar to `third_party/frodo-pir-ffi/`:

1. **Create FFI Wrapper Directory:**
   ```
   third_party/okvs-ffi/
   ├── Cargo.toml (or CMakeLists.txt for C++)
   ├── src/lib.rs (or .cpp/.h files)
   ├── build.rs
   └── cbindgen.toml (or header generation)
   ```

2. **FFI Functions Needed:**
   ```rust
   // Note: Our use case is string keys → float64 values
   // Keys will be hashed to 8-byte OkvsKey using BLAKE2b512
   // Values are f64 (8 bytes) converted to OkvsValue<8>

   // Encode: map[string]float64 → OKVS blob
   fn rb_okvs_encode(
       pairs: *const u8,        // Serialized (key, value) pairs
       pairs_len: usize,        // Number of pairs
       encoding_out: *mut *mut u8,   // Output OKVS encoding
       encoding_len: *mut usize       // Length of encoding
   ) -> c_int

   // Decode: OKVS blob + key → float64 value
   fn rb_okvs_decode(
       encoding: *const u8,      // OKVS encoding blob
       encoding_len: usize,
       key: *const u8,           // String key (will be hashed)
       key_len: usize,
       value_out: *mut *mut u8,  // Output f64 value (8 bytes)
       value_len: *mut usize
   ) -> c_int
   ```

3. **Design Decisions:**
   - Keys: Hash string keys to `OkvsKey<8>` using BLAKE2b512
   - Values: Convert f64 to `OkvsValue<8>` using little-endian bytes
   - Serialization: Use bincode for Rust ↔ C data transfer
   - Memory: Follow same pattern as FrodoPIR FFI (owned buffers)

### Phase 3: Go cgo Bindings

1. **Create `internal/crypto/okvs_impl.go`:**
   - Similar structure to `pir.go`
   - Implement `RBOKVSEncoder` struct (implements `OKVSEncoder`)
   - Implement `RBOKVSDecoder` struct (implements `OKVSDecoder`)
   - Use cgo to call FFI functions
   - Handle key hashing (BLAKE2 → 8 bytes) in Go
   - Handle f64 conversion (binary encoding)

2. **Interface Implementation:**
   ```go
   // RBOKVSEncoder implements OKVSEncoder for float64 values
   type RBOKVSEncoder struct {
       // No state needed - stateless encoding
   }

   func (e *RBOKVSEncoder) Encode(pairs map[string][]byte) ([]byte, error) {
       // Convert map[string][]byte to (key, f64) pairs
       // Hash keys to 8 bytes using BLAKE2
       // Convert values from []byte to f64, then to OkvsValue<8>
       // Call rb_okvs_encode FFI function
       // Return serialized OKVS encoding
   }

   // RBOKVSDecoder implements OKVSDecoder for float64 values
   type RBOKVSDecoder struct {
       encoding []byte  // Store OKVS encoding blob
   }

   func (d *RBOKVSDecoder) Decode(okvsBlob []byte, key string) ([]byte, error) {
       // Hash key to 8-byte OkvsKey using BLAKE2
       // Call rb_okvs_decode FFI function
       // Convert returned OkvsValue<8> to f64, then to []byte
       // Return value as []byte
   }
   ```

3. **Key Design Details:**
   - **Key Hashing**: Use Go's `golang.org/x/crypto/blake2b` for key hashing (BLAKE2b512 → 8 bytes)
   - **Value Conversion**: Use `encoding/binary` for f64 ↔ []byte conversion (little-endian)
   - **Memory Management**: Follow FrodoPIR pattern - allocate buffers in Rust, free in Go

### Phase 4: Server Integration

1. **Update `server.go`:**
   - Replace `MockOKVSEncoder` with `RBOKVSEncoder`
   - Ensure OKVS encoding happens before PIR shard creation
   - Store OKVS blob in Raft (already done)

2. **PIR Integration:**
   - **Option A:** PIR queries on OKVS-encoded blob (recommended for complete obliviousness)
   - **Option B:** Keep current approach (PIR on raw pairs) but store OKVS blob
   - **Decision:** Need to verify if FrodoPIR can work with OKVS-encoded blobs or if we need a different approach

### Phase 5: Testing

1. **Unit Tests:**
   - Test encoding/decoding with various key-value sets
   - Test with different sizes
   - Verify obliviousness properties

2. **Integration Tests:**
   - Test full flow: encode → store → retrieve via PIR
   - Test server integration

3. **Benchmarks:**
   - Encoding performance
   - Decoding performance
   - Size overhead

## Key Questions Answered ✅

1. **Implementation Source:**
   - ✅ **Yes**: Open-source RB-OKVS implementation found at `felicityin/rb-okvs`
   - ✅ **Rust**: Written in Rust (matches FrodoPIR integration pattern)
   - ✅ **Maintained**: Active repository with proper structure

2. **Architecture:**
   - ✅ **OKVS + PIR Flow**: OKVS encodes key-value pairs into an oblivious structure, then PIR queries this structure
   - ✅ **Approach**: PIR should query the OKVS-encoded blob (complete obliviousness)
   - ✅ **Size Overhead**: ~10-20% (epsilon = 0.1 means ~1.1x original size)

3. **Parameters:**
   - ✅ **Parameters**: RB-OKVS needs only `kv_count` (number of key-value pairs)
   - ✅ **Internal**: Library calculates `columns = (1 + epsilon) * kv_count` and `band_width` automatically
   - ✅ **No Manual Tuning**: Unlike FrodoPIR, RB-OKVS handles parameter selection internally
   - ✅ **Minimum Size**: Requires ~100+ pairs for reliable operation (matrix rank requirement)

4. **Use Case Details:**
   - ✅ **Keys**: String keys → hashed to `OkvsKey<8>` using BLAKE2b512
   - ✅ **Values**: float64 values → converted to `OkvsValue<8>` (8 bytes, little-endian)
   - ✅ **Conversion**: Tested and verified in `tests/float64_use_case.rs`

## Next Steps

### ✅ Phase 1 Complete: Research and Testing
- RB-OKVS library found and added as submodule
- Test suite created and all tests passing
- Library limitations documented

### 🚧 Phase 2: FFI Wrapper Creation (NEXT)

1. **Create FFI Wrapper Directory:**
   ```bash
   mkdir -p third_party/rb-okvs-ffi
   ```

2. **Set up Rust FFI Project:**
   - Create `Cargo.toml` with `rb-okvs` as dependency
   - Create `src/lib.rs` with FFI functions
   - Add `cbindgen.toml` for C header generation
   - Add `build.rs` for build script
   - Reference: Use `third_party/frodo-pir-ffi/` as template

3. **Implement FFI Functions:**
   - `rb_okvs_encode`: Take serialized (key, f64) pairs, return OKVS encoding
   - `rb_okvs_decode`: Take OKVS encoding + key, return f64 value
   - Handle key hashing (BLAKE2b512 → 8 bytes) in Rust
   - Handle f64 conversion (little-endian bytes)

4. **Build and Test:**
   - Build static library: `cargo build --release`
   - Generate C header: `cbindgen --config cbindgen.toml --crate rb-okvs-ffi --output rb_okvs_ffi.h`
   - Verify FFI functions work correctly

### Phase 3: Go cgo Bindings

1. Create `internal/crypto/okvs_impl.go`
2. Implement `RBOKVSEncoder` and `RBOKVSDecoder`
3. Add cgo directives for linking Rust library
4. Implement key hashing in Go (BLAKE2)
5. Implement f64 conversion in Go

### Phase 4: Server Integration

1. Replace `MockOKVSEncoder` with `RBOKVSEncoder` in `server.go`
2. Update PIR integration to work with OKVS-encoded blob
3. Verify end-to-end flow

### Phase 5: Testing and Benchmarking

1. Unit tests for encoding/decoding
2. Integration tests with server
3. Performance benchmarks

## References

- GUIDE.md mentions RB-OKVS as recommended algorithm
- USENIX Security 2023 paper on OKVS
- FrodoPIR integration (successful pattern to follow)
- RB-OKVS Library: `third_party/rb-okvs/` (git submodule from `felicityin/rb-okvs`)
- Test Suite: `third_party/rb-okvs/tests/float64_use_case.rs` (7 tests, all passing)
- FFI Template: `third_party/frodo-pir-ffi/` (reference implementation)

## Similarity to FrodoPIR Integration

This follows the exact same pattern as FrodoPIR:

| Phase | FrodoPIR Status | RB-OKVS Status |
|-------|------------------|----------------|
| 1. Find/create FFI wrapper | ✅ Complete | ✅ Library found |
| 2. Build static library | ✅ Complete | ⚠️ Next step |
| 3. Generate C headers | ✅ Complete | ⚠️ Next step |
| 4. Create Go cgo bindings | ✅ Complete | ⚠️ Pending |
| 5. Integrate with server | ✅ Complete | ⚠️ Pending |
| 6. Test and benchmark | ✅ Complete | ✅ Tests created (Rust) |

**Status**: Phase 1 complete. Ready to proceed with Phase 2 (FFI wrapper creation).

**Reference Implementation**: `third_party/frodo-pir-ffi/` serves as the template for `third_party/rb-okvs-ffi/`.


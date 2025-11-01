# FrodoPIR FFI Wrapper

This directory contains a Rust FFI wrapper for FrodoPIR that provides a C-compatible API for use with Go's `cgo`.

## Overview

The FFI wrapper bridges the Rust implementation of FrodoPIR with Go code, allowing the `silhouette-db` project to use FrodoPIR for Private Information Retrieval (PIR) queries.

## Structure

```
frodo-pir-ffi/
├── Cargo.toml           # Rust project configuration
├── build.rs             # Build script to generate C headers
├── cbindgen.toml        # Configuration for header generation
├── src/
│   └── lib.rs           # FFI wrapper implementation
└── frodopir_ffi.h       # Generated C header (auto-generated)
```

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

### Build Static Library

```bash
cd third_party/frodo-pir-ffi
cargo build --release

# This will create:
# - target/release/libfrodopirffi.a (static library)
# - target/release/libfrodopirffi.dylib (on macOS) or .so (on Linux)
# - frodopir_ffi.h (C header file)
```

## FFI API

### Server Functions

#### `frodopir_shard_create`
Creates a FrodoPIR server shard from a database of base64-encoded strings.

**Signature:**
```c
int frodopir_shard_create(
    const char **db_elements_ptr,
    size_t num_elements,
    size_t lwe_dim,
    size_t m,
    size_t elem_size,
    size_t plaintext_bits,
    FrodoPIRShard *shard_out,
    uint8_t **base_params_out,
    size_t *base_params_len
);
```

**Parameters:**
- `db_elements_ptr`: Array of C strings (base64-encoded database elements)
- `num_elements`: Number of elements
- `lwe_dim`: LWE dimension (512, 1024, or 1572)
- `m`: Number of database elements (must equal num_elements)
- `elem_size`: Element size in bits
- `plaintext_bits`: Plaintext bits per matrix element (10 or 9)

**Outputs:**
- `shard_out`: Created shard handle
- `base_params_out`: Serialized BaseParams (allocated, caller must free)
- `base_params_len`: Length of BaseParams

**Returns:** `FrodoPIRResult::Success` (0) on success, error code otherwise

#### `frodopir_shard_respond`
Processes a PIR query and returns the response.

**Signature:**
```c
int frodopir_shard_respond(
    FrodoPIRShard shard,
    const uint8_t *query_bytes,
    size_t query_len,
    uint8_t **response_out,
    size_t *response_len
);
```

**Returns:** Serialized Response bytes (allocated, caller must free)

#### `frodopir_shard_free`
Frees memory allocated for a shard handle.

### Client Functions

#### `frodopir_client_create`
Creates a FrodoPIR client from serialized BaseParams.

**Signature:**
```c
int frodopir_client_create(
    const uint8_t *base_params_bytes,
    size_t base_params_len,
    FrodoPIRQueryParams *client_out
);
```

#### `frodopir_client_generate_query`
Generates a PIR query for a specific row index.

**Signature:**
```c
int frodopir_client_generate_query(
    FrodoPIRQueryParams client,
    size_t row_index,
    uint8_t **query_out,
    size_t *query_len,
    uint8_t **query_params_out,
    size_t *query_params_len
);
```

**Note:** Returns both the query and QueryParams. The QueryParams are needed for decoding the response.

#### `frodopir_client_decode_response`
Decodes a PIR server response to extract the value.

**Signature:**
```c
int frodopir_client_decode_response(
    FrodoPIRQueryParams client,
    const uint8_t *response_bytes,
    size_t response_len,
    const uint8_t *query_params_bytes,
    size_t query_params_len,
    uint8_t **output_out,
    size_t *output_len
);
```

#### `frodopir_client_free`
Frees memory allocated for a client handle.

### Memory Management

#### `frodopir_free_buffer`
Frees memory allocated for byte buffers returned by FFI functions.

**Signature:**
```c
void frodopir_free_buffer(uint8_t *ptr, size_t len);
```

## Error Codes

- `FrodoPIRResult::Success` (0) - Operation succeeded
- `FrodoPIRResult::InvalidInput` (1) - Invalid input parameters
- `FrodoPIRResult::SerializationError` (2) - Serialization failed
- `FrodoPIRResult::DeserializationError` (3) - Deserialization failed
- `FrodoPIRResult::QueryParamsReused` (4) - QueryParams already used
- `FrodoPIRResult::NotFound` (5) - Resource not found
- `FrodoPIRResult::UnknownError` (99) - Unknown error

## Integration with Go

The static library (`libfrodopirffi.a`) and header file (`frodopir_ffi.h`) are used in Go's `cgo` bindings. See `internal/crypto/pir.go` for the Go integration.

## Notes

- All byte buffers returned by FFI functions must be freed using `frodopir_free_buffer`
- Shard and client handles must be freed using their respective `_free` functions
- QueryParams can only be used once - each query requires a new QueryParams instance
- The client must store QueryParams alongside queries for proper response decoding


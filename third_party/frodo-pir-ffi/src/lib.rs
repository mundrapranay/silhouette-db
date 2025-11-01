//! FFI wrapper for FrodoPIR to enable C-compatible API for Go cgo bindings.
//! 
//! This module provides a C-compatible interface to FrodoPIR, allowing it to be
//! called from Go via cgo. The wrapper handles:
//! - Memory management (allocating/freeing buffers)
//! - Serialization/deserialization (using bincode)
//! - Error handling (converting Rust errors to error codes)

use std::ffi::CStr;
use std::os::raw::{c_char, c_int, c_void};
use std::slice;

use bincode;
use frodo_pir::api::*;

/// Error codes returned by FFI functions
#[repr(C)]
pub enum FrodoPIRResult {
    Success = 0,
    InvalidInput = 1,
    SerializationError = 2,
    DeserializationError = 3,
    QueryParamsReused = 4,
    OverflownAdd = 5,
    NotFound = 6,
    UnknownError = 99,
}

/// Opaque handle to a FrodoPIR Shard (server-side database)
#[repr(C)]
pub struct FrodoPIRShard(*mut c_void);

/// Opaque handle to FrodoPIR QueryParams (client-side)
#[repr(C)]
pub struct FrodoPIRQueryParams(*mut c_void);

/// Server handle containing Shard and BaseParams
struct FrodoPIRServer {
    shard: Box<Shard>,
    #[allow(dead_code)]
    base_params_serialized: Vec<u8>, // Serialized BaseParams for client (kept for future use)
}

/// Client handle containing BaseParams and CommonParams
struct FrodoPIRClient {
    base_params: Box<BaseParams>,
    common_params: Box<CommonParams>,
}


/// Create a FrodoPIR server from a database of base64-encoded strings.
/// 
/// # Arguments
/// - `db_elements_ptr`: Pointer to array of C strings (base64-encoded)
/// - `num_elements`: Number of elements in the database
/// - `lwe_dim`: LWE dimension (typically 512, 1024, or 1572)
/// - `m`: Number of database elements (should equal num_elements)
/// - `elem_size`: Element size in bits
/// - `plaintext_bits`: Number of plaintext bits per matrix element (10 or 9)
/// - `shard_out`: Pointer to store the created shard handle
/// - `base_params_out`: Pointer to buffer to store serialized BaseParams (output)
/// - `base_params_len`: Pointer to store length of serialized BaseParams (output)
/// 
/// # Returns
/// FrodoPIRResult::Success on success, error code otherwise
#[no_mangle]
pub extern "C" fn frodopir_shard_create(
    db_elements_ptr: *const *const c_char,
    num_elements: usize,
    lwe_dim: usize,
    m: usize,
    elem_size: usize,
    plaintext_bits: usize,
    shard_out: *mut FrodoPIRShard,
    base_params_out: *mut *mut u8,
    base_params_len: *mut usize,
) -> c_int {
    if db_elements_ptr.is_null() || shard_out.is_null() || base_params_out.is_null() || base_params_len.is_null() {
        return FrodoPIRResult::InvalidInput as c_int;
    }

    unsafe {
        // Convert C string array to Vec<String>
        let mut db_elements = Vec::with_capacity(num_elements);
        for i in 0..num_elements {
            let c_str = CStr::from_ptr(*db_elements_ptr.add(i));
            match c_str.to_str() {
                Ok(s) => db_elements.push(s.to_string()),
                Err(_) => return FrodoPIRResult::InvalidInput as c_int,
            }
        }

        // Create shard
        let shard = match Shard::from_base64_strings(&db_elements, lwe_dim, m, elem_size, plaintext_bits) {
            Ok(s) => s,
            Err(_) => return FrodoPIRResult::UnknownError as c_int,
        };

        // Get base params and serialize
        let base_params = shard.get_base_params();
        let serialized = match bincode::serialize(base_params) {
            Ok(v) => v,
            Err(_) => return FrodoPIRResult::SerializationError as c_int,
        };

        // Allocate memory for serialized params
        let len = serialized.len();
        let mut buf = Vec::with_capacity(len);
        buf.extend_from_slice(&serialized);
        let boxed = buf.into_boxed_slice();
        let raw_ptr = Box::into_raw(boxed) as *mut u8;

        *base_params_out = raw_ptr;
        *base_params_len = len;

        // Create server handle
        let server = Box::new(FrodoPIRServer {
            shard: Box::new(shard),
            base_params_serialized: serialized,
        });

        *shard_out = FrodoPIRShard(Box::into_raw(server) as *mut c_void);

        FrodoPIRResult::Success as c_int
    }
}

/// Process a PIR query on the server side.
/// 
/// # Arguments
/// - `shard`: Shard handle
/// - `query_bytes`: Serialized Query bytes
/// - `query_len`: Length of query bytes
/// - `response_out`: Pointer to buffer to store response (output)
/// - `response_len`: Pointer to store response length (output)
/// 
/// # Returns
/// FrodoPIRResult::Success on success
#[no_mangle]
pub extern "C" fn frodopir_shard_respond(
    shard: FrodoPIRShard,
    query_bytes: *const u8,
    query_len: usize,
    response_out: *mut *mut u8,
    response_len: *mut usize,
) -> c_int {
    if shard.0.is_null() || query_bytes.is_null() || response_out.is_null() || response_len.is_null() {
        return FrodoPIRResult::InvalidInput as c_int;
    }

    unsafe {
        let server = &*(shard.0 as *const FrodoPIRServer);
        
        // Deserialize query
        let query_slice = slice::from_raw_parts(query_bytes, query_len);
        let query: Query = match bincode::deserialize(query_slice) {
            Ok(q) => q,
            Err(_) => return FrodoPIRResult::DeserializationError as c_int,
        };

        // Process query
        let response_bytes = match server.shard.respond(&query) {
            Ok(r) => r,
            Err(_) => return FrodoPIRResult::UnknownError as c_int,
        };

        // Allocate memory for response
        let len = response_bytes.len();
        let mut buf = Vec::with_capacity(len);
        buf.extend_from_slice(&response_bytes);
        let boxed = buf.into_boxed_slice();
        let raw_ptr = Box::into_raw(boxed) as *mut u8;

        *response_out = raw_ptr;
        *response_len = len;

        FrodoPIRResult::Success as c_int
    }
}

/// Create a FrodoPIR client from serialized BaseParams.
/// 
/// # Arguments
/// - `base_params_bytes`: Serialized BaseParams
/// - `base_params_len`: Length of BaseParams bytes
/// - `client_out`: Pointer to store client handle
/// 
/// # Returns
/// FrodoPIRResult::Success on success
#[no_mangle]
pub extern "C" fn frodopir_client_create(
    base_params_bytes: *const u8,
    base_params_len: usize,
    client_out: *mut FrodoPIRQueryParams,
) -> c_int {
    if base_params_bytes.is_null() || client_out.is_null() {
        return FrodoPIRResult::InvalidInput as c_int;
    }

    unsafe {
        // Deserialize BaseParams
        let params_slice = slice::from_raw_parts(base_params_bytes, base_params_len);
        let base_params: BaseParams = match bincode::deserialize(params_slice) {
            Ok(p) => p,
            Err(_) => return FrodoPIRResult::DeserializationError as c_int,
        };

        // Create CommonParams from BaseParams
        let common_params = CommonParams::from(&base_params);

        let client = Box::new(FrodoPIRClient {
            base_params: Box::new(base_params),
            common_params: Box::new(common_params),
        });

        *client_out = FrodoPIRQueryParams(Box::into_raw(client) as *mut c_void);

        FrodoPIRResult::Success as c_int
    }
}

/// Generate a PIR query for a specific row index.
/// 
/// Returns both the serialized query and the serialized QueryParams needed for decoding.
/// 
/// # Arguments
/// - `client`: Client handle
/// - `row_index`: Index of the database row to query
/// - `query_out`: Pointer to buffer to store query (output)
/// - `query_len`: Pointer to store query length (output)
/// - `query_params_out`: Pointer to buffer to store QueryParams (output)
/// - `query_params_len`: Pointer to store QueryParams length (output)
/// 
/// # Returns
/// FrodoPIRResult::Success on success
#[no_mangle]
pub extern "C" fn frodopir_client_generate_query(
    client: FrodoPIRQueryParams,
    row_index: usize,
    query_out: *mut *mut u8,
    query_len: *mut usize,
    query_params_out: *mut *mut u8,
    query_params_len: *mut usize,
) -> c_int {
    if client.0.is_null() || query_out.is_null() || query_len.is_null() || 
       query_params_out.is_null() || query_params_len.is_null() {
        return FrodoPIRResult::InvalidInput as c_int;
    }

    unsafe {
        let client_ref = &*(client.0 as *const FrodoPIRClient);
        
        // Create new QueryParams for this query
        let mut query_params = match QueryParams::new(&client_ref.common_params, &client_ref.base_params) {
            Ok(qp) => qp,
            Err(_) => return FrodoPIRResult::UnknownError as c_int,
        };

        // Generate query (this marks query_params as used)
        // Note: row_index bounds are checked inside generate_query
        let query = match query_params.generate_query(row_index) {
            Ok(q) => q,
            Err(e) => {
                // Check the actual error type by matching the error message
                let err_str = format!("{}", e);
                if err_str.contains("reuse") || err_str.contains("used already") {
                    return FrodoPIRResult::QueryParamsReused as c_int;
                } else if err_str.contains("overflow") || err_str.contains("overflow addition") {
                    return FrodoPIRResult::OverflownAdd as c_int;
                }
                // For any other error (including bounds issues), return UnknownError
                return FrodoPIRResult::UnknownError as c_int;
            }
        };

        // Serialize query
        let query_bytes = match bincode::serialize(&query) {
            Ok(b) => b,
            Err(_) => return FrodoPIRResult::SerializationError as c_int,
        };

        // Serialize QueryParams (needed for decoding)
        let query_params_bytes = match bincode::serialize(&query_params) {
            Ok(b) => b,
            Err(_) => return FrodoPIRResult::SerializationError as c_int,
        };

        // Allocate memory for query
        let q_len = query_bytes.len();
        let mut q_buf = Vec::with_capacity(q_len);
        q_buf.extend_from_slice(&query_bytes);
        let q_boxed = q_buf.into_boxed_slice();
        let q_raw_ptr = Box::into_raw(q_boxed) as *mut u8;

        // Allocate memory for query_params
        let qp_len = query_params_bytes.len();
        let mut qp_buf = Vec::with_capacity(qp_len);
        qp_buf.extend_from_slice(&query_params_bytes);
        let qp_boxed = qp_buf.into_boxed_slice();
        let qp_raw_ptr = Box::into_raw(qp_boxed) as *mut u8;

        *query_out = q_raw_ptr;
        *query_len = q_len;
        *query_params_out = qp_raw_ptr;
        *query_params_len = qp_len;

        FrodoPIRResult::Success as c_int
    }
}

/// Decode a PIR server response to extract the value.
/// 
/// Note: This requires the QueryParams used to generate the query, but QueryParams
/// can only be used once. For now, we create a new QueryParams which works but is
/// not optimal. In a real implementation, the client should store the QueryParams
/// alongside the query.
/// 
/// # Arguments
/// - `client`: Client handle
/// - `response_bytes`: Serialized Response bytes
/// - `response_len`: Length of response bytes
/// - `query_params_bytes`: Serialized QueryParams used to generate the query
/// - `query_params_len`: Length of QueryParams bytes
/// - `output_out`: Pointer to buffer to store output bytes (output)
/// - `output_len`: Pointer to store output length (output)
/// 
/// # Returns
/// FrodoPIRResult::Success on success
#[no_mangle]
pub extern "C" fn frodopir_client_decode_response(
    client: FrodoPIRQueryParams,
    response_bytes: *const u8,
    response_len: usize,
    query_params_bytes: *const u8,
    query_params_len: usize,
    output_out: *mut *mut u8,
    output_len: *mut usize,
) -> c_int {
    if client.0.is_null() || response_bytes.is_null() || output_out.is_null() || output_len.is_null() {
        return FrodoPIRResult::InvalidInput as c_int;
    }

    unsafe {
        // Deserialize response
        let resp_slice = slice::from_raw_parts(response_bytes, response_len);
        let response: Response = match bincode::deserialize(resp_slice) {
            Ok(r) => r,
            Err(_) => return FrodoPIRResult::DeserializationError as c_int,
        };

        // Deserialize QueryParams (passed from client)
        let qp_slice = slice::from_raw_parts(query_params_bytes, query_params_len);
        let query_params: QueryParams = match bincode::deserialize(qp_slice) {
            Ok(qp) => qp,
            Err(_) => return FrodoPIRResult::DeserializationError as c_int,
        };

        // Parse output as bytes
        let output_bytes = response.parse_output_as_bytes(&query_params);

        // Allocate memory for output
        let len = output_bytes.len();
        let mut buf = Vec::with_capacity(len);
        buf.extend_from_slice(&output_bytes);
        let boxed = buf.into_boxed_slice();
        let raw_ptr = Box::into_raw(boxed) as *mut u8;

        *output_out = raw_ptr;
        *output_len = len;

        FrodoPIRResult::Success as c_int
    }
}

/// Free memory allocated for a shard handle.
#[no_mangle]
pub extern "C" fn frodopir_shard_free(shard: FrodoPIRShard) {
    if !shard.0.is_null() {
        unsafe {
            let _ = Box::from_raw(shard.0 as *mut FrodoPIRServer);
        }
    }
}

/// Free memory allocated for a client handle.
#[no_mangle]
pub extern "C" fn frodopir_client_free(client: FrodoPIRQueryParams) {
    if !client.0.is_null() {
        unsafe {
            let _ = Box::from_raw(client.0 as *mut FrodoPIRClient);
        }
    }
}

/// Free memory allocated for a byte buffer (returned by FFI functions).
#[no_mangle]
pub extern "C" fn frodopir_free_buffer(ptr: *mut u8, len: usize) {
    if !ptr.is_null() && len > 0 {
        unsafe {
            let _ = Box::from_raw(slice::from_raw_parts_mut(ptr, len) as *mut [u8]);
        }
    }
}

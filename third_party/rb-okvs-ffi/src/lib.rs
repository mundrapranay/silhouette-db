//! FFI wrapper for RB-OKVS to enable C-compatible API for Go cgo bindings.
//! 
//! This module provides a C-compatible interface to RB-OKVS, allowing it to be
//! called from Go via cgo. The wrapper handles:
//! - Memory management (allocating/freeing buffers)
//! - Serialization/deserialization (using bincode)
//! - Error handling (converting Rust errors to error codes)
//! - Key hashing (BLAKE2b512 → 8-byte OkvsKey)
//! - Value conversion (f64 ↔ OkvsValue<8>)

use std::ffi::CStr;
use std::os::raw::{c_char, c_int};
use std::slice;

use blake2::{Blake2b512, Digest};
use rb_okvs::okvs::RbOkvs;
use rb_okvs::types::{Okvs, OkvsKey, OkvsValue, Pair};

/// Error codes returned by FFI functions
#[repr(C)]
pub enum RBOKVSResult {
    Success = 0,
    InvalidInput = 1,
    SerializationError = 2,
    DeserializationError = 3,
    EncodingError = 4,
    DecodingError = 5,
    UnknownError = 99,
}

/// Fixed value size for f64 (8 bytes)
const VALUE_SIZE: usize = 8;

/// Hash a string key to an 8-byte OkvsKey using BLAKE2b512
fn hash_key_to_okvs_key(key: &str) -> OkvsKey {
    let mut hasher = Blake2b512::new();
    hasher.update(key.as_bytes());
    let hash = hasher.finalize();
    
    let mut key_bytes = [0u8; 8];
    key_bytes.copy_from_slice(&hash[..8]);
    OkvsKey(key_bytes)
}

/// Convert a f64 value to OkvsValue<8>
fn f64_to_okvs_value(value: f64) -> OkvsValue<VALUE_SIZE> {
    let bytes = value.to_le_bytes();
    OkvsValue(bytes)
}

/// Convert OkvsValue<8> back to f64
fn okvs_value_to_f64(value: &OkvsValue<VALUE_SIZE>) -> f64 {
    f64::from_le_bytes(value.0)
}

/// Free a buffer allocated by the FFI
#[no_mangle]
pub extern "C" fn rb_okvs_free_buffer(ptr: *mut u8, len: usize) {
    if !ptr.is_null() && len > 0 {
        unsafe {
            let _ = Vec::from_raw_parts(ptr, len, len);
        }
    }
}

/// Encode key-value pairs (string keys → float64 values) into an OKVS blob.
/// 
/// # Arguments
/// - `keys_ptr`: Pointer to array of C strings (keys)
/// - `values_ptr`: Pointer to array of f64 values (8 bytes each)
/// - `num_pairs`: Number of key-value pairs
/// - `encoding_out`: Pointer to buffer to store serialized OKVS encoding (output)
/// - `encoding_len`: Pointer to store length of encoding (output)
/// 
/// # Returns
/// RBOKVSResult::Success on success, error code otherwise
#[no_mangle]
pub extern "C" fn rb_okvs_encode(
    keys_ptr: *const *const c_char,
    values_ptr: *const f64,
    num_pairs: usize,
    encoding_out: *mut *mut u8,
    encoding_len: *mut usize,
) -> c_int {
    if keys_ptr.is_null() || values_ptr.is_null() || encoding_out.is_null() || encoding_len.is_null() {
        return RBOKVSResult::InvalidInput as c_int;
    }

    if num_pairs == 0 {
        return RBOKVSResult::InvalidInput as c_int;
    }

    unsafe {
        // Convert C string array and f64 array to Vec<(String, f64)>
        let mut pairs_vec: Vec<(String, f64)> = Vec::with_capacity(num_pairs);
        
        for i in 0..num_pairs {
            // Get key string
            let key_cstr = CStr::from_ptr(*keys_ptr.add(i));
            let key_str = match key_cstr.to_str() {
                Ok(s) => s.to_string(),
                Err(_) => return RBOKVSResult::InvalidInput as c_int,
            };
            
            // Get f64 value
            let value_f64 = *values_ptr.add(i);
            
            pairs_vec.push((key_str, value_f64));
        }

        // Convert to OKVS pairs: (OkvsKey, OkvsValue<8>)
        let okvs_pairs: Vec<Pair<OkvsKey, OkvsValue<VALUE_SIZE>>> = pairs_vec
            .iter()
            .map(|(key_str, value_f64)| {
                let key = hash_key_to_okvs_key(key_str);
                let value = f64_to_okvs_value(*value_f64);
                (key, value)
            })
            .collect();

        // Create RB-OKVS instance and store parameters
        // We need to calculate columns and band_width to store for decoding
        let kv_count = okvs_pairs.len();
        let columns = ((1.0 + 0.1) * kv_count as f64) as usize; // epsilon = 0.1
        let band_width = if 128 < columns { 128 } else { (columns * 80 / 100).max(8) };
        let band_width = band_width.min(columns.saturating_sub(1)).max(8);
        
        let rb_okvs = RbOkvs::new(okvs_pairs.len());
        
        // Encode
        let encoding = match rb_okvs.encode(okvs_pairs) {
            Ok(e) => e,
            Err(_) => return RBOKVSResult::EncodingError as c_int,
        };

        // Manually serialize encoding: convert OkvsValue<8> to bytes
        // Format: [encoding_len (8 bytes)] [num_values (8 bytes)] [value0 (8 bytes)] [value1 (8 bytes)] ...
        let mut serialized = Vec::new();
        serialized.extend_from_slice(&(encoding.len() as u64).to_le_bytes());
        for value in &encoding {
            serialized.extend_from_slice(&value.0);
        }

        // Store RbOkvs parameters for decoding
        // Format: [params_len (8 bytes)] [kv_count (8)] [columns (8)] [band_width (8)]
        let mut params_serialized = Vec::new();
        params_serialized.extend_from_slice(&(kv_count as u64).to_le_bytes());
        params_serialized.extend_from_slice(&(columns as u64).to_le_bytes());
        params_serialized.extend_from_slice(&(band_width as u64).to_le_bytes());

        // Combine: [params_len (8 bytes)] [columns (8)] [band_width (8)] [encoding_len (8)] [encoding]
        let mut final_output = Vec::new();
        final_output.extend_from_slice(&(params_serialized.len() as u64).to_le_bytes());
        final_output.extend_from_slice(&params_serialized);
        final_output.extend_from_slice(&serialized);
        
        let final_len = final_output.len();
        let final_box = final_output.into_boxed_slice();
        let final_raw = Box::into_raw(final_box) as *mut u8;

        *encoding_out = final_raw;
        *encoding_len = final_len;

        RBOKVSResult::Success as c_int
    }
}

/// Decode a float64 value from an OKVS encoding blob using a string key.
/// 
/// # Arguments
/// - `encoding_ptr`: Pointer to serialized OKVS encoding blob
/// - `encoding_len`: Length of encoding blob
/// - `key_ptr`: Pointer to C string (key to decode)
/// - `value_out`: Pointer to buffer to store f64 value (output, 8 bytes)
/// 
/// # Returns
/// RBOKVSResult::Success on success, error code otherwise
#[no_mangle]
pub extern "C" fn rb_okvs_decode(
    encoding_ptr: *const u8,
    encoding_len: usize,
    key_ptr: *const c_char,
    value_out: *mut f64,
) -> c_int {
    if encoding_ptr.is_null() || encoding_len == 0 || key_ptr.is_null() || value_out.is_null() {
        return RBOKVSResult::InvalidInput as c_int;
    }

    unsafe {
        // Get key string
        let key_cstr = CStr::from_ptr(key_ptr);
        let key_str = match key_cstr.to_str() {
            Ok(s) => s,
            Err(_) => return RBOKVSResult::InvalidInput as c_int,
        };

        // Deserialize blob: [params_len (8)] [kv_count (8)] [columns (8)] [band_width (8)] [encoding_len (8)] [encoding]
        let encoding_slice = slice::from_raw_parts(encoding_ptr, encoding_len);
        
        if encoding_len < 8 {
            return RBOKVSResult::DeserializationError as c_int;
        }

        // Read params length
        let params_len = u64::from_le_bytes([
            encoding_slice[0], encoding_slice[1], encoding_slice[2], encoding_slice[3],
            encoding_slice[4], encoding_slice[5], encoding_slice[6], encoding_slice[7],
        ]) as usize;

        if encoding_len < 8 + params_len || params_len != 24 {
            return RBOKVSResult::DeserializationError as c_int;
        }

        // Read params: kv_count, columns, and band_width (8 bytes each)
        let params_slice = &encoding_slice[8..8 + params_len];
        let _kv_count = u64::from_le_bytes([
            params_slice[0], params_slice[1], params_slice[2], params_slice[3],
            params_slice[4], params_slice[5], params_slice[6], params_slice[7],
        ]) as usize;
        
        let _columns = u64::from_le_bytes([
            params_slice[8], params_slice[9], params_slice[10], params_slice[11],
            params_slice[12], params_slice[13], params_slice[14], params_slice[15],
        ]) as usize;
        
        let _band_width = u64::from_le_bytes([
            params_slice[16], params_slice[17], params_slice[18], params_slice[19],
            params_slice[20], params_slice[21], params_slice[22], params_slice[23],
        ]) as usize;

        // Read encoding length
        if encoding_len < 8 + params_len + 8 {
            return RBOKVSResult::DeserializationError as c_int;
        }
        
        let encoding_len_offset = 8 + params_len;
        let encoding_len_val = u64::from_le_bytes([
            encoding_slice[encoding_len_offset], encoding_slice[encoding_len_offset + 1],
            encoding_slice[encoding_len_offset + 2], encoding_slice[encoding_len_offset + 3],
            encoding_slice[encoding_len_offset + 4], encoding_slice[encoding_len_offset + 5],
            encoding_slice[encoding_len_offset + 6], encoding_slice[encoding_len_offset + 7],
        ]) as usize;

        // Deserialize encoding: manually convert bytes to OkvsValue<8>
        let encoding_start = encoding_len_offset + 8;
        if encoding_len < encoding_start + encoding_len_val * VALUE_SIZE {
            return RBOKVSResult::DeserializationError as c_int;
        }

        let mut encoding = Vec::with_capacity(encoding_len_val);
        for i in 0..encoding_len_val {
            let offset = encoding_start + i * VALUE_SIZE;
            let mut value_bytes = [0u8; VALUE_SIZE];
            value_bytes.copy_from_slice(&encoding_slice[offset..offset + VALUE_SIZE]);
            encoding.push(OkvsValue(value_bytes));
        }

        // Recreate RbOkvs from kv_count (RbOkvs::new will calculate columns and band_width)
        // This should match the original since we're using the same kv_count
        let rb_okvs = RbOkvs::new(_kv_count);

        // Hash key to OkvsKey
        let key = hash_key_to_okvs_key(key_str);

        // Decode
        let decoded_value = rb_okvs.decode(&encoding, &key);

        // Convert to f64
        let value_f64 = okvs_value_to_f64(&decoded_value);

        // Write output
        *value_out = value_f64;

        RBOKVSResult::Success as c_int
    }
}


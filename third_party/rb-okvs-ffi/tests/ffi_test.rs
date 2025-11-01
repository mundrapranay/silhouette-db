//! FFI tests for RB-OKVS to verify encode/decode functionality

use std::ffi::CString;

// Use the library functions directly for testing
// In actual Go cgo usage, these will be called via FFI
use rbokvsffi::*;

#[test]
fn test_ffi_encode_decode() {
    // Create test data: 100 pairs (minimum for reliable operation)
    let mut keys = Vec::new();
    let mut values = Vec::new();
    
    for i in 0..100 {
        keys.push(CString::new(format!("key{}", i)).unwrap());
        values.push(i as f64 * 0.123);
    }

    // Create array of C string pointers
    let key_ptrs: Vec<*const i8> = keys.iter().map(|k| k.as_ptr()).collect();
    
    // Encode
    let mut encoding_out: *mut u8 = std::ptr::null_mut();
    let mut encoding_len: usize = 0;
    
    let result = unsafe {
        rb_okvs_encode(
            key_ptrs.as_ptr(),
            values.as_ptr(),
            100,
            &mut encoding_out,
            &mut encoding_len,
        )
    };
    
    assert_eq!(result, RBOKVSResult::Success as i32, "Encoding should succeed");
    assert!(!encoding_out.is_null(), "Encoding output should not be null");
    assert!(encoding_len > 0, "Encoding length should be > 0");

    // Decode a few keys
    for i in 0..10 {
        let key_str = format!("key{}", i);
        let key_cstr = CString::new(key_str).unwrap();
        let mut value_out: f64 = 0.0;
        
        let decode_result = unsafe {
            rb_okvs_decode(
                encoding_out,
                encoding_len,
                key_cstr.as_ptr(),
                &mut value_out,
            )
        };
        
        assert_eq!(decode_result, RBOKVSResult::Success as i32, 
                   "Decoding should succeed for key{}", i);
        
        let expected = i as f64 * 0.123;
        let epsilon = f64::EPSILON * 100.0;
        assert!(
            (value_out - expected).abs() < epsilon || value_out == expected,
            "Decoded value for key{}: expected {}, got {}",
            i, expected, value_out
        );
    }

    // Free encoding buffer
    unsafe {
        rb_okvs_free_buffer(encoding_out, encoding_len);
    }
}


/* WARNING: This file is auto-generated. Do not edit manually. */


#ifndef RB_OKVS_FFI_H
#define RB_OKVS_FFI_H

#pragma once

/* WARNING: This file is auto-generated. Do not edit manually. */

#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

/**
 * Error codes returned by FFI functions
 */
typedef enum RBOKVSResult {
  Success = 0,
  InvalidInput = 1,
  SerializationError = 2,
  DeserializationError = 3,
  EncodingError = 4,
  DecodingError = 5,
  UnknownError = 99,
} RBOKVSResult;

/**
 * Free a buffer allocated by the FFI
 */
void rb_okvs_free_buffer(uint8_t *ptr, uintptr_t len);

/**
 * Encode key-value pairs (string keys → float64 values) into an OKVS blob.
 *
 * # Arguments
 * - `keys_ptr`: Pointer to array of C strings (keys)
 * - `values_ptr`: Pointer to array of f64 values (8 bytes each)
 * - `num_pairs`: Number of key-value pairs
 * - `encoding_out`: Pointer to buffer to store serialized OKVS encoding (output)
 * - `encoding_len`: Pointer to store length of encoding (output)
 *
 * # Returns
 * RBOKVSResult::Success on success, error code otherwise
 */
int rb_okvs_encode(const char *const *keys_ptr,
                   const double *values_ptr,
                   uintptr_t num_pairs,
                   uint8_t **encoding_out,
                   uintptr_t *encoding_len);

/**
 * Decode a float64 value from an OKVS encoding blob using a string key.
 *
 * # Arguments
 * - `encoding_ptr`: Pointer to serialized OKVS encoding blob
 * - `encoding_len`: Length of encoding blob
 * - `key_ptr`: Pointer to C string (key to decode)
 * - `value_out`: Pointer to buffer to store f64 value (output, 8 bytes)
 *
 * # Returns
 * RBOKVSResult::Success on success, error code otherwise
 */
int rb_okvs_decode(const uint8_t *encoding_ptr,
                   uintptr_t encoding_len,
                   const char *key_ptr,
                   double *value_out);

#endif /* RB_OKVS_FFI_H */

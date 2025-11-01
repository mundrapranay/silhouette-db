/* WARNING: This file is auto-generated. Do not edit manually. */


#ifndef FRODOPIR_FFI_H
#define FRODOPIR_FFI_H

#pragma once

/* WARNING: This file is auto-generated. Do not edit manually. */

#include <stdarg.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

/**
 * Client handle containing BaseParams and CommonParams
 */
typedef struct FrodoPIRClient FrodoPIRClient;

/**
 * Server handle containing Shard and BaseParams
 */
typedef struct FrodoPIRServer FrodoPIRServer;

/**
 * Opaque handle to a FrodoPIR Shard (server-side database)
 */
typedef struct FrodoPIRShard {
  void *_0;
} FrodoPIRShard;

/**
 * Opaque handle to FrodoPIR QueryParams (client-side)
 */
typedef struct FrodoPIRQueryParams {
  void *_0;
} FrodoPIRQueryParams;

/**
 * Create a FrodoPIR server from a database of base64-encoded strings.
 *
 * # Arguments
 * - `db_elements_ptr`: Pointer to array of C strings (base64-encoded)
 * - `num_elements`: Number of elements in the database
 * - `lwe_dim`: LWE dimension (typically 512, 1024, or 1572)
 * - `m`: Number of database elements (should equal num_elements)
 * - `elem_size`: Element size in bits
 * - `plaintext_bits`: Number of plaintext bits per matrix element (10 or 9)
 * - `shard_out`: Pointer to store the created shard handle
 * - `base_params_out`: Pointer to buffer to store serialized BaseParams (output)
 * - `base_params_len`: Pointer to store length of serialized BaseParams (output)
 *
 * # Returns
 * FrodoPIRResult::Success on success, error code otherwise
 */
int frodopir_shard_create(const char *const *db_elements_ptr,
                          uintptr_t num_elements,
                          uintptr_t lwe_dim,
                          uintptr_t m,
                          uintptr_t elem_size,
                          uintptr_t plaintext_bits,
                          struct FrodoPIRShard *shard_out,
                          uint8_t **base_params_out,
                          uintptr_t *base_params_len);

/**
 * Process a PIR query on the server side.
 *
 * # Arguments
 * - `shard`: Shard handle
 * - `query_bytes`: Serialized Query bytes
 * - `query_len`: Length of query bytes
 * - `response_out`: Pointer to buffer to store response (output)
 * - `response_len`: Pointer to store response length (output)
 *
 * # Returns
 * FrodoPIRResult::Success on success
 */
int frodopir_shard_respond(struct FrodoPIRShard shard,
                           const uint8_t *query_bytes,
                           uintptr_t query_len,
                           uint8_t **response_out,
                           uintptr_t *response_len);

/**
 * Create a FrodoPIR client from serialized BaseParams.
 *
 * # Arguments
 * - `base_params_bytes`: Serialized BaseParams
 * - `base_params_len`: Length of BaseParams bytes
 * - `client_out`: Pointer to store client handle
 *
 * # Returns
 * FrodoPIRResult::Success on success
 */
int frodopir_client_create(const uint8_t *base_params_bytes,
                           uintptr_t base_params_len,
                           struct FrodoPIRQueryParams *client_out);

/**
 * Generate a PIR query for a specific row index.
 *
 * Returns both the serialized query and the serialized QueryParams needed for decoding.
 *
 * # Arguments
 * - `client`: Client handle
 * - `row_index`: Index of the database row to query
 * - `query_out`: Pointer to buffer to store query (output)
 * - `query_len`: Pointer to store query length (output)
 * - `query_params_out`: Pointer to buffer to store QueryParams (output)
 * - `query_params_len`: Pointer to store QueryParams length (output)
 *
 * # Returns
 * FrodoPIRResult::Success on success
 */
int frodopir_client_generate_query(struct FrodoPIRQueryParams client,
                                   uintptr_t row_index,
                                   uint8_t **query_out,
                                   uintptr_t *query_len,
                                   uint8_t **query_params_out,
                                   uintptr_t *query_params_len);

/**
 * Decode a PIR server response to extract the value.
 *
 * Note: This requires the QueryParams used to generate the query, but QueryParams
 * can only be used once. For now, we create a new QueryParams which works but is
 * not optimal. In a real implementation, the client should store the QueryParams
 * alongside the query.
 *
 * # Arguments
 * - `client`: Client handle
 * - `response_bytes`: Serialized Response bytes
 * - `response_len`: Length of response bytes
 * - `query_params_bytes`: Serialized QueryParams used to generate the query
 * - `query_params_len`: Length of QueryParams bytes
 * - `output_out`: Pointer to buffer to store output bytes (output)
 * - `output_len`: Pointer to store output length (output)
 *
 * # Returns
 * FrodoPIRResult::Success on success
 */
int frodopir_client_decode_response(struct FrodoPIRQueryParams client,
                                    const uint8_t *response_bytes,
                                    uintptr_t response_len,
                                    const uint8_t *query_params_bytes,
                                    uintptr_t query_params_len,
                                    uint8_t **output_out,
                                    uintptr_t *output_len);

/**
 * Free memory allocated for a shard handle.
 */
void frodopir_shard_free(struct FrodoPIRShard shard);

/**
 * Free memory allocated for a client handle.
 */
void frodopir_client_free(struct FrodoPIRQueryParams client);

/**
 * Free memory allocated for a byte buffer (returned by FFI functions).
 */
void frodopir_free_buffer(uint8_t *ptr, uintptr_t len);

#endif /* FRODOPIR_FFI_H */

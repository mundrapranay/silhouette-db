package crypto

/*
#cgo CFLAGS: -I${SRCDIR}/../../third_party/frodo-pir-ffi
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/frodo-pir-ffi/target/release -lfrodopirffi -lm
#cgo LDFLAGS: -Wl,-rpath,${SRCDIR}/../../third_party/frodo-pir-ffi/target/release

#include "frodopir_ffi.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"encoding/base64"
	"fmt"
	"sort"
	"unsafe"
)

// FrodoPIRResult represents error codes from FrodoPIR FFI
type FrodoPIRResult int

const (
	FrodoPIRResultSuccess FrodoPIRResult = iota
	FrodoPIRResultInvalidInput
	FrodoPIRResultSerializationError
	FrodoPIRResultDeserializationError
	FrodoPIRResultQueryParamsReused
	FrodoPIRResultOverflownAdd
	FrodoPIRResultNotFound
	FrodoPIRResultUnknownError = 99
)

// FrodoPIRServer implements PIRServer using FrodoPIR
type FrodoPIRServer struct {
	shard         C.struct_FrodoPIRShard
	lweDim        uintptr
	m             uintptr
	elemSize      uintptr
	plaintextBits uintptr
}

// NewFrodoPIRServer creates a new FrodoPIR server from a database of key-value pairs.
// The pairs are converted to base64-encoded strings for FrodoPIR.
//
// Parameters:
// - pairs: Map of key-value pairs to store in the database
// - lweDim: LWE dimension (typically 512, 1024, or 1572)
// - elemSize: Element size in bits
// - plaintextBits: Plaintext bits per matrix element (10 or 9)
//
// Returns the server instance and serialized BaseParams for clients.
func NewFrodoPIRServer(pairs map[string][]byte, lweDim, elemSize, plaintextBits int) (*FrodoPIRServer, []byte, error) {
	if len(pairs) == 0 {
		return nil, nil, fmt.Errorf("cannot create server with empty database")
	}

	// Convert pairs to base64-encoded strings (ordered by key)
	// Note: We're storing both key and value in the database element
	// The client will need to know the index to query
	keys := make([]string, 0, len(pairs))
	for k := range pairs {
		keys = append(keys, k)
	}
	// Sort keys for deterministic ordering (must match keyToIndex mapping)
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	// Create array of base64-encoded strings
	// Format: base64(value)
	// All elements must decode to exactly elemSize/8 bytes
	// Note: elemSize is in BITS, so we need elemSize/8 bytes after decoding
	elemSizeBytes := elemSize / 8 // Target bytes after decoding
	dbElements := make([]string, 0, len(pairs))
	for _, key := range keys {
		value := pairs[key]

		// Pad or truncate value to exact byte size before encoding
		if len(value) > elemSizeBytes {
			// Truncate if too large
			value = value[:elemSizeBytes]
		} else if len(value) < elemSizeBytes {
			// Pad with zeros to exact byte size
			padding := make([]byte, elemSizeBytes-len(value))
			value = append(value, padding...)
		}

		// Encode padded value as base64
		// The decoded result will be exactly elemSizeBytes bytes
		encoded := base64.StdEncoding.EncodeToString(value)
		dbElements = append(dbElements, encoded)
	}

	m := len(dbElements)

	// Convert to C array of C strings
	cElements := make([]*C.char, m)
	cStrings := make([]*C.char, m)
	for i, elem := range dbElements {
		cStr := C.CString(elem)
		cStrings[i] = cStr
		cElements[i] = cStr
	}
	defer func() {
		for i := range cStrings {
			if cStrings[i] != nil {
				C.free(unsafe.Pointer(cStrings[i]))
			}
		}
	}()

	var shard C.struct_FrodoPIRShard
	var baseParamsPtr *C.uint8_t
	var baseParamsLen C.uintptr_t

	result := C.frodopir_shard_create(
		(**C.char)(unsafe.Pointer(&cElements[0])),
		C.uintptr_t(m),
		C.uintptr_t(lweDim),
		C.uintptr_t(m),
		C.uintptr_t(elemSize),
		C.uintptr_t(plaintextBits),
		&shard,
		&baseParamsPtr,
		&baseParamsLen,
	)

	if result != C.int(FrodoPIRResultSuccess) {
		return nil, nil, fmt.Errorf("failed to create shard: error code %d", result)
	}

	// Copy base params to Go slice
	baseParams := C.GoBytes(unsafe.Pointer(baseParamsPtr), C.int(baseParamsLen))
	baseParamsCopy := make([]byte, len(baseParams))
	copy(baseParamsCopy, baseParams)
	// Free the C-allocated memory
	C.frodopir_free_buffer(baseParamsPtr, C.uintptr_t(baseParamsLen))

	// Store key-to-index mapping (needed for client queries)
	// This is a simplified approach - in production, this mapping should be
	// handled differently (e.g., via OKVS or a separate key-index service)

	return &FrodoPIRServer{
		shard:         shard,
		lweDim:        uintptr(lweDim),
		m:             uintptr(m),
		elemSize:      uintptr(elemSize),
		plaintextBits: uintptr(plaintextBits),
	}, baseParamsCopy, nil
}

// ProcessQuery processes a PIR query and returns the response.
func (s *FrodoPIRServer) ProcessQuery(db []byte, query []byte) ([]byte, error) {
	// Note: db parameter is not used here because the shard already contains the database
	// This is a design issue - we need to reconsider the interface

	if len(query) == 0 {
		return nil, fmt.Errorf("query cannot be empty")
	}

	var responsePtr *C.uint8_t
	var responseLen C.uintptr_t

	result := C.frodopir_shard_respond(
		s.shard,
		(*C.uint8_t)(unsafe.Pointer(&query[0])),
		C.uintptr_t(len(query)),
		&responsePtr,
		&responseLen,
	)

	if result != C.int(FrodoPIRResultSuccess) {
		return nil, fmt.Errorf("failed to process query: error code %d", result)
	}

	// Copy response to Go slice
	response := C.GoBytes(unsafe.Pointer(responsePtr), C.int(responseLen))
	responseCopy := make([]byte, len(response))
	copy(responseCopy, response)
	// Free the C-allocated memory
	C.frodopir_free_buffer(responsePtr, C.uintptr_t(responseLen))

	return responseCopy, nil
}

// Close frees the server resources.
func (s *FrodoPIRServer) Close() error {
	C.frodopir_shard_free(s.shard)
	return nil
}

// FrodoPIRClient implements PIRClient using FrodoPIR
type FrodoPIRClient struct {
	client         C.struct_FrodoPIRQueryParams
	keyToIndex     map[string]int    // Maps keys to database indices
	queryParams    map[string][]byte // Store queryParams per query (key-based)
	lastQueriedKey string            // Track the last key that was queried (for DecodeResponse)
}

// NewFrodoPIRClient creates a new FrodoPIR client from serialized BaseParams.
// keyToIndex is a mapping from keys to database row indices.
func NewFrodoPIRClient(baseParams []byte, keyToIndex map[string]int) (*FrodoPIRClient, error) {
	var client C.struct_FrodoPIRQueryParams

	if len(baseParams) == 0 {
		return nil, fmt.Errorf("baseParams cannot be empty")
	}

	result := C.frodopir_client_create(
		(*C.uint8_t)(unsafe.Pointer(&baseParams[0])),
		C.uintptr_t(len(baseParams)),
		&client,
	)

	if result != C.int(FrodoPIRResultSuccess) {
		return nil, fmt.Errorf("failed to create client: error code %d", result)
	}

	return &FrodoPIRClient{
		client:         client,
		keyToIndex:     keyToIndex,
		queryParams:    make(map[string][]byte),
		lastQueriedKey: "",
	}, nil
}

// GenerateQuery creates a PIR query for a specific key.
// The key is mapped to a database index, and a query is generated for that index.
// The queryParams are stored internally for later use in DecodeResponse.
//
// Note: This method implements retry logic for overflow errors. FrodoPIR query generation
// can occasionally fail with an overflow error when random values in the query parameters
// are close to u32::MAX. Since query generation uses randomness, retrying will create
// a new QueryParams with different random values, often avoiding the overflow.
//
// This method implements the PIRClient interface.
func (c *FrodoPIRClient) GenerateQuery(key string) ([]byte, error) {
	return c.generateQueryWithRetry(key, 3) // Retry up to 3 times
}

// generateQueryWithRetry attempts to generate a query with retry logic for overflow errors.
// maxRetries specifies the maximum number of retry attempts.
func (c *FrodoPIRClient) generateQueryWithRetry(key string, maxRetries int) ([]byte, error) {
	// Find the index for this key
	index, ok := c.keyToIndex[key]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	// Validate index is non-negative (within database bounds)
	// Note: Upper bounds are checked inside FrodoPIR generate_query
	if index < 0 {
		return nil, fmt.Errorf("invalid index %d for key %s (must be non-negative)", index, key)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		var queryPtr *C.uint8_t
		var queryLen C.uintptr_t
		var queryParamsPtr *C.uint8_t
		var queryParamsLen C.uintptr_t

		result := C.frodopir_client_generate_query(
			c.client,
			C.uintptr_t(index),
			&queryPtr,
			&queryLen,
			&queryParamsPtr,
			&queryParamsLen,
		)

		if result == C.int(FrodoPIRResultSuccess) {
			// Success! Copy the results
			query := C.GoBytes(unsafe.Pointer(queryPtr), C.int(queryLen))
			queryCopy := make([]byte, len(query))
			copy(queryCopy, query)
			C.frodopir_free_buffer(queryPtr, C.uintptr_t(queryLen))

			queryParams := C.GoBytes(unsafe.Pointer(queryParamsPtr), C.int(queryParamsLen))
			queryParamsCopy := make([]byte, len(queryParams))
			copy(queryParamsCopy, queryParams)
			C.frodopir_free_buffer(queryParamsPtr, C.uintptr_t(queryParamsLen))

			// Store queryParams for this key (needed for decoding)
			c.queryParams[key] = queryParamsCopy
			// Track the last queried key so DecodeResponse knows which queryParams to use
			c.lastQueriedKey = key

			return queryCopy, nil
		}

		// Handle errors
		var errMsg string
		switch result {
		case C.int(FrodoPIRResultQueryParamsReused):
			// QueryParams reused is not retryable - this indicates a logic error
			return nil, fmt.Errorf("query params reused (non-retryable error)")
		case C.int(FrodoPIRResultOverflownAdd):
			// Overflow error - retryable since it's probabilistic
			errMsg = "overflow in addition (probabilistic error, will retry)"
			lastErr = fmt.Errorf("failed to generate query after %d attempts: %s (error code %d)", attempt+1, errMsg, result)
			// Continue to retry
			continue
		case C.int(FrodoPIRResultInvalidInput):
			// Invalid input is not retryable
			return nil, fmt.Errorf("invalid input (row_index out of bounds, non-retryable)")
		default:
			// Unknown error - not retryable
			errMsg = fmt.Sprintf("unknown error code %d", result)
			return nil, fmt.Errorf("failed to generate query: %s", errMsg)
		}
	}

	// All retries exhausted
	return nil, fmt.Errorf("failed to generate query after %d retries: %w", maxRetries+1, lastErr)
}

// DecodeResponse decodes a PIR server response to extract the value.
// It uses the queryParams from the most recently queried key (stored via lastQueriedKey).
// Note: This method doesn't take a key parameter. If multiple queries have been made,
// it will use the queryParams for the last key that was queried.
//
// TODO: Consider updating interface to accept key parameter for explicit matching
func (c *FrodoPIRClient) DecodeResponse(response []byte) ([]byte, error) {
	if len(c.queryParams) == 0 {
		return nil, fmt.Errorf("no queryParams available - call GenerateQuery first")
	}

	// Use the queryParams for the last queried key
	var queryParams []byte
	var ok bool
	if c.lastQueriedKey != "" {
		queryParams, ok = c.queryParams[c.lastQueriedKey]
		if !ok {
			return nil, fmt.Errorf("queryParams not found for last queried key: %s", c.lastQueriedKey)
		}
	} else {
		// Fallback: use the first available queryParams (shouldn't happen in normal flow)
		for _, qp := range c.queryParams {
			queryParams = qp
			break
		}
	}

	var outputPtr *C.uint8_t
	var outputLen C.uintptr_t

	if len(response) == 0 {
		return nil, fmt.Errorf("response cannot be empty")
	}

	if len(queryParams) == 0 {
		return nil, fmt.Errorf("queryParams cannot be empty")
	}

	result := C.frodopir_client_decode_response(
		c.client,
		(*C.uint8_t)(unsafe.Pointer(&response[0])),
		C.uintptr_t(len(response)),
		(*C.uint8_t)(unsafe.Pointer(&queryParams[0])),
		C.uintptr_t(len(queryParams)),
		&outputPtr,
		&outputLen,
	)

	if result != C.int(FrodoPIRResultSuccess) {
		return nil, fmt.Errorf("failed to decode response: error code %d", result)
	}

	// Copy output to Go slice
	output := C.GoBytes(unsafe.Pointer(outputPtr), C.int(outputLen))
	outputCopy := make([]byte, len(output))
	copy(outputCopy, output)
	// Free the C-allocated memory
	C.frodopir_free_buffer(outputPtr, C.uintptr_t(outputLen))

	// The output from FrodoPIR is already decoded bytes (parse_output_as_bytes returns Vec<u8>)
	// We stored values as bytes, padded to elemSize, then base64-encoded for storage
	// FrodoPIR decodes and returns the raw bytes, so we can return them directly
	// However, we need to remove the padding zeros we added

	// Note: For OKVS integration, we store float64 values (8 bytes).
	// When we decode from PIR, we get padded bytes (elemSizeBytes, typically 64 bytes).
	// We need to extract the original value size. For float64, it's always 8 bytes.
	// For now, we'll try to preserve the original size by checking if it's a float64-sized value.

	// Find the actual value length (remove trailing zeros)
	// But preserve at least 8 bytes for float64 values (our use case)
	minSize := 8 // Minimum size for float64
	actualLen := len(outputCopy)
	for actualLen > minSize && outputCopy[actualLen-1] == 0 {
		actualLen--
	}

	// Ensure we return at least the minimum size (for float64)
	if actualLen < minSize {
		actualLen = minSize
	}

	// Return the unpadded value (but at least minSize bytes)
	if actualLen < len(outputCopy) {
		return outputCopy[:actualLen], nil
	}

	return outputCopy, nil
}

// Close frees the client resources.
func (c *FrodoPIRClient) Close() error {
	C.frodopir_client_free(c.client)
	return nil
}

// MockPIRServer is kept for backward compatibility and testing.
type MockPIRServer struct{}

func (m *MockPIRServer) ProcessQuery(db []byte, query []byte) ([]byte, error) {
	return query, nil
}

func (m *MockPIRServer) Close() error {
	return nil
}

// MockPIRClient is kept for backward compatibility and testing.
type MockPIRClient struct{}

func (m *MockPIRClient) GenerateQuery(key string) ([]byte, error) {
	return []byte("query:" + key), nil
}

func (m *MockPIRClient) DecodeResponse(response []byte) ([]byte, error) {
	return response, nil
}

func (m *MockPIRClient) Close() error {
	return nil
}

// Package crypto provides cryptographic primitives for silhouette-db.
// This file implements RB-OKVS encoder/decoder using cgo to call the Rust FFI library.

//go:build cgo
// +build cgo

package crypto

/*
#cgo LDFLAGS: -L${SRCDIR}/../../third_party/rb-okvs-ffi/target/release -lrbokvsffi -lm -ldl
#cgo CFLAGS: -I${SRCDIR}/../../third_party/rb-okvs-ffi

#include "rb_okvs_ffi.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"encoding/binary"
	"fmt"
	"unsafe"
)

// RBOKVSResult represents error codes from RB-OKVS FFI
type RBOKVSResult int

const (
	RBOKVSResultSuccess RBOKVSResult = iota
	RBOKVSResultInvalidInput
	RBOKVSResultSerializationError
	RBOKVSResultDeserializationError
	RBOKVSResultEncodingError
	RBOKVSResultDecodingError
	RBOKVSResultUnknownError = 99
)

// RBOKVSEncoder implements OKVSEncoder using RB-OKVS for float64 values
type RBOKVSEncoder struct{}

// NewRBOKVSEncoder creates a new RB-OKVS encoder
func NewRBOKVSEncoder() *RBOKVSEncoder {
	return &RBOKVSEncoder{}
}

// Encode takes a map of key-value pairs and returns an OKVS-encoded blob.
// Values must be 8-byte float64 values (as []byte, little-endian).
func (e *RBOKVSEncoder) Encode(pairs map[string][]byte) ([]byte, error) {
	if len(pairs) == 0 {
		return nil, fmt.Errorf("cannot encode empty pairs")
	}

	// Note: Library requires 100+ pairs for reliable operation
	// We should check this or document it
	if len(pairs) < 100 {
		return nil, fmt.Errorf("RB-OKVS requires at least 100 key-value pairs for reliable operation, got %d", len(pairs))
	}

	// Prepare arrays for C FFI
	numPairs := len(pairs)

	// Allocate C string array for keys
	keysC := C.malloc(C.size_t(numPairs) * C.size_t(unsafe.Sizeof(uintptr(0))))
	if keysC == nil {
		return nil, fmt.Errorf("failed to allocate memory for keys")
	}
	defer C.free(keysC)

	keysPtr := (*[1 << 30]*C.char)(keysC)

	// Allocate C double array for values
	valuesC := C.malloc(C.size_t(numPairs) * C.size_t(unsafe.Sizeof(C.double(0))))
	if valuesC == nil {
		return nil, fmt.Errorf("failed to allocate memory for values")
	}
	defer C.free(valuesC)

	valuesPtr := (*[1 << 30]C.double)(valuesC)

	// Convert Go map to C arrays and keep references for cleanup
	var cStrings []*C.char
	defer func() {
		// Free all C strings
		for _, cs := range cStrings {
			if cs != nil {
				C.free(unsafe.Pointer(cs))
			}
		}
	}()

	idx := 0
	for key, valueBytes := range pairs {
		// Convert value bytes to float64
		if len(valueBytes) != 8 {
			return nil, fmt.Errorf("value must be 8 bytes (float64), got %d bytes for key %s", len(valueBytes), key)
		}

		value := binary.LittleEndian.Uint64(valueBytes)
		valueF64 := *(*float64)(unsafe.Pointer(&value))

		// Convert key to C string
		keyC := C.CString(key)
		cStrings = append(cStrings, keyC)
		keysPtr[idx] = keyC

		// Store value
		valuesPtr[idx] = C.double(valueF64)

		idx++
	}

	// Call FFI encode function
	var encodingOut *C.uint8_t
	var encodingLen C.uintptr_t

	result := C.rb_okvs_encode(
		(**C.char)(keysC),
		(*C.double)(valuesC),
		C.uintptr_t(numPairs),
		&encodingOut,
		&encodingLen,
	)

	if result != C.int(RBOKVSResultSuccess) {
		return nil, fmt.Errorf("rb_okvs_encode failed with error code %d", result)
	}

	if encodingOut == nil || encodingLen == 0 {
		return nil, fmt.Errorf("rb_okvs_encode returned null or empty encoding")
	}

	// Copy encoding to Go slice
	encoding := C.GoBytes(unsafe.Pointer(encodingOut), C.int(encodingLen))
	encodingCopy := make([]byte, len(encoding))
	copy(encodingCopy, encoding)

	// Free the C-allocated buffer
	C.rb_okvs_free_buffer(encodingOut, encodingLen)

	return encodingCopy, nil
}

// RBOKVSDecoder implements OKVSDecoder for float64 values
type RBOKVSDecoder struct {
	encoding []byte
}

// NewRBOKVSDecoder creates a new RB-OKVS decoder from an encoded blob
func NewRBOKVSDecoder(encoding []byte) *RBOKVSDecoder {
	return &RBOKVSDecoder{
		encoding: encoding,
	}
}

// Decode takes an OKVS blob and a key, and returns the corresponding float64 value.
func (d *RBOKVSDecoder) Decode(okvsBlob []byte, key string) ([]byte, error) {
	if len(okvsBlob) == 0 {
		return nil, fmt.Errorf("okvsBlob cannot be empty")
	}

	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	// Convert key to C string
	keyC := C.CString(key)
	defer C.free(unsafe.Pointer(keyC))

	// Call FFI decode function
	var valueOut C.double

	result := C.rb_okvs_decode(
		(*C.uint8_t)(unsafe.Pointer(&okvsBlob[0])),
		C.uintptr_t(len(okvsBlob)),
		keyC,
		&valueOut,
	)

	if result != C.int(RBOKVSResultSuccess) {
		return nil, fmt.Errorf("rb_okvs_decode failed with error code %d", result)
	}

	// Convert float64 to bytes (little-endian)
	valueBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(valueBytes, *(*uint64)(unsafe.Pointer(&valueOut)))

	return valueBytes, nil
}

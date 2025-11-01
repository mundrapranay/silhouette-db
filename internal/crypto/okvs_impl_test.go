//go:build cgo
// +build cgo

package crypto

import (
	"encoding/binary"
	"testing"
	"unsafe"
)

func TestRBOKVSEncoder_Encode(t *testing.T) {
	encoder := NewRBOKVSEncoder()

	// Create 100 pairs (minimum for reliable operation)
	pairs := make(map[string][]byte)
	for i := 0; i < 100; i++ {
		key := "key" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		valueF64 := float64(i) * 0.123
		valueBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(valueBytes, *(*uint64)(unsafe.Pointer(&valueF64)))
		pairs[key] = valueBytes
	}

	encoding, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(encoding) == 0 {
		t.Fatal("Encoding should not be empty")
	}

	// Verify encoding size is reasonable (~10-20% overhead)
	expectedMinSize := len(pairs) * 8     // At least one value per pair
	expectedMaxSize := len(pairs) * 8 * 2 // Max ~2x overhead
	if len(encoding) < expectedMinSize {
		t.Errorf("Encoding too small: got %d, expected at least %d", len(encoding), expectedMinSize)
	}
	if len(encoding) > expectedMaxSize {
		t.Errorf("Encoding too large: got %d, expected at most %d", len(encoding), expectedMaxSize)
	}

	t.Logf("Encoded %d pairs into %d bytes (%.2f%% overhead)",
		len(pairs), len(encoding),
		float64(len(encoding)-expectedMinSize)/float64(expectedMinSize)*100)
}

func TestRBOKVSEncoder_Encode_EmptyPairs(t *testing.T) {
	encoder := NewRBOKVSEncoder()
	pairs := make(map[string][]byte)

	_, err := encoder.Encode(pairs)
	if err == nil {
		t.Fatal("Encode should fail with empty pairs")
	}
}

func TestRBOKVSEncoder_Encode_TooFewPairs(t *testing.T) {
	encoder := NewRBOKVSEncoder()
	pairs := make(map[string][]byte)

	// Add only 10 pairs (below minimum of 100)
	for i := 0; i < 10; i++ {
		key := "key" + string(rune('0'+i))
		valueF64 := float64(i)
		valueBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(valueBytes, *(*uint64)(unsafe.Pointer(&valueF64)))
		pairs[key] = valueBytes
	}

	_, err := encoder.Encode(pairs)
	if err == nil {
		t.Fatal("Encode should fail with too few pairs")
	}
}

func TestRBOKVSDecoder_Decode(t *testing.T) {
	encoder := NewRBOKVSEncoder()

	// Create 100 pairs
	pairs := make(map[string][]byte)
	testKeys := make([]string, 0, 100)
	testValues := make([]float64, 0, 100)

	for i := 0; i < 100; i++ {
		key := "key" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		valueF64 := float64(i) * 0.123
		valueBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(valueBytes, *(*uint64)(unsafe.Pointer(&valueF64)))
		pairs[key] = valueBytes
		testKeys = append(testKeys, key)
		testValues = append(testValues, valueF64)
	}

	// Encode
	encoding, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoder := NewRBOKVSDecoder(encoding)

	// Test decoding a few keys
	for i := 0; i < 10; i++ {
		key := testKeys[i]
		expectedValue := testValues[i]

		decodedBytes, err := decoder.Decode(encoding, key)
		if err != nil {
			t.Fatalf("Decode failed for key %s: %v", key, err)
		}

		if len(decodedBytes) != 8 {
			t.Fatalf("Decoded value should be 8 bytes, got %d for key %s", len(decodedBytes), key)
		}

		// Convert back to float64
		decodedValue := *(*float64)(unsafe.Pointer(&decodedBytes[0]))

		// Use approximate equality for floating point comparison
		epsilon := 1e-10
		diff := decodedValue - expectedValue
		if diff < 0 {
			diff = -diff
		}
		if diff > epsilon && decodedValue != expectedValue {
			t.Errorf("Decoded value mismatch for key %s: expected %f, got %f (diff: %e)",
				key, expectedValue, decodedValue, diff)
		}
	}
}

func TestRBOKVSDecoder_Decode_InvalidKey(t *testing.T) {
	encoder := NewRBOKVSEncoder()

	// Create 100 pairs
	pairs := make(map[string][]byte)
	for i := 0; i < 100; i++ {
		key := "key" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		valueF64 := float64(i) * 0.123
		valueBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(valueBytes, *(*uint64)(unsafe.Pointer(&valueF64)))
		pairs[key] = valueBytes
	}

	encoding, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder := NewRBOKVSDecoder(encoding)

	// Try to decode with invalid key
	_, err = decoder.Decode(encoding, "nonexistent_key")
	// Decode might succeed even with invalid key (OKVS property),
	// or it might return an error or garbage data
	// We'll just verify it doesn't panic
	if err != nil {
		t.Logf("Decode with invalid key returned error (expected): %v", err)
	}
}

func TestRBOKVS_EncodeDecode_AllPairs(t *testing.T) {
	encoder := NewRBOKVSEncoder()

	// Create 100 pairs
	pairs := make(map[string][]byte)
	for i := 0; i < 100; i++ {
		key := "key" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		valueF64 := float64(i) * 0.123
		valueBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(valueBytes, *(*uint64)(unsafe.Pointer(&valueF64)))
		pairs[key] = valueBytes
	}

	// Encode
	encoding, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode all pairs
	decoder := NewRBOKVSDecoder(encoding)

	for key, originalBytes := range pairs {
		decodedBytes, err := decoder.Decode(encoding, key)
		if err != nil {
			t.Errorf("Decode failed for key %s: %v", key, err)
			continue
		}

		if len(decodedBytes) != len(originalBytes) {
			t.Errorf("Decoded value length mismatch for key %s: expected %d, got %d",
				key, len(originalBytes), len(decodedBytes))
			continue
		}

		// Compare as float64
		originalValue := *(*float64)(unsafe.Pointer(&originalBytes[0]))
		decodedValue := *(*float64)(unsafe.Pointer(&decodedBytes[0]))

		epsilon := 1e-10
		diff := decodedValue - originalValue
		if diff < 0 {
			diff = -diff
		}
		if diff > epsilon && decodedValue != originalValue {
			t.Errorf("Decoded value mismatch for key %s: expected %f, got %f (diff: %e)",
				key, originalValue, decodedValue, diff)
		}
	}
}

package crypto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"testing"
)

func TestKVSEncoder_Encode_EmptyMap(t *testing.T) {
	encoder := NewKVSEncoder()
	pairs := make(map[string][]byte)

	blob, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if blob == nil {
		t.Fatal("Encode returned nil blob")
	}

	// Empty map should serialize to "{}"
	if len(blob) == 0 {
		t.Fatal("Encode returned empty blob")
	}
}

func TestKVSEncoder_Encode_SinglePair(t *testing.T) {
	encoder := NewKVSEncoder()
	pairs := map[string][]byte{
		"key1": []byte("value1"),
	}

	blob, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	if len(blob) == 0 {
		t.Fatal("Encode returned empty blob")
	}

	// Verify we can decode it
	decoder, err := NewKVSDecoder(blob)
	if err != nil {
		t.Fatalf("NewKVSDecoder failed: %v", err)
	}

	value, err := decoder.Decode(blob, "key1")
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if !bytes.Equal(value, []byte("value1")) {
		t.Errorf("Expected value1, got %s", string(value))
	}
}

func TestKVSEncoder_Encode_MultiplePairs(t *testing.T) {
	encoder := NewKVSEncoder()
	pairs := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	blob, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder, err := NewKVSDecoder(blob)
	if err != nil {
		t.Fatalf("NewKVSDecoder failed: %v", err)
	}

	// Verify all pairs
	for k, expected := range pairs {
		value, err := decoder.Decode(blob, k)
		if err != nil {
			t.Fatalf("Decode failed for key %s: %v", k, err)
		}
		if !bytes.Equal(value, expected) {
			t.Errorf("Expected %s for key %s, got %s", string(expected), k, string(value))
		}
	}
}

func TestKVSEncoder_Encode_Float64Values(t *testing.T) {
	encoder := NewKVSEncoder()
	pairs := map[string][]byte{
		"key1": float64ToBytes(1.5),
		"key2": float64ToBytes(2.5),
		"key3": float64ToBytes(3.14159),
	}

	blob, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder, err := NewKVSDecoder(blob)
	if err != nil {
		t.Fatalf("NewKVSDecoder failed: %v", err)
	}

	// Verify float64 values
	testCases := map[string]float64{
		"key1": 1.5,
		"key2": 2.5,
		"key3": 3.14159,
	}

	for k, expected := range testCases {
		value, err := decoder.Decode(blob, k)
		if err != nil {
			t.Fatalf("Decode failed for key %s: %v", k, err)
		}
		actual := bytesToFloat64(value)
		if math.Abs(actual-expected) > 0.0001 {
			t.Errorf("Expected %f for key %s, got %f", expected, k, actual)
		}
	}
}

func TestKVSEncoder_Encode_LargeDataset(t *testing.T) {
	encoder := NewKVSEncoder()
	pairs := make(map[string][]byte, 1000)

	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := float64ToBytes(float64(i))
		pairs[key] = value
	}

	blob, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder, err := NewKVSDecoder(blob)
	if err != nil {
		t.Fatalf("NewKVSDecoder failed: %v", err)
	}

	// Verify random sample
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i*10)
		value, err := decoder.Decode(blob, key)
		if err != nil {
			t.Fatalf("Decode failed for key %s: %v", key, err)
		}
		expected := float64(i * 10)
		actual := bytesToFloat64(value)
		if math.Abs(actual-expected) > 0.0001 {
			t.Errorf("Expected %f for key %s, got %f", expected, key, actual)
		}
	}
}

func TestKVSEncoder_Encode_SpecialCharacters(t *testing.T) {
	encoder := NewKVSEncoder()
	pairs := map[string][]byte{
		"key-with-dash":       []byte("value-with-dash"),
		"key_with_underscore": []byte("value_with_underscore"),
		"key.with.dots":       []byte("value.with.dots"),
		"key:with:colons":     []byte("value:with:colons"),
	}

	blob, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder, err := NewKVSDecoder(blob)
	if err != nil {
		t.Fatalf("NewKVSDecoder failed: %v", err)
	}

	for k, expected := range pairs {
		value, err := decoder.Decode(blob, k)
		if err != nil {
			t.Fatalf("Decode failed for key %s: %v", k, err)
		}
		if !bytes.Equal(value, expected) {
			t.Errorf("Expected %s for key %s, got %s", string(expected), k, string(value))
		}
	}
}

func TestKVSDecoder_Decode_NonExistentKey(t *testing.T) {
	encoder := NewKVSEncoder()
	pairs := map[string][]byte{
		"key1": []byte("value1"),
	}

	blob, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder, err := NewKVSDecoder(blob)
	if err != nil {
		t.Fatalf("NewKVSDecoder failed: %v", err)
	}

	_, err = decoder.Decode(blob, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent key")
	}
}

func TestKVSDecoder_Decode_EmptyBlob(t *testing.T) {
	_, err := NewKVSDecoder([]byte{})
	if err == nil {
		t.Fatal("Expected error for empty blob")
	}
}

func TestKVSDecoder_RoundTrip(t *testing.T) {
	encoder := NewKVSEncoder()
	originalPairs := map[string][]byte{
		"key1": float64ToBytes(1.0),
		"key2": float64ToBytes(2.0),
		"key3": float64ToBytes(3.0),
		"key4": []byte("string-value"),
		"key5": []byte{0x00, 0x01, 0x02, 0x03, 0xFF},
	}

	// Encode
	blob, err := encoder.Encode(originalPairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// Decode
	decoder, err := NewKVSDecoder(blob)
	if err != nil {
		t.Fatalf("NewKVSDecoder failed: %v", err)
	}

	// Verify all pairs
	for k, expected := range originalPairs {
		value, err := decoder.Decode(blob, k)
		if err != nil {
			t.Fatalf("Decode failed for key %s: %v", k, err)
		}
		if !bytes.Equal(value, expected) {
			t.Errorf("Round-trip failed for key %s: expected %v, got %v", k, expected, value)
		}
	}
}

func TestKVSEncoder_Encode_NilMap(t *testing.T) {
	encoder := NewKVSEncoder()
	_, err := encoder.Encode(nil)
	if err == nil {
		t.Fatal("Expected error for nil map")
	}
}

func TestKVSDecoder_Decode_EmptyKey(t *testing.T) {
	encoder := NewKVSEncoder()
	pairs := map[string][]byte{
		"key1": []byte("value1"),
	}

	blob, err := encoder.Encode(pairs)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoder, err := NewKVSDecoder(blob)
	if err != nil {
		t.Fatalf("NewKVSDecoder failed: %v", err)
	}

	_, err = decoder.Decode(blob, "")
	if err == nil {
		t.Fatal("Expected error for empty key")
	}
}

// Helper functions for float64 conversion (same as in other test files)
func float64ToBytes(f float64) []byte {
	bytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(bytes, math.Float64bits(f))
	return bytes
}

func bytesToFloat64(bytes []byte) float64 {
	if len(bytes) < 8 {
		return 0.0
	}
	bits := binary.LittleEndian.Uint64(bytes[:8])
	return math.Float64frombits(bits)
}

package crypto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// KVSEncoder implements OKVSEncoder using simple key-value storage.
// This is a non-oblivious implementation that stores pairs as a JSON-serialized map.
// It's faster and simpler than OKVS but doesn't provide oblivious properties.
type KVSEncoder struct{}

// NewKVSEncoder creates a new simple KV store encoder
func NewKVSEncoder() *KVSEncoder {
	return &KVSEncoder{}
}

// Encode takes a map of key-value pairs and returns a JSON-serialized blob.
// Unlike OKVS, this doesn't hide the keys or their mapping, but it's faster
// and works with any number of pairs (no minimum requirement).
func (e *KVSEncoder) Encode(pairs map[string][]byte) ([]byte, error) {
	if pairs == nil {
		return nil, fmt.Errorf("pairs map cannot be nil")
	}

	// Convert []byte values to base64 strings for JSON encoding
	data := make(map[string]string, len(pairs))
	for k, v := range pairs {
		// Use base64 encoding for binary data in JSON
		data[k] = base64Encode(v)
	}

	// Serialize to JSON
	blob, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize KVS data: %w", err)
	}

	return blob, nil
}

// KVSDecoder implements OKVSDecoder using simple key-value storage.
// It deserializes the JSON blob and provides O(1) lookup by key.
type KVSDecoder struct {
	pairs map[string][]byte
}

// NewKVSDecoder creates a new KVS decoder from an encoded blob.
func NewKVSDecoder(blob []byte) (*KVSDecoder, error) {
	if len(blob) == 0 {
		return nil, fmt.Errorf("blob cannot be empty")
	}

	// Deserialize JSON
	var data map[string]string
	if err := json.Unmarshal(blob, &data); err != nil {
		return nil, fmt.Errorf("failed to deserialize KVS data: %w", err)
	}

	// Convert base64 strings back to []byte
	pairs := make(map[string][]byte, len(data))
	for k, v := range data {
		decoded, err := base64Decode(v)
		if err != nil {
			return nil, fmt.Errorf("failed to decode value for key %s: %w", k, err)
		}
		pairs[k] = decoded
	}

	return &KVSDecoder{
		pairs: pairs,
	}, nil
}

// Decode takes a KVS blob and a key, and returns the corresponding value.
// Note: The blob parameter is ignored (we use the pre-deserialized pairs).
// This is kept for interface compatibility with OKVSDecoder.
func (d *KVSDecoder) Decode(okvsBlob []byte, key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	value, exists := d.pairs[key]
	if !exists {
		return nil, fmt.Errorf("key %s not found in KVS", key)
	}

	return value, nil
}

// base64Encode encodes binary data as a base64 string for JSON compatibility
func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// base64Decode decodes a base64 string back to binary data
func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

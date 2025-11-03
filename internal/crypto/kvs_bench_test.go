package crypto

import (
	"fmt"
	"testing"
)

// BenchmarkKVSEncoder_Encode benchmarks KVS encoding performance
func BenchmarkKVSEncoder_Encode(b *testing.B) {
	encoder := NewKVSEncoder()

	// Test with different sizes
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		pairs := make(map[string][]byte, size)
		for i := 0; i < size; i++ {
			key := fmt.Sprintf("key%d", i)
			value := float64ToBytes(float64(i))
			pairs[key] = value
		}

		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := encoder.Encode(pairs)
				if err != nil {
					b.Fatalf("Encode failed: %v", err)
				}
			}
		})
	}
}

// BenchmarkKVSDecoder_Decode benchmarks KVS decoding performance
func BenchmarkKVSDecoder_Decode(b *testing.B) {
	encoder := NewKVSEncoder()

	// Test with different sizes
	sizes := []int{10, 100, 1000, 10000}
	for _, size := range sizes {
		pairs := make(map[string][]byte, size)
		for i := 0; i < size; i++ {
			key := fmt.Sprintf("key%d", i)
			value := float64ToBytes(float64(i))
			pairs[key] = value
		}

		blob, err := encoder.Encode(pairs)
		if err != nil {
			b.Fatalf("Encode failed: %v", err)
		}

		decoder, err := NewKVSDecoder(blob)
		if err != nil {
			b.Fatalf("NewKVSDecoder failed: %v", err)
		}

		// Benchmark single key lookup
		b.Run(fmt.Sprintf("SingleKey_Size_%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := decoder.Decode(blob, "key0")
				if err != nil {
					b.Fatalf("Decode failed: %v", err)
				}
			}
		})

		// Benchmark decoding all keys
		b.Run(fmt.Sprintf("AllKeys_Size_%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for key := range pairs {
					_, err := decoder.Decode(blob, key)
					if err != nil {
						b.Fatalf("Decode failed for key %s: %v", key, err)
					}
				}
			}
		})
	}
}

// BenchmarkKVS_vs_OKVS_Encoding compares KVS and OKVS encoding performance
func BenchmarkKVS_vs_OKVS_Encoding(b *testing.B) {
	// Only run if cgo is available (OKVS requires cgo)
	// This is a comparison benchmark
	kvsEncoder := NewKVSEncoder()

	sizes := []int{100, 1000, 10000}
	for _, size := range sizes {
		pairs := make(map[string][]byte, size)
		for i := 0; i < size; i++ {
			key := fmt.Sprintf("key%d", i)
			value := float64ToBytes(float64(i))
			pairs[key] = value
		}

		// Benchmark KVS encoding
		b.Run(fmt.Sprintf("KVS_Size_%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := kvsEncoder.Encode(pairs)
				if err != nil {
					b.Fatalf("KVS Encode failed: %v", err)
				}
			}
		})

		// Benchmark OKVS encoding (only if size >= 100 and cgo available)
		if size >= 100 {
			okvsEncoder := NewRBOKVSEncoder()
			if okvsEncoder != nil {
				b.Run(fmt.Sprintf("OKVS_Size_%d", size), func(b *testing.B) {
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, err := okvsEncoder.Encode(pairs)
						if err != nil {
							b.Fatalf("OKVS Encode failed: %v", err)
						}
					}
				})
			}
		}
	}
}

// BenchmarkKVS_vs_OKVS_Decoding compares KVS and OKVS decoding performance
func BenchmarkKVS_vs_OKVS_Decoding(b *testing.B) {
	kvsEncoder := NewKVSEncoder()

	sizes := []int{100, 1000, 10000}
	for _, size := range sizes {
		pairs := make(map[string][]byte, size)
		for i := 0; i < size; i++ {
			key := fmt.Sprintf("key%d", i)
			value := float64ToBytes(float64(i))
			pairs[key] = value
		}

		// KVS encoding and decoding
		kvsBlob, err := kvsEncoder.Encode(pairs)
		if err != nil {
			b.Fatalf("KVS Encode failed: %v", err)
		}

		kvsDecoder, err := NewKVSDecoder(kvsBlob)
		if err != nil {
			b.Fatalf("NewKVSDecoder failed: %v", err)
		}

		// Benchmark KVS decoding
		b.Run(fmt.Sprintf("KVS_SingleKey_Size_%d", size), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := kvsDecoder.Decode(kvsBlob, "key0")
				if err != nil {
					b.Fatalf("KVS Decode failed: %v", err)
				}
			}
		})

		// OKVS encoding and decoding (only if size >= 100 and cgo available)
		if size >= 100 {
			okvsEncoder := NewRBOKVSEncoder()
			if okvsEncoder != nil {
				okvsBlob, err := okvsEncoder.Encode(pairs)
				if err != nil {
					b.Fatalf("OKVS Encode failed: %v", err)
				}

				okvsDecoder := NewRBOKVSDecoder(okvsBlob)

				// Benchmark OKVS decoding
				b.Run(fmt.Sprintf("OKVS_SingleKey_Size_%d", size), func(b *testing.B) {
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						_, err := okvsDecoder.Decode(okvsBlob, "key0")
						if err != nil {
							b.Fatalf("OKVS Decode failed: %v", err)
						}
					}
				})
			}
		}
	}
}

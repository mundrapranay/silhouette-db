package server

import (
	"fmt"
	"testing"

	"github.com/mundrapranay/silhouette-db/internal/crypto"
)

// BenchmarkPIR_ShardCreation benchmarks FrodoPIR shard creation.
func BenchmarkPIR_ShardCreation(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping PIR benchmark in short mode")
	}

	// Create test data
	pairs := make(map[string][]byte, 100)
	for i := 0; i < 100; i++ {
		pairs[fmt.Sprintf("key%d", i)] = []byte(fmt.Sprintf("value%d", i))
	}

	lweDim := 512
	elemSize := 8192
	plaintextBits := 10

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := crypto.NewFrodoPIRServer(pairs, lweDim, elemSize, plaintextBits)
		if err != nil {
			b.Fatalf("Failed to create PIR server: %v", err)
		}
		// Note: We're not closing the server here to benchmark creation time
		// In production, you should close it
	}
}

// BenchmarkPIR_QueryGeneration benchmarks PIR query generation.
func BenchmarkPIR_QueryGeneration(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping PIR benchmark in short mode")
	}

	// Setup: Create server and client
	pairs := make(map[string][]byte, 100)
	keyToIndex := make(map[string]int, 100)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		pairs[key] = []byte(fmt.Sprintf("value%d", i))
		keyToIndex[key] = i
	}

	lweDim := 512
	elemSize := 8192
	plaintextBits := 10

	pirServer, baseParams, err := crypto.NewFrodoPIRServer(pairs, lweDim, elemSize, plaintextBits)
	if err != nil {
		b.Fatalf("Failed to create PIR server: %v", err)
	}
	defer pirServer.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a new client for each query to avoid QueryParams reuse issue
		pirClient, err := crypto.NewFrodoPIRClient(baseParams, keyToIndex)
		if err != nil {
			b.Fatalf("Failed to create PIR client: %v", err)
		}

		key := fmt.Sprintf("key%d", i%100)
		_, err = pirClient.GenerateQuery(key)
		if err != nil {
			pirClient.Close()
			b.Fatalf("Failed to generate query: %v", err)
		}
		pirClient.Close()
	}
}

// BenchmarkPIR_QueryProcessing benchmarks PIR query processing on server.
func BenchmarkPIR_QueryProcessing(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping PIR benchmark in short mode")
	}

	// Setup: Create server and client
	pairs := make(map[string][]byte, 100)
	keyToIndex := make(map[string]int, 100)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		pairs[key] = []byte(fmt.Sprintf("value%d", i))
		keyToIndex[key] = i
	}

	lweDim := 512
	elemSize := 8192
	plaintextBits := 10

	pirServer, baseParams, err := crypto.NewFrodoPIRServer(pairs, lweDim, elemSize, plaintextBits)
	if err != nil {
		b.Fatalf("Failed to create PIR server: %v", err)
	}
	defer pirServer.Close()

	pirClient, err := crypto.NewFrodoPIRClient(baseParams, keyToIndex)
	if err != nil {
		b.Fatalf("Failed to create PIR client: %v", err)
	}
	defer pirClient.Close()

	// Generate query once
	query, err := pirClient.GenerateQuery("key0")
	if err != nil {
		b.Fatalf("Failed to generate query: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pirServer.ProcessQuery(nil, query)
		if err != nil {
			b.Fatalf("Failed to process query: %v", err)
		}
	}
}

// BenchmarkPIR_EndToEnd benchmarks complete PIR workflow (query generation + processing + decoding).
func BenchmarkPIR_EndToEnd(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping PIR benchmark in short mode")
	}

	// Setup: Create server
	pairs := make(map[string][]byte, 100)
	keyToIndex := make(map[string]int, 100)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		pairs[key] = []byte(fmt.Sprintf("value%d", i))
		keyToIndex[key] = i
	}

	lweDim := 512
	elemSize := 8192
	plaintextBits := 10

	pirServer, baseParams, err := crypto.NewFrodoPIRServer(pairs, lweDim, elemSize, plaintextBits)
	if err != nil {
		b.Fatalf("Failed to create PIR server: %v", err)
	}
	defer pirServer.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create a new client for each query to avoid QueryParams reuse issue
		pirClient, err := crypto.NewFrodoPIRClient(baseParams, keyToIndex)
		if err != nil {
			b.Fatalf("Failed to create PIR client: %v", err)
		}

		key := fmt.Sprintf("key%d", i%100)

		// Generate query
		query, err := pirClient.GenerateQuery(key)
		if err != nil {
			pirClient.Close()
			b.Fatalf("Failed to generate query: %v", err)
		}

		// Process query
		response, err := pirServer.ProcessQuery(nil, query)
		if err != nil {
			pirClient.Close()
			b.Fatalf("Failed to process query: %v", err)
		}

		// Decode response
		_, err = pirClient.DecodeResponse(response)
		if err != nil {
			pirClient.Close()
			b.Fatalf("Failed to decode response: %v", err)
		}

		pirClient.Close()
	}
}

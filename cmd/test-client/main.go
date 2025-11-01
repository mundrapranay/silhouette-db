package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
	"unsafe"

	"github.com/mundrapranay/silhouette-db/pkg/client"
)

var (
	serverAddr = flag.String("server", "127.0.0.1:9090", "Server address (host:port)")
	numPairs   = flag.Int("pairs", 150, "Number of key-value pairs to publish")
	roundID    = flag.Uint64("round", 1, "Round ID")
	testKey    = flag.String("key", "", "Specific key to query (optional)")
)

// float64ToBytes converts a float64 to 8-byte little-endian bytes
func float64ToBytes(f float64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, *(*uint64)(unsafe.Pointer(&f)))
	return buf
}

// bytesToFloat64 converts 8-byte little-endian bytes to float64
func bytesToFloat64(b []byte) float64 {
	return *(*float64)(unsafe.Pointer(&b[0]))
}

func main() {
	flag.Parse()

	ctx := context.Background()

	// Create client (PIR client will be initialized automatically when needed)
	fmt.Printf("🔌 Connecting to server at %s...\n", *serverAddr)
	c, err := client.NewClient(*serverAddr, nil)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()
	fmt.Printf("✅ Connected successfully!\n\n")

	// Step 1: Start round
	fmt.Printf("📋 Starting round %d with 1 worker...\n", *roundID)
	err = c.StartRound(ctx, *roundID, 1)
	if err != nil {
		log.Fatalf("Failed to start round: %v", err)
	}
	fmt.Printf("✅ Round started!\n\n")

	// Step 2: Create key-value pairs
	fmt.Printf("📦 Creating %d key-value pairs (float64 values)...\n", *numPairs)
	pairs := make(map[string][]byte)
	testValues := make(map[string]float64)

	for i := 0; i < *numPairs; i++ {
		key := fmt.Sprintf("test-key-%03d", i)
		value := float64(i) * 0.12345
		testValues[key] = value
		pairs[key] = float64ToBytes(value)
	}
	fmt.Printf("✅ Created %d pairs\n\n", len(pairs))

	// Check if OKVS will be used
	useOKVS := *numPairs >= 100
	if useOKVS {
		fmt.Printf("ℹ️  OKVS encoding will be used (> 100 pairs)\n")
	} else {
		fmt.Printf("ℹ️  Direct PIR will be used (< 100 pairs)\n")
	}
	fmt.Println()

	// Step 3: Publish values
	fmt.Printf("📤 Publishing values...\n")
	startTime := time.Now()
	err = c.PublishValues(ctx, *roundID, "test-worker-1", pairs)
	if err != nil {
		log.Fatalf("Failed to publish values: %v", err)
	}
	duration := time.Since(startTime)
	fmt.Printf("✅ Published successfully in %v\n", duration)
	fmt.Println()

	// Wait a bit for server to process
	fmt.Printf("⏳ Waiting for server to process (2 seconds)...\n")
	time.Sleep(2 * time.Second)
	fmt.Println()

	// Step 4: Test PIR queries
	fmt.Printf("🔍 Testing PIR queries...\n")
	fmt.Println()

	// Query a few specific keys
	keysToQuery := []string{}
	if *testKey != "" {
		keysToQuery = append(keysToQuery, *testKey)
	} else {
		// Default: query first, middle, and last keys
		if *numPairs > 0 {
			keysToQuery = []string{
				fmt.Sprintf("test-key-%03d", 0),
				fmt.Sprintf("test-key-%03d", *numPairs/2),
				fmt.Sprintf("test-key-%03d", *numPairs-1),
			}
		} else {
			keysToQuery = []string{"test-key-000"}
		}
	}

	successCount := 0
	failCount := 0

	for _, key := range keysToQuery {
		expectedValue, exists := testValues[key]
		if !exists {
			fmt.Printf("❌ Key %s not found in test values\n", key)
			failCount++
			continue
		}

		fmt.Printf("  Querying key: %s (expected: %f)\n", key, expectedValue)

		queryStart := time.Now()
		retrievedBytes, err := c.GetValue(ctx, *roundID, key)
		queryDuration := time.Since(queryStart)

		if err != nil {
			fmt.Printf("    ❌ Error: %v\n", err)
			failCount++
			continue
		}

		if len(retrievedBytes) != 8 {
			fmt.Printf("    ❌ Invalid value length: expected 8 bytes, got %d\n", len(retrievedBytes))
			failCount++
			continue
		}

		retrievedValue := bytesToFloat64(retrievedBytes)

		// Use epsilon for floating point comparison
		epsilon := 1e-10
		diff := retrievedValue - expectedValue
		if diff < 0 {
			diff = -diff
		}

		if diff < epsilon || retrievedValue == expectedValue {
			fmt.Printf("    ✅ Retrieved: %f (matches expected) [%v]\n", retrievedValue, queryDuration)
			successCount++
		} else {
			fmt.Printf("    ❌ Mismatch: retrieved %f, expected %f (diff: %e)\n",
				retrievedValue, expectedValue, diff)
			failCount++
		}
	}

	fmt.Println()
	fmt.Printf("📊 Query Results: %d successful, %d failed\n", successCount, failCount)
	fmt.Println()

	// Summary
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📋 Summary\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Round ID:           %d\n", *roundID)
	fmt.Printf("  Number of pairs:   %d\n", *numPairs)
	fmt.Printf("  Encoding method:    ")
	if useOKVS {
		fmt.Printf("OKVS + PIR\n")
	} else {
		fmt.Printf("Direct PIR\n")
	}
	fmt.Printf("  Publish duration:  %v\n", duration)
	fmt.Printf("  Queries tested:    %d\n", len(keysToQuery))
	fmt.Printf("  Queries successful:%d\n", successCount)
	fmt.Printf("  Queries failed:    %d\n", failCount)

	if failCount == 0 {
		fmt.Printf("\n✅ All tests passed!\n")
		os.Exit(0)
	} else {
		fmt.Printf("\n❌ Some tests failed!\n")
		os.Exit(1)
	}
}

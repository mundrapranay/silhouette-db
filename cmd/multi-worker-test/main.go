package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	"github.com/mundrapranay/silhouette-db/pkg/client"
)

var (
	serverAddr     = flag.String("server", "127.0.0.1:9090", "Server address (host:port)")
	numWorkers     = flag.Int("workers", 10, "Number of concurrent workers")
	pairsPerWorker = flag.Int("pairs", 20, "Number of key-value pairs per worker")
	roundID        = flag.Uint64("round", 1, "Round ID")
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

	// Create a client for starting the round (just one needed)
	fmt.Printf("🔌 Connecting to server at %s...\n", *serverAddr)
	adminClient, err := client.NewClient(*serverAddr, nil)
	if err != nil {
		log.Fatalf("Failed to create admin client: %v", err)
	}
	defer adminClient.Close()
	fmt.Printf("✅ Connected successfully!\n\n")

	// Start round
	fmt.Printf("📋 Starting round %d with %d workers...\n", *roundID, *numWorkers)
	err = adminClient.StartRound(ctx, *roundID, int32(*numWorkers))
	if err != nil {
		log.Fatalf("Failed to start round: %v", err)
	}
	fmt.Printf("✅ Round started!\n\n")

	// Create worker clients
	fmt.Printf("📦 Creating %d worker clients...\n", *numWorkers)
	workers := make([]*client.Client, *numWorkers)
	for i := 0; i < *numWorkers; i++ {
		c, err := client.NewClient(*serverAddr, nil)
		if err != nil {
			log.Fatalf("Failed to create worker client %d: %v", i+1, err)
		}
		workers[i] = c
	}
	fmt.Printf("✅ All workers created!\n\n")

	// Track all published pairs for verification
	allPairs := make(map[string]float64)
	var pairsMutex sync.Mutex

	// Each worker publishes its pairs concurrently
	fmt.Printf("📤 Publishing from %d concurrent workers...\n", *numWorkers)
	fmt.Printf("   Each worker publishes %d pairs\n", *pairsPerWorker)
	fmt.Printf("   Total pairs expected: %d\n\n", *numWorkers**pairsPerWorker)

	startTime := time.Now()
	var wg sync.WaitGroup
	errors := make([]error, *numWorkers)

	for i := 0; i < *numWorkers; i++ {
		wg.Add(1)
		workerID := fmt.Sprintf("worker-%03d", i+1)
		workerClient := workers[i]

		go func(workerNum int, wID string, c *client.Client) {
			defer wg.Done()

			// Create pairs for this worker
			pairs := make(map[string][]byte)
			workerPairs := make(map[string]float64)

			for j := 0; j < *pairsPerWorker; j++ {
				key := fmt.Sprintf("worker-%03d-key-%03d", workerNum+1, j)
				value := float64(workerNum*1000+j) * 0.12345
				workerPairs[key] = value
				pairs[key] = float64ToBytes(value)
			}

			// Record for verification
			pairsMutex.Lock()
			for k, v := range workerPairs {
				allPairs[k] = v
			}
			pairsMutex.Unlock()

			// Publish values
			publishStart := time.Now()
			err := c.PublishValues(ctx, *roundID, wID, pairs)
			publishDuration := time.Since(publishStart)

			if err != nil {
				errors[workerNum] = fmt.Errorf("worker %s: %v", wID, err)
				log.Printf("❌ Worker %s failed: %v\n", wID, err)
			} else {
				fmt.Printf("   ✅ Worker %s published %d pairs in %v\n", wID, *pairsPerWorker, publishDuration)
			}
		}(i, workerID, workerClient)
	}

	// Wait for all workers to complete
	wg.Wait()
	totalDuration := time.Since(startTime)

	// Check for errors
	errorCount := 0
	for _, err := range errors {
		if err != nil {
			errorCount++
		}
	}

	fmt.Println()
	if errorCount > 0 {
		fmt.Printf("⚠️  %d worker(s) encountered errors\n", errorCount)
	}

	fmt.Printf("✅ All workers completed in %v\n", totalDuration)
	fmt.Printf("   Average time per worker: %v\n", totalDuration/time.Duration(*numWorkers))
	fmt.Println()

	// Check if round completed (need to wait a bit for server processing)
	fmt.Printf("⏳ Waiting for server to process (3 seconds)...\n")
	time.Sleep(3 * time.Second)
	fmt.Println()

	// Verify aggregation by querying some keys from different workers
	fmt.Printf("🔍 Verifying worker aggregation...\n")
	fmt.Println()

	// Test querying keys from different workers
	testKeys := []string{}
	testKeys = append(testKeys,
		fmt.Sprintf("worker-%03d-key-%03d", 1, 0),                           // First worker, first key
		fmt.Sprintf("worker-%03d-key-%03d", *numWorkers/2, 0),               // Middle worker
		fmt.Sprintf("worker-%03d-key-%03d", *numWorkers, *pairsPerWorker-1), // Last worker, last key
	)

	successCount := 0
	failCount := 0

	for _, key := range testKeys {
		expectedValue, exists := allPairs[key]
		if !exists {
			fmt.Printf("❌ Key %s not found in published pairs\n", key)
			failCount++
			continue
		}

		fmt.Printf("  Querying key: %s (expected: %f)\n", key, expectedValue)

		queryStart := time.Now()
		retrievedBytes, err := adminClient.GetValue(ctx, *roundID, key)
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

	// Close all worker clients
	for i, c := range workers {
		if err := c.Close(); err != nil {
			log.Printf("Failed to close worker client %d: %v", i+1, err)
		}
	}

	// Summary
	fmt.Println()
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📋 Summary\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Round ID:           %d\n", *roundID)
	fmt.Printf("  Number of workers:  %d\n", *numWorkers)
	fmt.Printf("  Pairs per worker:   %d\n", *pairsPerWorker)
	fmt.Printf("  Total pairs:        %d\n", len(allPairs))
	fmt.Printf("  Publish duration:  %v\n", totalDuration)
	fmt.Printf("  Avg per worker:    %v\n", totalDuration/time.Duration(*numWorkers))
	fmt.Printf("  Verification queries: %d\n", len(testKeys))
	fmt.Printf("  Queries successful: %d\n", successCount)
	fmt.Printf("  Queries failed:    %d\n", failCount)

	totalPairs := len(allPairs)
	useOKVS := totalPairs >= 100
	fmt.Printf("  Encoding method:    ")
	if useOKVS {
		fmt.Printf("OKVS + PIR (%d pairs >= 100)\n", totalPairs)
	} else {
		fmt.Printf("Direct PIR (%d pairs < 100)\n", totalPairs)
	}

	fmt.Println()

	if errorCount == 0 && failCount == 0 {
		fmt.Printf("✅ All tests passed!\n")
	} else {
		fmt.Printf("❌ Some tests failed (errors: %d, query failures: %d)\n", errorCount, failCount)
		log.Fatalf("Test failed")
	}
}

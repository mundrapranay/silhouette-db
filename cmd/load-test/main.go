package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/mundrapranay/silhouette-db/pkg/client"
)

var (
	serverAddr      = flag.String("server", "127.0.0.1:9090", "Server address (host:port)")
	numRounds       = flag.Int("rounds", 10, "Number of concurrent rounds")
	pairsPerRound   = flag.Int("pairs", 150, "Number of key-value pairs per round")
	workersPerRound = flag.Int("workers", 5, "Number of workers per round")
	queriesPerSec   = flag.Float64("qps", 10.0, "Queries per second (PIR queries)")
	duration        = flag.Duration("duration", 30*time.Second, "Test duration")
)

// float64ToBytes converts a float64 to 8-byte little-endian bytes
func float64ToBytes(f float64) []byte {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, *(*uint64)(unsafe.Pointer(&f)))
	return buf
}

func main() {
	flag.Parse()

	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("🚀 silhouette-db Load Testing\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Println()
	fmt.Printf("📋 Configuration:\n")
	fmt.Printf("   Server:           %s\n", *serverAddr)
	fmt.Printf("   Concurrent rounds: %d\n", *numRounds)
	fmt.Printf("   Pairs per round:  %d\n", *pairsPerRound)
	fmt.Printf("   Workers per round:%d\n", *workersPerRound)
	fmt.Printf("   Queries per sec:  %.1f\n", *queriesPerSec)
	fmt.Printf("   Test duration:    %v\n", *duration)
	fmt.Println()

	ctx := context.Background()

	// Create admin client
	fmt.Printf("🔌 Connecting to server...\n")
	adminClient, err := client.NewClient(*serverAddr, nil)
	if err != nil {
		log.Fatalf("Failed to create admin client: %v", err)
	}
	defer adminClient.Close()
	fmt.Printf("✅ Connected successfully!\n\n")

	// Track metrics
	var (
		roundsCompleted  int64
		roundsFailed     int64
		queriesCompleted int64
		queriesFailed    int64
		totalPublishTime int64 // nanoseconds
		totalQueryTime   int64 // nanoseconds
	)

	// Test rounds
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📋 Phase 1: Concurrent Rounds Test\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Println()

	roundsStart := time.Now()
	var roundsWg sync.WaitGroup

	for roundNum := 0; roundNum < *numRounds; roundNum++ {
		roundsWg.Add(1)
		go func(rID uint64) {
			defer roundsWg.Done()

			roundStart := time.Now()

			// Create worker clients for this round
			workers := make([]*client.Client, *workersPerRound)
			for i := 0; i < *workersPerRound; i++ {
				c, err := client.NewClient(*serverAddr, nil)
				if err != nil {
					log.Printf("Failed to create worker client for round %d: %v", rID, err)
					atomic.AddInt64(&roundsFailed, 1)
					return
				}
				workers[i] = c
			}

			// Start round
			err := adminClient.StartRound(ctx, rID, int32(*workersPerRound))
			if err != nil {
				log.Printf("Failed to start round %d: %v", rID, err)
				atomic.AddInt64(&roundsFailed, 1)
				return
			}

			// Calculate pairs per worker
			pairsPerWorker := *pairsPerRound / *workersPerRound
			if pairsPerWorker == 0 {
				pairsPerWorker = 1
			}

			// Workers publish concurrently
			var workersWg sync.WaitGroup
			var workerErrors int64

			for i := 0; i < *workersPerRound; i++ {
				workersWg.Add(1)
				go func(workerNum int, wClient *client.Client) {
					defer workersWg.Done()

					workerID := fmt.Sprintf("round-%d-worker-%d", rID, workerNum+1)
					pairs := make(map[string][]byte)

					startIdx := workerNum * pairsPerWorker
					for j := 0; j < pairsPerWorker && startIdx+j < *pairsPerRound; j++ {
						key := fmt.Sprintf("round-%d-key-%04d", rID, startIdx+j)
						value := float64(startIdx+j) * 0.12345
						pairs[key] = float64ToBytes(value)
					}

					err := wClient.PublishValues(ctx, rID, workerID, pairs)
					if err != nil {
						atomic.AddInt64(&workerErrors, 1)
						log.Printf("Worker %s failed: %v", workerID, err)
					}
				}(i, workers[i])
			}

			workersWg.Wait()

			// Close worker clients
			for _, c := range workers {
				c.Close()
			}

			if atomic.LoadInt64(&workerErrors) > 0 {
				atomic.AddInt64(&roundsFailed, 1)
				return
			}

			roundDuration := time.Since(roundStart)
			atomic.AddInt64(&totalPublishTime, roundDuration.Nanoseconds())
			atomic.AddInt64(&roundsCompleted, 1)

			fmt.Printf("   ✅ Round %d completed in %v\n", rID, roundDuration)
		}(uint64(roundNum + 1))
	}

	roundsWg.Wait()
	roundsDuration := time.Since(roundsStart)

	fmt.Println()
	fmt.Printf("📊 Round Results:\n")
	fmt.Printf("   Completed: %d\n", atomic.LoadInt64(&roundsCompleted))
	fmt.Printf("   Failed:    %d\n", atomic.LoadInt64(&roundsFailed))
	fmt.Printf("   Duration:  %v\n", roundsDuration)
	if atomic.LoadInt64(&roundsCompleted) > 0 {
		avgTime := time.Duration(atomic.LoadInt64(&totalPublishTime) / atomic.LoadInt64(&roundsCompleted))
		fmt.Printf("   Avg time:  %v\n", avgTime)
	}
	fmt.Println()

	// Wait for all rounds to be processed
	fmt.Printf("⏳ Waiting for server to process all rounds (3 seconds)...\n")
	time.Sleep(3 * time.Second)
	fmt.Println()

	// PIR Query Load Test
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📋 Phase 2: PIR Query Load Test\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Println()

	queryInterval := time.Duration(float64(time.Second) / *queriesPerSec)
	fmt.Printf("🔍 Running PIR queries at %.1f QPS for %v...\n", *queriesPerSec, *duration)
	fmt.Printf("   Query interval: %v\n", queryInterval)
	fmt.Println()

	testStart := time.Now()
	stopTime := testStart.Add(*duration)
	ticker := time.NewTicker(queryInterval)
	defer ticker.Stop()

	// Progress update ticker (every 5 seconds)
	progressTicker := time.NewTicker(5 * time.Second)
	defer progressTicker.Stop()

	// Start query goroutine
	done := make(chan bool, 1)
	go func() {
		for {
			select {
			case <-ticker.C:
				if time.Now().After(stopTime) {
					return
				}

				// Randomly pick a round and key to query
				// Use nanosecond timestamp for better randomness since Unix() only changes per second
				now := time.Now()
				roundID := uint64((now.Unix()*1000000000+int64(now.Nanosecond()))%int64(*numRounds) + 1)
				keyIndex := int((now.Unix()*1000000000 + int64(now.Nanosecond())) % int64(*pairsPerRound))
				key := fmt.Sprintf("round-%d-key-%04d", roundID, keyIndex)

				go func(rID uint64, k string) {
					queryStart := time.Now()

					// Create a context with timeout for this query
					queryCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()

					_, err := adminClient.GetValue(queryCtx, rID, k)
					queryDuration := time.Since(queryStart)

					if err != nil {
						atomic.AddInt64(&queriesFailed, 1)
						// Log first few errors and periodically after that
						failed := atomic.LoadInt64(&queriesFailed)
						if failed <= 5 || failed%100 == 0 {
							fmt.Printf("   ❌ Query failed: round %d, key %s [%v] - Error: %v\n", rID, k, queryDuration, err)
						}
					} else {
						atomic.AddInt64(&queriesCompleted, 1)
						atomic.AddInt64(&totalQueryTime, queryDuration.Nanoseconds())
					}

					total := atomic.LoadInt64(&queriesCompleted) + atomic.LoadInt64(&queriesFailed)
					if total < 10 {
						fmt.Printf("   Query: round %d, key %s [%v]\n", rID, k, queryDuration)
					}
				}(roundID, key)
			case <-done:
				return
			}
		}
	}()

	// Progress reporting goroutine
	progressDone := make(chan bool, 1)
	go func() {
		for {
			select {
			case <-progressTicker.C:
				if time.Now().After(stopTime) {
					return
				}
				elapsed := time.Since(testStart)
				remaining := *duration - elapsed
				if remaining < 0 {
					remaining = 0
				}
				completed := atomic.LoadInt64(&queriesCompleted)
				failed := atomic.LoadInt64(&queriesFailed)
				total := completed + failed
				successRate := 0.0
				if total > 0 {
					successRate = float64(completed) * 100.0 / float64(total)
				}
				fmt.Printf("   ⏱️  Progress: %v elapsed, %v remaining | Queries: %d total (%d completed, %d failed, %.1f%% success)\n",
					elapsed.Round(time.Second), remaining.Round(time.Second), total, completed, failed, successRate)
			case <-progressDone:
				return
			}
		}
	}()

	// Wait for test duration
	time.Sleep(*duration)
	testDuration := time.Since(testStart)

	// Signal done and wait a bit for in-flight queries
	done <- true
	progressDone <- true
	time.Sleep(2 * time.Second)

	fmt.Println()
	fmt.Printf("📊 Query Results:\n")
	totalQueries := atomic.LoadInt64(&queriesCompleted) + atomic.LoadInt64(&queriesFailed)
	fmt.Printf("   Completed:     %d\n", atomic.LoadInt64(&queriesCompleted))
	fmt.Printf("   Failed:        %d\n", atomic.LoadInt64(&queriesFailed))
	fmt.Printf("   Total:         %d\n", totalQueries)
	fmt.Printf("   Duration:      %v\n", testDuration)
	if totalQueries > 0 {
		actualQPS := float64(totalQueries) / testDuration.Seconds()
		fmt.Printf("   Actual QPS:    %.2f\n", actualQPS)
	}
	if atomic.LoadInt64(&queriesCompleted) > 0 {
		avgQueryTime := time.Duration(atomic.LoadInt64(&totalQueryTime) / atomic.LoadInt64(&queriesCompleted))
		fmt.Printf("   Avg query time: %v\n", avgQueryTime)
	}
	fmt.Println()

	// Summary
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("📋 Load Test Summary\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Concurrent rounds:    %d\n", *numRounds)
	fmt.Printf("  Rounds completed:    %d\n", atomic.LoadInt64(&roundsCompleted))
	fmt.Printf("  Rounds failed:       %d\n", atomic.LoadInt64(&roundsFailed))
	fmt.Printf("  Total queries:       %d\n", totalQueries)
	fmt.Printf("  Queries succeeded:   %d\n", atomic.LoadInt64(&queriesCompleted))
	fmt.Printf("  Queries failed:      %d\n", atomic.LoadInt64(&queriesFailed))
	if totalQueries > 0 {
		fmt.Printf("  Query success rate: %.2f%%\n", float64(atomic.LoadInt64(&queriesCompleted))*100.0/float64(totalQueries))
	}
	fmt.Println()

	if atomic.LoadInt64(&roundsFailed) == 0 && atomic.LoadInt64(&queriesFailed) == 0 {
		fmt.Printf("✅ Load test passed!\n")
	} else {
		fmt.Printf("⚠️  Load test completed with some failures\n")
	}
}

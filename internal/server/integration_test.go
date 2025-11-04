package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"github.com/mundrapranay/silhouette-db/internal/crypto"
	"github.com/mundrapranay/silhouette-db/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// setupTestServerWithGRPC sets up a complete server with gRPC endpoint
// backend can be "okvs" or "kvs" (defaults to "okvs")
func setupTestServerWithGRPC(t *testing.T, backend string) (*grpc.Server, *Server, *store.Store, string) {
	if backend == "" {
		backend = "okvs" // Default to OKVS
	}

	tmpDir := t.TempDir()

	config := store.Config{
		NodeID:           "test-node",
		ListenAddr:       "127.0.0.1:0",
		DataDir:          tmpDir,
		Bootstrap:        true,
		HeartbeatTimeout: 500 * time.Millisecond,
		ElectionTimeout:  500 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	s, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	// Wait for leadership
	timeout := time.After(5 * time.Second)
	tick := time.Tick(100 * time.Millisecond)
	for !s.IsLeader() {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for leadership")
		case <-tick:
		}
	}

	var encoder crypto.OKVSEncoder
	if backend == "kvs" {
		encoder = crypto.NewKVSEncoder()
	} else {
		// For OKVS backend, use real RB-OKVS encoder (requires cgo)
		encoder = crypto.NewRBOKVSEncoder()
	}
	server := NewServer(s, encoder, backend)

	// Start gRPC server on random port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	grpcSrv := grpc.NewServer()
	apiv1.RegisterCoordinationServiceServer(grpcSrv, server)

	// Start server in background
	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			t.Logf("gRPC server error: %v", err)
		}
	}()

	addr := lis.Addr().String()
	return grpcSrv, server, s, addr
}

// createTestClient creates a gRPC client connected to the test server
func createTestClient(t *testing.T, addr string) apiv1.CoordinationServiceClient {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	return apiv1.NewCoordinationServiceClient(conn)
}

// TestRoundLifecycle_EndToEnd tests the complete round lifecycle with both backends
// For <100 pairs: uses KVS backend
// For >=100 pairs: tests both OKVS and KVS backends
func TestRoundLifecycle_EndToEnd(t *testing.T) {
	// Test with KVS backend (<100 pairs)
	t.Run("KVS_Backend", func(t *testing.T) {
		grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "kvs")
		defer grpcSrv.Stop()
		defer store.Shutdown()

		client := createTestClient(t, addr)
		ctx := context.Background()

		roundID := uint64(1)
		expectedWorkers := int32(3)

		startResp, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
			RoundId:         roundID,
			ExpectedWorkers: expectedWorkers,
		})
		if err != nil {
			t.Fatalf("StartRound failed: %v", err)
		}
		if !startResp.Success {
			t.Fatal("StartRound should return success=true")
		}

		workers := []string{"worker1", "worker2", "worker3"}
		for _, workerID := range workers {
			publishResp, err := client.PublishValues(ctx, &apiv1.PublishValuesRequest{
				RoundId:  roundID,
				WorkerId: workerID,
				Pairs: []*apiv1.KeyValuePair{
					{Key: "test-" + workerID + "-1", Value: []byte("value-" + workerID + "-1")},
					{Key: "test-" + workerID + "-2", Value: []byte("value-" + workerID + "-2")},
				},
			})
			if err != nil {
				t.Fatalf("PublishValues failed for %s: %v", workerID, err)
			}
			if !publishResp.Success {
				t.Fatalf("PublishValues should return success=true for %s", workerID)
			}
		}

		time.Sleep(500 * time.Millisecond)

		roundKey := "round_1_results"
		blob, exists := store.Get(roundKey)
		if !exists {
			t.Fatal("Round data should be stored after all workers publish")
		}
		if len(blob) == 0 {
			t.Fatal("Storage blob should not be empty")
		}

		// Note: GetValue requires proper PIR query setup, which is tested in PIR integration tests
		// For this lifecycle test, we just verify the round was stored
		t.Logf("✅ KVS backend: RoundID=%d, Workers=%d, BlobSize=%d bytes", roundID, len(workers), len(blob))
	})

	// Test with OKVS backend (>=100 pairs)
	if testing.Short() {
		t.Skip("Skipping OKVS test in short mode")
	}
	t.Run("OKVS_Backend_100Plus", func(t *testing.T) {
		grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "okvs")
		defer grpcSrv.Stop()
		defer store.Shutdown()

		client := createTestClient(t, addr)
		ctx := context.Background()

		roundID := uint64(2)
		expectedWorkers := int32(2)

		startResp, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
			RoundId:         roundID,
			ExpectedWorkers: expectedWorkers,
		})
		if err != nil {
			t.Fatalf("StartRound failed: %v", err)
		}
		if !startResp.Success {
			t.Fatal("StartRound should return success=true")
		}

		// Create 100+ pairs (50 per worker, total 100)
		workers := []string{"worker1", "worker2"}
		for i, workerID := range workers {
			pairs := make([]*apiv1.KeyValuePair, 50)
			for j := 0; j < 50; j++ {
				// Create 8-byte values for OKVS
				valueBytes := make([]byte, 8)
				valueBytes[0] = byte(i*50 + j + 1)
				pairs[j] = &apiv1.KeyValuePair{
					Key:   fmt.Sprintf("test-%s-%d", workerID, j+1),
					Value: valueBytes,
				}
			}

			publishResp, err := client.PublishValues(ctx, &apiv1.PublishValuesRequest{
				RoundId:  roundID,
				WorkerId: workerID,
				Pairs:    pairs,
			})
			if err != nil {
				t.Fatalf("PublishValues failed for %s: %v", workerID, err)
			}
			if !publishResp.Success {
				t.Fatalf("PublishValues should return success=true for %s", workerID)
			}
		}

		time.Sleep(500 * time.Millisecond)

		roundKey := "round_2_results"
		blob, exists := store.Get(roundKey)
		if !exists {
			t.Fatal("Round data should be stored after all workers publish")
		}
		if len(blob) == 0 {
			t.Fatal("OKVS blob should not be empty")
		}

		t.Logf("✅ OKVS backend: RoundID=%d, Workers=%d, OKVSSize=%d bytes", roundID, len(workers), len(blob))
	})
}

func TestRoundLifecycle_MultipleRounds(t *testing.T) {
	// Test with KVS backend (<100 pairs)
	t.Run("KVS_Backend", func(t *testing.T) {
		grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "kvs")
		defer grpcSrv.Stop()
		defer store.Shutdown()

		client := createTestClient(t, addr)
		ctx := context.Background()

		numRounds := 3
		for roundID := uint64(1); roundID <= uint64(numRounds); roundID++ {
			_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
				RoundId:         roundID,
				ExpectedWorkers: 1,
			})
			if err != nil {
				t.Fatalf("StartRound failed for round %d: %v", roundID, err)
			}

			_, err = client.PublishValues(ctx, &apiv1.PublishValuesRequest{
				RoundId:  roundID,
				WorkerId: "worker1",
				Pairs: []*apiv1.KeyValuePair{
					{Key: "round-key", Value: []byte("round-value")},
				},
			})
			if err != nil {
				t.Fatalf("PublishValues failed for round %d: %v", roundID, err)
			}

			time.Sleep(200 * time.Millisecond)

			roundKey := fmt.Sprintf("round_%d_results", roundID)
			_, exists := store.Get(roundKey)
			if !exists {
				t.Fatalf("Round %d data should be stored", roundID)
			}
		}

		t.Logf("✅ KVS backend: Completed %d sequential rounds successfully", numRounds)
	})
}

func TestRoundLifecycle_ConcurrentWorkers(t *testing.T) {
	// Test with KVS backend (<100 pairs)
	t.Run("KVS_Backend", func(t *testing.T) {
		grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "kvs")
		defer grpcSrv.Stop()
		defer store.Shutdown()

		client := createTestClient(t, addr)
		ctx := context.Background()

		roundID := uint64(1)
		_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
			RoundId:         roundID,
			ExpectedWorkers: 5,
		})
		if err != nil {
			t.Fatalf("StartRound failed: %v", err)
		}

		done := make(chan error, 5)
		for i := 0; i < 5; i++ {
			go func(workerNum int) {
				_, err := client.PublishValues(ctx, &apiv1.PublishValuesRequest{
					RoundId:  roundID,
					WorkerId: fmt.Sprintf("worker%d", workerNum+1),
					Pairs: []*apiv1.KeyValuePair{
						{Key: "concurrent-key", Value: []byte("concurrent-value")},
					},
				})
				done <- err
			}(i)
		}

		for i := 0; i < 5; i++ {
			if err := <-done; err != nil {
				t.Fatalf("Concurrent publish failed: %v", err)
			}
		}

		time.Sleep(500 * time.Millisecond)

		roundKey := "round_1_results"
		_, exists := store.Get(roundKey)
		if !exists {
			t.Fatal("Round data should be stored after concurrent publishes")
		}

		t.Log("✅ KVS backend: Concurrent workers published successfully")
	})
}

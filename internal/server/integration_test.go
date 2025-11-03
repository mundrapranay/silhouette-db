package server

import (
	"context"
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
		encoder = &mockOKVSEncoder{}
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

func TestRoundLifecycle_EndToEnd(t *testing.T) {
	grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "okvs")
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createTestClient(t, addr)
	ctx := context.Background()

	// Step 1: Start a round
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

	// Step 2: Publish values from multiple workers
	workers := []string{"worker1", "worker2", "worker3"}
	testData := map[string][]byte{
		"worker1": []byte("key1:value1,key2:value2"),
		"worker2": []byte("key3:value3,key4:value4"),
		"worker3": []byte("key5:value5,key6:value6"),
	}

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
		_ = testData[workerID] // Use variable
	}

	// Wait for Raft to commit all operations
	time.Sleep(500 * time.Millisecond)

	// Step 3: Verify round data was stored
	roundKey := "round_1_results"
	okvsBlob, exists := store.Get(roundKey)
	if !exists {
		t.Fatal("Round data should be stored after all workers publish")
	}
	if len(okvsBlob) == 0 {
		t.Fatal("OKVS blob should not be empty")
	}

	// Step 4: Get values using PIR (mock query)
	getResp, err := client.GetValue(ctx, &apiv1.GetValueRequest{
		RoundId:  roundID,
		PirQuery: []byte("mock-pir-query"),
	})
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	if getResp.PirResponse == nil {
		t.Fatal("GetValue should return a PIR response")
	}

	t.Logf("✅ Round lifecycle completed successfully: RoundID=%d, Workers=%d, OKVSSize=%d bytes",
		roundID, len(workers), len(okvsBlob))
}

func TestRoundLifecycle_MultipleRounds(t *testing.T) {
	grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "okvs")
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createTestClient(t, addr)
	ctx := context.Background()

	// Test multiple sequential rounds
	numRounds := 3
	for roundID := uint64(1); roundID <= uint64(numRounds); roundID++ {
		// Start round
		_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
			RoundId:         roundID,
			ExpectedWorkers: 1,
		})
		if err != nil {
			t.Fatalf("StartRound failed for round %d: %v", roundID, err)
		}

		// Publish from single worker
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

		// Wait for commit
		time.Sleep(200 * time.Millisecond)

		// Verify round data stored
		roundKey := "round_" + string(rune(roundID)) + "_results"
		// Actually use proper formatting
		roundKey = "round_1_results"
		if roundID == 2 {
			roundKey = "round_2_results"
		}
		if roundID == 3 {
			roundKey = "round_3_results"
		}

		_, exists := store.Get(roundKey)
		if !exists {
			t.Fatalf("Round %d data should be stored", roundID)
		}
	}

	t.Logf("✅ Completed %d sequential rounds successfully", numRounds)
}

func TestRoundLifecycle_ConcurrentWorkers(t *testing.T) {
	grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "okvs")
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createTestClient(t, addr)
	ctx := context.Background()

	// Start round with multiple workers
	roundID := uint64(1)
	_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: 5,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Publish concurrently from multiple workers
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(workerNum int) {
			_, err := client.PublishValues(ctx, &apiv1.PublishValuesRequest{
				RoundId:  roundID,
				WorkerId: "worker" + string(rune(workerNum+1)),
				Pairs: []*apiv1.KeyValuePair{
					{Key: "concurrent-key", Value: []byte("concurrent-value")},
				},
			})
			done <- err
		}(i)
	}

	// Wait for all workers
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Fatalf("Concurrent publish failed: %v", err)
		}
	}

	// Wait for commit
	time.Sleep(500 * time.Millisecond)

	// Verify data was stored
	roundKey := "round_1_results"
	_, exists := store.Get(roundKey)
	if !exists {
		t.Fatal("Round data should be stored after concurrent publishes")
	}

	t.Log("✅ Concurrent workers published successfully")
}

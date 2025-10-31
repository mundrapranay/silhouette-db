package server

import (
	"context"
	"net"
	"testing"
	"time"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"github.com/mundrapranay/silhouette-db/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// setupBenchServer sets up a server for benchmarks
func setupBenchServer(b *testing.B) (*grpc.Server, *Server, *store.Store, string) {
	tmpDir := b.TempDir()

	config := store.Config{
		NodeID:           "bench-node",
		ListenAddr:       "127.0.0.1:0",
		DataDir:          tmpDir,
		Bootstrap:        true,
		HeartbeatTimeout: 500 * time.Millisecond,
		ElectionTimeout:  500 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	s, err := store.NewStore(config)
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}

	// Wait for leadership
	timeout := time.After(5 * time.Second)
	tick := time.Tick(100 * time.Millisecond)
	for !s.IsLeader() {
		select {
		case <-timeout:
			b.Fatal("Timeout waiting for leadership")
		case <-tick:
		}
	}

	okvsEncoder := &mockOKVSEncoder{}
	pirServer := &mockPIRServer{}
	server := NewServer(s, okvsEncoder, pirServer)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("Failed to listen: %v", err)
	}

	grpcSrv := grpc.NewServer()
	apiv1.RegisterCoordinationServiceServer(grpcSrv, server)

	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			b.Logf("gRPC server error: %v", err)
		}
	}()

	addr := lis.Addr().String()
	return grpcSrv, server, s, addr
}

// createBenchClient creates a gRPC client for benchmarks
func createBenchClient(b *testing.B, addr string) apiv1.CoordinationServiceClient {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	return apiv1.NewCoordinationServiceClient(conn)
}

func BenchmarkStartRound(b *testing.B) {
	grpcSrv, _, store, addr := setupBenchServer(b)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createBenchClient(b, addr)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
			RoundId:         uint64(i + 1),
			ExpectedWorkers: 1,
		})
		if err != nil {
			b.Fatalf("StartRound failed: %v", err)
		}
	}
}

func BenchmarkPublishValues(b *testing.B) {
	grpcSrv, _, store, addr := setupBenchServer(b)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createBenchClient(b, addr)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique round ID for each iteration
		roundID := uint64(i + 1)

		// Start a new round for each iteration
		_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
			RoundId:         roundID,
			ExpectedWorkers: 1,
		})
		if err != nil {
			b.Fatalf("StartRound failed: %v", err)
		}

		// Publish values
		_, err = client.PublishValues(ctx, &apiv1.PublishValuesRequest{
			RoundId:  roundID,
			WorkerId: "worker1",
			Pairs: []*apiv1.KeyValuePair{
				{Key: "bench-key", Value: []byte("bench-value")},
			},
		})
		if err != nil {
			b.Fatalf("PublishValues failed: %v", err)
		}
	}
}

func BenchmarkPublishValues_ManyPairs(b *testing.B) {
	grpcSrv, _, store, addr := setupBenchServer(b)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createBenchClient(b, addr)
	ctx := context.Background()

	// Create 100 pairs (reused across iterations)
	pairs := make([]*apiv1.KeyValuePair, 100)
	for i := 0; i < 100; i++ {
		pairs[i] = &apiv1.KeyValuePair{
			Key:   "key-" + string(rune(i)),
			Value: []byte("value-" + string(rune(i))),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use unique round ID for each iteration
		roundID := uint64(i + 1)

		// Start a new round for each iteration
		_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
			RoundId:         roundID,
			ExpectedWorkers: 1,
		})
		if err != nil {
			b.Fatalf("StartRound failed: %v", err)
		}

		// Publish values
		_, err = client.PublishValues(ctx, &apiv1.PublishValuesRequest{
			RoundId:  roundID,
			WorkerId: "worker1",
			Pairs:    pairs,
		})
		if err != nil {
			b.Fatalf("PublishValues failed: %v", err)
		}
	}
}

func BenchmarkGetValue(b *testing.B) {
	grpcSrv, _, store, addr := setupBenchServer(b)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createBenchClient(b, addr)
	ctx := context.Background()

	// Setup: Start round and publish
	roundID := uint64(1)
	_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: 1,
	})
	if err != nil {
		b.Fatalf("StartRound failed: %v", err)
	}

	_, err = client.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "test-key", Value: []byte("test-value")},
		},
	})
	if err != nil {
		b.Fatalf("PublishValues failed: %v", err)
	}

	// Wait for commit
	time.Sleep(200 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.GetValue(ctx, &apiv1.GetValueRequest{
			RoundId:  roundID,
			PirQuery: []byte("bench-query"),
		})
		if err != nil {
			b.Fatalf("GetValue failed: %v", err)
		}
	}
}

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

// setupBenchServer sets up a server for benchmarks
// backend can be "okvs" or "kvs" (defaults to "kvs")
func setupBenchServer(b *testing.B, backend string) (*grpc.Server, *Server, *store.Store, string) {
	if backend == "" {
		backend = "kvs" // Default to KVS for benchmarks
	}

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

	var encoder crypto.OKVSEncoder
	if backend == "kvs" {
		encoder = crypto.NewKVSEncoder()
	} else {
		encoder = crypto.NewRBOKVSEncoder()
	}
	server := NewServer(s, encoder, backend)

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
	grpcSrv, _, store, addr := setupBenchServer(b, "kvs")
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
	// Use KVS backend for <100 pairs
	grpcSrv, _, store, addr := setupBenchServer(b, "kvs")
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
	// Use OKVS backend for >=100 pairs
	grpcSrv, _, store, addr := setupBenchServer(b, "okvs")
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createBenchClient(b, addr)
	ctx := context.Background()

	// Create 100 pairs with 8-byte values (OKVS requires 8-byte float64 values)
	pairs := make([]*apiv1.KeyValuePair, 100)
	for i := 0; i < 100; i++ {
		// Create 8-byte value (float64 representation)
		valueBytes := make([]byte, 8)
		valueBytes[0] = byte(i)
		valueBytes[1] = byte(i >> 8)
		valueBytes[2] = byte(i >> 16)
		valueBytes[3] = byte(i >> 24)
		// Fill remaining bytes
		valueBytes[4] = 0
		valueBytes[5] = 0
		valueBytes[6] = 0
		valueBytes[7] = 0

		pairs[i] = &apiv1.KeyValuePair{
			Key:   fmt.Sprintf("key-%d", i),
			Value: valueBytes,
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
	// Use KVS backend for <100 pairs
	grpcSrv, server, store, addr := setupBenchServer(b, "kvs")
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createBenchClient(b, addr)
	ctx := context.Background()

	// Setup: Start round and publish
	roundID := uint64(1)
	_, err := server.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: 1,
	})
	if err != nil {
		b.Fatalf("StartRound failed: %v", err)
	}

	_, err = server.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "test-key", Value: []byte("test-value")},
		},
	})
	if err != nil {
		b.Fatalf("PublishValues failed: %v", err)
	}

	// Wait for commit and PIR server creation
	time.Sleep(500 * time.Millisecond)

	// Get BaseParams and key mapping to create a proper PIR client
	baseParamsReq := &apiv1.GetBaseParamsRequest{RoundId: roundID}
	baseParamsResp, err := server.GetBaseParams(ctx, baseParamsReq)
	if err != nil {
		b.Fatalf("GetBaseParams failed: %v", err)
	}

	keyMappingReq := &apiv1.GetKeyMappingRequest{RoundId: roundID}
	keyMappingResp, err := server.GetKeyMapping(ctx, keyMappingReq)
	if err != nil {
		b.Fatalf("GetKeyMapping failed: %v", err)
	}

	// Convert protobuf entries to map
	keyToIndex := make(map[string]int)
	for _, entry := range keyMappingResp.Entries {
		keyToIndex[entry.Key] = int(entry.Index)
	}

	// Create PIR client and generate a real query
	pirClient, err := crypto.NewFrodoPIRClient(baseParamsResp.BaseParams, keyToIndex)
	if err != nil {
		b.Fatalf("Failed to create PIR client: %v", err)
	}
	defer pirClient.Close()

	// Generate a real PIR query for "test-key"
	query, err := pirClient.GenerateQuery("test-key")
	if err != nil {
		b.Fatalf("Failed to generate PIR query: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use the real PIR query
		_, err := client.GetValue(ctx, &apiv1.GetValueRequest{
			RoundId:  roundID,
			PirQuery: query,
		})
		if err != nil {
			b.Fatalf("GetValue failed: %v", err)
		}
	}
}

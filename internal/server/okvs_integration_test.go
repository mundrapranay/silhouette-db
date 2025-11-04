//go:build cgo
// +build cgo

package server

import (
	"context"
	"net"
	"testing"
	"time"
	"unsafe"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"github.com/mundrapranay/silhouette-db/internal/crypto"
	"github.com/mundrapranay/silhouette-db/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// setupTestServerWithRBOKVS sets up a complete server with gRPC endpoint
// using RB-OKVS encoder (requires cgo)
func setupTestServerWithRBOKVS(t *testing.T) (*grpc.Server, *Server, *store.Store, string) {
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

	// Use RB-OKVS encoder (requires cgo)
	okvsEncoder := crypto.NewRBOKVSEncoder()
	server := NewServer(s, okvsEncoder, "okvs")

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

// TestRBOKVSIntegration_MinimumPairs tests OKVS encoding with minimum required pairs (100)
func TestRBOKVSIntegration_MinimumPairs(t *testing.T) {
	grpcSrv, _, store, _ := setupTestServerWithRBOKVS(t)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	ctx := context.Background()
	roundID := uint64(1)
	expectedWorkers := int32(1)

	// Start round
	startReq := &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: expectedWorkers,
	}

	server := NewServer(store, crypto.NewRBOKVSEncoder(), "okvs")
	_, err := server.StartRound(ctx, startReq)
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Create 100 pairs (minimum required)
	pairs := make([]*apiv1.KeyValuePair, 100)
	for i := 0; i < 100; i++ {
		key := "key" + string(rune('0'+(i%10))) + string(rune('0'+(i/10)))
		valueF64 := float64(i) * 0.123
		valueBytes := float64ToBytes(valueF64)
		pairs[i] = &apiv1.KeyValuePair{
			Key:   key,
			Value: valueBytes,
		}
	}

	// Publish values
	publishReq := &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs:    pairs,
	}

	_, err = server.PublishValues(ctx, publishReq)
	if err != nil {
		t.Fatalf("PublishValues failed: %v", err)
	}

	// Wait for Raft to commit
	time.Sleep(500 * time.Millisecond)

	t.Logf("✅ Successfully encoded %d pairs using RB-OKVS", len(pairs))
}

// TestRBOKVSIntegration_FullRound tests complete round lifecycle with RB-OKVS
func TestRBOKVSIntegration_FullRound(t *testing.T) {
	grpcSrv, server, store, _ := setupTestServerWithRBOKVS(t)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	ctx := context.Background()
	roundID := uint64(100)
	expectedWorkers := int32(1)

	// Step 1: Start round
	_, err := server.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: expectedWorkers,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Step 2: Create 150 pairs (above minimum, with some test data)
	testPairs := make([]*apiv1.KeyValuePair, 150)
	testValues := make(map[string]float64)

	for i := 0; i < 150; i++ {
		key := "test-key-" + string(rune('0'+(i%10))) + string(rune('0'+(i/10)))
		valueF64 := float64(i) * 0.456
		testValues[key] = valueF64
		valueBytes := float64ToBytes(valueF64)

		testPairs[i] = &apiv1.KeyValuePair{
			Key:   key,
			Value: valueBytes,
		}
	}

	// Step 3: Publish values
	_, err = server.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "test-worker",
		Pairs:    testPairs,
	})
	if err != nil {
		t.Fatalf("PublishValues failed: %v", err)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	// Step 4: Verify OKVS blob was stored
	roundKey := "round_100_results"
	okvsBlob, exists := store.Get(roundKey)
	if !exists {
		t.Fatal("OKVS blob should be stored in Raft")
	}
	if len(okvsBlob) == 0 {
		t.Fatal("OKVS blob should not be empty")
	}

	t.Logf("✅ OKVS blob stored: %d bytes", len(okvsBlob))

	// Step 5: Verify we can decode values from OKVS
	decoder := crypto.NewRBOKVSDecoder(okvsBlob)

	// Test decoding a few keys
	testKeys := []string{"test-key-00", "test-key-50", "test-key-99"}
	for _, key := range testKeys {
		expectedValue, ok := testValues[key]
		if !ok {
			t.Fatalf("Test key %s not found in test values", key)
		}

		decodedBytes, err := decoder.Decode(okvsBlob, key)
		if err != nil {
			t.Fatalf("Decode failed for key %s: %v", key, err)
		}

		if len(decodedBytes) != 8 {
			t.Fatalf("Decoded value should be 8 bytes, got %d for key %s", len(decodedBytes), key)
		}

		decodedValue := *(*float64)(unsafe.Pointer(&decodedBytes[0]))

		// Use approximate equality for floating point comparison
		epsilon := 1e-10
		diff := decodedValue - expectedValue
		if diff < 0 {
			diff = -diff
		}
		if diff > epsilon && decodedValue != expectedValue {
			t.Errorf("Decoded value mismatch for key %s: expected %f, got %f (diff: %e)",
				key, expectedValue, decodedValue, diff)
		} else {
			t.Logf("✅ Decoded key %s: %f (matches expected)", key, decodedValue)
		}
	}
}

// TestRBOKVSIntegration_TooFewPairs tests that OKVS backend rejects fewer than 100 pairs
func TestRBOKVSIntegration_TooFewPairs(t *testing.T) {
	grpcSrv, server, store, _ := setupTestServerWithRBOKVS(t)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	ctx := context.Background()
	roundID := uint64(200)
	expectedWorkers := int32(1)

	// Start round
	_, err := server.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: expectedWorkers,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Create only 50 pairs (below minimum of 100)
	pairs := make([]*apiv1.KeyValuePair, 50)
	testValues := make(map[string]float64)
	for i := 0; i < 50; i++ {
		key := "key" + string(rune('0'+(i%10))) + string(rune('0'+(i/10)))
		valueF64 := float64(i) * 0.123
		testValues[key] = valueF64
		valueBytes := float64ToBytes(valueF64)
		pairs[i] = &apiv1.KeyValuePair{
			Key:   key,
			Value: valueBytes,
		}
	}

	// Publish values - should fail with OKVS backend (< 100 pairs)
	_, err = server.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs:    pairs,
	})

	// This should fail - OKVS requires at least 100 pairs
	if err == nil {
		t.Fatal("PublishValues should fail with OKVS backend when < 100 pairs")
	}

	// Verify error is InvalidArgument
	s, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Error should be a gRPC status error: %v", err)
	}
	if s.Code().String() != "InvalidArgument" {
		t.Fatalf("Expected InvalidArgument error, got: %s", s.Code().String())
	}

	t.Logf("✅ OKVS backend correctly rejects < 100 pairs: %v", err)
}

// TestRBOKVSIntegration_InvalidValueSize tests that encoding fails with non-8-byte values
func TestRBOKVSIntegration_InvalidValueSize(t *testing.T) {
	grpcSrv, server, store, _ := setupTestServerWithRBOKVS(t)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	ctx := context.Background()
	roundID := uint64(300)
	expectedWorkers := int32(1)

	// Start round
	_, err := server.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: expectedWorkers,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Create 100 pairs with invalid value sizes (not 8 bytes)
	pairs := make([]*apiv1.KeyValuePair, 100)
	for i := 0; i < 100; i++ {
		key := "key" + string(rune('0'+(i%10))) + string(rune('0'+(i/10)))
		// Use 4-byte value instead of 8 bytes
		valueBytes := []byte("1234") // Only 4 bytes, not 8
		pairs[i] = &apiv1.KeyValuePair{
			Key:   key,
			Value: valueBytes,
		}
	}

	// Publish values - should fail with value size error
	_, err = server.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs:    pairs,
	})

	// This should fail because values are not 8 bytes
	if err == nil {
		t.Fatal("PublishValues should fail with invalid value size, but succeeded")
	}

	t.Logf("✅ Correctly rejected encoding with invalid value sizes: %v", err)
}

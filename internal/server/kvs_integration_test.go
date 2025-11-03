package server

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"github.com/mundrapranay/silhouette-db/internal/crypto"
	"github.com/mundrapranay/silhouette-db/internal/store"
	"github.com/mundrapranay/silhouette-db/pkg/client"
)

// TestServer_KVS_Integration tests the server with KVS backend
func TestServer_KVS_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	config := store.Config{
		NodeID:           "test-node",
		ListenAddr:       "127.0.0.1:0",
		DataDir:          tmpDir,
		Bootstrap:        true,
		HeartbeatTimeout: 1000 * time.Millisecond,
		ElectionTimeout:  1000 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	s, err := store.NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Shutdown()

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

	// Use KVS encoder
	kvsEncoder := crypto.NewKVSEncoder()
	server := NewServer(s, kvsEncoder, "kvs")

	// Start gRPC server on random port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	grpcSrv := grpc.NewServer()
	apiv1.RegisterCoordinationServiceServer(grpcSrv, server)

	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			t.Logf("gRPC server error: %v", err)
		}
	}()
	defer grpcSrv.GracefulStop()

	// Get server address
	addr := lis.Addr().String()
	t.Logf("Server listening on %s", addr)

	// Create client (pass nil for PIR client - will be created per round)
	dbClient, err := client.NewClient(addr, nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer dbClient.Close()

	ctx := context.Background()

	// Test with small number of pairs (KVS should work, OKVS would fail)
	roundID := uint64(1)
	expectedWorkers := int32(1)

	// Start round
	err = dbClient.StartRound(ctx, roundID, expectedWorkers)
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Publish pairs (less than 100 - OKVS would fail here, KVS should work)
	pairs := make(map[string][]byte)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		value := float64ToBytes(float64(i))
		pairs[key] = value
	}

	err = dbClient.PublishValues(ctx, roundID, "worker1", pairs)
	if err != nil {
		t.Fatalf("PublishValues failed: %v", err)
	}

	// Wait for round completion
	time.Sleep(500 * time.Millisecond)

	// Initialize PIR client
	err = dbClient.InitializePIRClient(ctx, roundID)
	if err != nil {
		t.Fatalf("InitializePIRClient failed: %v", err)
	}

	// Query a value
	value, err := dbClient.GetValue(ctx, roundID, "key0")
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}

	// Verify value (allowing for padding)
	actual := bytesToFloat64(value)
	if math.Abs(actual-0.0) > 0.0001 {
		t.Errorf("Expected 0.0, got %f", actual)
	}
	t.Logf("Successfully retrieved value: %f", actual)
}

// TestServer_KVS_vs_OKVS_Comparison compares results between KVS and OKVS backends
func TestServer_KVS_vs_OKVS_Comparison(t *testing.T) {
	// Test with 100 pairs (both should work)
	pairs := make(map[string][]byte, 100)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key%d", i)
		value := float64ToBytes(float64(i))
		pairs[key] = value
	}

	// Test KVS encoding/decoding
	kvsEncoder := crypto.NewKVSEncoder()
	kvsBlob, err := kvsEncoder.Encode(pairs)
	if err != nil {
		t.Fatalf("KVS Encode failed: %v", err)
	}

	kvsDecoder, err := crypto.NewKVSDecoder(kvsBlob)
	if err != nil {
		t.Fatalf("KVS Decode failed: %v", err)
	}

	// Verify KVS round-trip
	for k, expected := range pairs {
		value, err := kvsDecoder.Decode(kvsBlob, k)
		if err != nil {
			t.Fatalf("KVS Decode failed for key %s: %v", k, err)
		}
		if !bytes.Equal(value, expected) {
			t.Errorf("KVS round-trip failed for key %s", k)
		}
	}

	// Test OKVS encoding/decoding (if cgo available)
	if testing.Short() {
		t.Skip("Skipping OKVS comparison in short mode")
	}

	// Note: OKVS test requires cgo and 100+ pairs
	// This is tested in okvs_integration_test.go
	t.Log("KVS test passed. OKVS comparison requires cgo (tested separately).")
}

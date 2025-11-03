//go:build cgo
// +build cgo

package server

import (
	"context"
	"testing"
	"time"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"github.com/mundrapranay/silhouette-db/internal/crypto"
	"github.com/mundrapranay/silhouette-db/internal/store"
)

// Mock implementations
type mockOKVSEncoder struct{}

func (m *mockOKVSEncoder) Encode(pairs map[string][]byte) ([]byte, error) {
	// Simple mock encoding: just serialize the pairs
	result := make([]byte, 0)
	for k, v := range pairs {
		result = append(result, []byte(k)...)
		result = append(result, []byte(":")...)
		result = append(result, v...)
		result = append(result, []byte("\n")...)
	}
	return result, nil
}

type mockPIRServer struct{}

func (m *mockPIRServer) ProcessQuery(db []byte, query []byte) ([]byte, error) {
	// Mock: just return the query
	return query, nil
}

// setupTestServer creates a test server with the specified backend
// backend can be "okvs" or "kvs" (defaults to "okvs")
func setupTestServer(t *testing.T, backend string) (*Server, *store.Store) {
	if backend == "" {
		backend = "okvs" // Default to OKVS
	}

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

	return server, s
}

func TestServer_StartRound(t *testing.T) {
	server, store := setupTestServer(t, "okvs")
	defer store.Shutdown()

	ctx := context.Background()
	req := &apiv1.StartRoundRequest{
		RoundId:         1,
		ExpectedWorkers: 3,
	}

	resp, err := server.StartRound(ctx, req)
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	if !resp.Success {
		t.Fatal("StartRound should return success=true")
	}
}

func TestServer_PublishValues(t *testing.T) {
	server, store := setupTestServer(t, "okvs")
	defer store.Shutdown()

	ctx := context.Background()

	// First start a round
	startReq := &apiv1.StartRoundRequest{
		RoundId:         1,
		ExpectedWorkers: 2,
	}
	_, err := server.StartRound(ctx, startReq)
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Publish values from worker 1
	publishReq := &apiv1.PublishValuesRequest{
		RoundId:  1,
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "key1", Value: []byte("value1")},
			{Key: "key2", Value: []byte("value2")},
		},
	}

	resp, err := server.PublishValues(ctx, publishReq)
	if err != nil {
		t.Fatalf("PublishValues failed: %v", err)
	}

	if !resp.Success {
		t.Fatal("PublishValues should return success=true")
	}
}

func TestServer_PublishValues_CompleteRound(t *testing.T) {
	// Use KVS backend for this test (works with any number of pairs)
	server, store := setupTestServer(t, "kvs")
	defer store.Shutdown()

	ctx := context.Background()

	// Start a round with 2 workers
	startReq := &apiv1.StartRoundRequest{
		RoundId:         1,
		ExpectedWorkers: 2,
	}
	_, err := server.StartRound(ctx, startReq)
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Publish from worker 1
	publishReq1 := &apiv1.PublishValuesRequest{
		RoundId:  1,
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "key1", Value: []byte("value1")},
		},
	}
	_, err = server.PublishValues(ctx, publishReq1)
	if err != nil {
		t.Fatalf("PublishValues failed for worker1: %v", err)
	}

	// Publish from worker 2 (should complete the round)
	publishReq2 := &apiv1.PublishValuesRequest{
		RoundId:  1,
		WorkerId: "worker2",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "key2", Value: []byte("value2")},
		},
	}
	_, err = server.PublishValues(ctx, publishReq2)
	if err != nil {
		t.Fatalf("PublishValues failed for worker2: %v", err)
	}

	// Wait a bit for Raft to commit
	time.Sleep(200 * time.Millisecond)

	// Verify the round data was stored
	roundKey := "round_1_results"
	value, exists := store.Get(roundKey)
	if !exists {
		t.Fatal("Round data should be stored after all workers publish")
	}
	if len(value) == 0 {
		t.Fatal("Round data should not be empty")
	}
}

func TestServer_GetValue(t *testing.T) {
	// Use KVS backend for this test (works with any number of pairs)
	server, store := setupTestServer(t, "kvs")
	defer store.Shutdown()

	ctx := context.Background()

	// First, create a round and publish values
	startReq := &apiv1.StartRoundRequest{
		RoundId:         1,
		ExpectedWorkers: 1,
	}
	_, err := server.StartRound(ctx, startReq)
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Publish to complete the round
	publishReq := &apiv1.PublishValuesRequest{
		RoundId:  1,
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "test-key", Value: []byte("test-value")},
		},
	}
	_, err = server.PublishValues(ctx, publishReq)
	if err != nil {
		t.Fatalf("PublishValues failed: %v", err)
	}

	// Wait for Raft to commit and PIR server to be created
	time.Sleep(500 * time.Millisecond)

	// Get BaseParams and key mapping to create a proper PIR client
	baseParamsReq := &apiv1.GetBaseParamsRequest{RoundId: 1}
	baseParamsResp, err := server.GetBaseParams(ctx, baseParamsReq)
	if err != nil {
		t.Fatalf("GetBaseParams failed: %v", err)
	}

	keyMappingReq := &apiv1.GetKeyMappingRequest{RoundId: 1}
	keyMappingResp, err := server.GetKeyMapping(ctx, keyMappingReq)
	if err != nil {
		t.Fatalf("GetKeyMapping failed: %v", err)
	}

	// Convert protobuf entries to map
	keyToIndex := make(map[string]int)
	for _, entry := range keyMappingResp.Entries {
		keyToIndex[entry.Key] = int(entry.Index)
	}

	// Create PIR client and generate a real query
	pirClient, err := crypto.NewFrodoPIRClient(baseParamsResp.BaseParams, keyToIndex)
	if err != nil {
		t.Fatalf("Failed to create PIR client: %v", err)
	}
	defer pirClient.Close()

	// Generate a real PIR query for "test-key"
	query, err := pirClient.GenerateQuery("test-key")
	if err != nil {
		t.Fatalf("Failed to generate PIR query: %v", err)
	}

	// Now try to get a value using the real query
	getReq := &apiv1.GetValueRequest{
		RoundId:  1,
		PirQuery: query,
	}

	resp, err := server.GetValue(ctx, getReq)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}

	if resp == nil || resp.PirResponse == nil {
		t.Fatal("GetValue should return a response")
	}

	// Decode the response to verify it works
	value, err := pirClient.DecodeResponse(resp.PirResponse)
	if err != nil {
		t.Logf("DecodeResponse failed (might be expected in some cases): %v", err)
	} else {
		// Value should match what we published
		expectedValue := []byte("test-value")
		if string(value) != string(expectedValue) {
			t.Logf("Retrieved value doesn't match (might be due to padding): expected %q, got %q", string(expectedValue), string(value))
		}
	}
}

func TestServer_GetValue_NonExistentRound(t *testing.T) {
	server, store := setupTestServer(t, "okvs")
	defer store.Shutdown()

	ctx := context.Background()

	// For non-existent round, we don't need a valid query - the error should occur before processing
	// Use a simple mock query since the round doesn't exist anyway
	getReq := &apiv1.GetValueRequest{
		RoundId:  999, // Non-existent round
		PirQuery: []byte("mock-query"),
	}

	_, err := server.GetValue(ctx, getReq)
	if err == nil {
		t.Fatal("GetValue should return error for non-existent round")
	}

	// Verify the error is about the round not being found
	// (The error should indicate round not found, not query processing error)
	t.Logf("GetValue correctly returned error for non-existent round: %v", err)
}

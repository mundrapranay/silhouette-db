package server

import (
	"context"
	"testing"
	"time"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
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

func setupTestServer(t *testing.T) (*Server, *store.Store) {
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

	okvsEncoder := &mockOKVSEncoder{}
	server := NewServer(s, okvsEncoder)

	return server, s
}

func TestServer_StartRound(t *testing.T) {
	server, store := setupTestServer(t)
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
	server, store := setupTestServer(t)
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
	server, store := setupTestServer(t)
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
	server, store := setupTestServer(t)
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

	// Wait for Raft to commit
	time.Sleep(200 * time.Millisecond)

	// Now try to get a value
	getReq := &apiv1.GetValueRequest{
		RoundId:  1,
		PirQuery: []byte("mock-query"),
	}

	resp, err := server.GetValue(ctx, getReq)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}

	if resp == nil || resp.PirResponse == nil {
		t.Fatal("GetValue should return a response")
	}
}

func TestServer_GetValue_NonExistentRound(t *testing.T) {
	server, store := setupTestServer(t)
	defer store.Shutdown()

	ctx := context.Background()

	getReq := &apiv1.GetValueRequest{
		RoundId:  999, // Non-existent round
		PirQuery: []byte("mock-query"),
	}

	_, err := server.GetValue(ctx, getReq)
	if err == nil {
		t.Fatal("GetValue should return error for non-existent round")
	}
}

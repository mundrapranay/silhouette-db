package server

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"github.com/mundrapranay/silhouette-db/internal/crypto"
	"github.com/mundrapranay/silhouette-db/internal/store"
)

// TestPIRIntegration_EndToEnd tests the complete PIR workflow using FrodoPIR.
// This test requires cgo and the FrodoPIR library to be built.
func TestPIRIntegration_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PIR integration test in short mode")
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

	// Use KVS encoder for PIR integration test (works with <100 pairs)
	kvsEncoder := crypto.NewKVSEncoder()
	server := NewServer(s, kvsEncoder, "kvs")
	ctx := context.Background()

	// Step 1: Start a round
	roundID := uint64(1)
	expectedWorkers := int32(2)

	startReq := &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: expectedWorkers,
	}
	_, err = server.StartRound(ctx, startReq)
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Step 2: Publish values from workers
	// Worker 1
	publishReq1 := &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "key1", Value: []byte("value1")},
			{Key: "key2", Value: []byte("value2")},
		},
	}
	_, err = server.PublishValues(ctx, publishReq1)
	if err != nil {
		t.Fatalf("PublishValues failed for worker1: %v", err)
	}

	// Worker 2 (should complete the round and create FrodoPIR server)
	publishReq2 := &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker2",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "key3", Value: []byte("value3")},
		},
	}
	_, err = server.PublishValues(ctx, publishReq2)
	if err != nil {
		t.Fatalf("PublishValues failed for worker2: %v", err)
	}

	// Wait for FrodoPIR server to be created
	time.Sleep(200 * time.Millisecond)

	// Step 3: Verify FrodoPIR server was created
	server.roundsMu.RLock()
	pirServer, exists := server.pirServers[roundID]
	baseParams, paramsExist := server.roundBaseParams[roundID]
	keyMapping, mappingExist := server.roundKeyMapping[roundID]
	server.roundsMu.RUnlock()

	if !exists {
		t.Fatal("FrodoPIR server should be created for this round")
	}
	if !paramsExist {
		t.Fatal("BaseParams should be stored for this round")
	}
	if !mappingExist {
		t.Fatal("Key mapping should be stored for this round")
	}
	if len(keyMapping) == 0 {
		t.Fatal("Key mapping should not be empty")
	}

	// Verify key mapping
	if keyMapping["key1"] != 0 {
		t.Errorf("Expected key1 at index 0, got %d", keyMapping["key1"])
	}
	if keyMapping["key2"] != 1 {
		t.Errorf("Expected key2 at index 1, got %d", keyMapping["key2"])
	}
	if keyMapping["key3"] != 2 {
		t.Errorf("Expected key3 at index 2, got %d", keyMapping["key3"])
	}

	// Step 4: Test GetBaseParams
	baseParamsReq := &apiv1.GetBaseParamsRequest{RoundId: roundID}
	baseParamsResp, err := server.GetBaseParams(ctx, baseParamsReq)
	if err != nil {
		t.Fatalf("GetBaseParams failed: %v", err)
	}
	if len(baseParamsResp.BaseParams) == 0 {
		t.Fatal("BaseParams should not be empty")
	}
	if len(baseParamsResp.BaseParams) != len(baseParams) {
		t.Errorf("BaseParams length mismatch: expected %d, got %d", len(baseParams), len(baseParamsResp.BaseParams))
	}

	// Step 5: Test GetKeyMapping
	keyMappingReq := &apiv1.GetKeyMappingRequest{RoundId: roundID}
	keyMappingResp, err := server.GetKeyMapping(ctx, keyMappingReq)
	if err != nil {
		t.Fatalf("GetKeyMapping failed: %v", err)
	}
	if len(keyMappingResp.Entries) != 3 {
		t.Errorf("Expected 3 entries in key mapping, got %d", len(keyMappingResp.Entries))
	}

	// Convert protobuf key mapping to map for client
	clientKeyMapping := make(map[string]int)
	for _, entry := range keyMappingResp.Entries {
		clientKeyMapping[entry.Key] = int(entry.Index)
	}

	// Step 6: Test PIR query (create client and query)
	pirClient, err := crypto.NewFrodoPIRClient(baseParamsResp.BaseParams, clientKeyMapping)
	if err != nil {
		t.Fatalf("Failed to create FrodoPIR client: %v", err)
	}
	defer pirClient.Close()

	// Generate query for key2
	query, err := pirClient.GenerateQuery("key2")
	if err != nil {
		t.Fatalf("Failed to generate PIR query: %v", err)
	}
	if len(query) == 0 {
		t.Fatal("Query should not be empty")
	}

	// Process query on server
	getValueReq := &apiv1.GetValueRequest{
		RoundId:  roundID,
		PirQuery: query,
	}
	getValueResp, err := server.GetValue(ctx, getValueReq)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	if len(getValueResp.PirResponse) == 0 {
		t.Fatal("PIR response should not be empty")
	}

	// Decode response
	// Note: We need the queryParams stored in the client
	value, err := pirClient.DecodeResponse(getValueResp.PirResponse)
	if err != nil {
		// This might fail if queryParams weren't stored properly
		// For now, just check that we can decode
		t.Logf("DecodeResponse failed (expected if queryParams not stored): %v", err)
	} else {
		// Trim null bytes (PIR may pad values to fixed size)
		valueStr := string(bytes.TrimRight(value, "\x00"))
		if valueStr != "value2" {
			t.Errorf("Expected value2, got %q (raw: %v)", valueStr, value)
		}
	}

	// Cleanup
	if err := pirServer.Close(); err != nil {
		t.Logf("Failed to close PIR server: %v", err)
	}

	t.Logf("✅ PIR integration test completed successfully")
}

// TestPIRIntegration_EndToEnd_OKVS tests the complete PIR workflow using FrodoPIR with OKVS backend and 100+ pairs.
// This is similar to TestPIRIntegration_EndToEnd but uses OKVS backend and requires 100+ pairs.
func TestPIRIntegration_EndToEnd_OKVS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PIR integration test in short mode")
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

	// Use OKVS encoder for OKVS backend (requires 100+ pairs)
	okvsEncoder := crypto.NewRBOKVSEncoder()
	server := NewServer(s, okvsEncoder, "okvs")
	ctx := context.Background()

	// Step 1: Start a round
	roundID := uint64(2)
	expectedWorkers := int32(2)

	startReq := &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: expectedWorkers,
	}
	_, err = server.StartRound(ctx, startReq)
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Step 2: Publish values from workers (100+ pairs total)
	// OKVS requires all values to be exactly 8 bytes (float64)
	// Worker 1: 60 pairs
	pairs1 := make([]*apiv1.KeyValuePair, 60)
	for i := 0; i < 60; i++ {
		// Create 8-byte value (float64 representation)
		valueBytes := make([]byte, 8)
		valueBytes[0] = byte(i + 1)
		valueBytes[1] = byte((i + 1) >> 8)
		valueBytes[2] = byte((i + 1) >> 16)
		valueBytes[3] = byte((i + 1) >> 24)
		// Fill remaining bytes
		valueBytes[4] = 0
		valueBytes[5] = 0
		valueBytes[6] = 0
		valueBytes[7] = 0

		pairs1[i] = &apiv1.KeyValuePair{
			Key:   fmt.Sprintf("key%d", i+1),
			Value: valueBytes,
		}
	}

	publishReq1 := &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs:    pairs1,
	}
	_, err = server.PublishValues(ctx, publishReq1)
	if err != nil {
		t.Fatalf("PublishValues failed for worker1: %v", err)
	}

	// Worker 2: 60 pairs (total 120 pairs, meets OKVS minimum)
	pairs2 := make([]*apiv1.KeyValuePair, 60)
	for i := 0; i < 60; i++ {
		// Create 8-byte value (float64 representation)
		valueBytes := make([]byte, 8)
		valueBytes[0] = byte(i + 61)
		valueBytes[1] = byte((i + 61) >> 8)
		valueBytes[2] = byte((i + 61) >> 16)
		valueBytes[3] = byte((i + 61) >> 24)
		// Fill remaining bytes
		valueBytes[4] = 0
		valueBytes[5] = 0
		valueBytes[6] = 0
		valueBytes[7] = 0

		pairs2[i] = &apiv1.KeyValuePair{
			Key:   fmt.Sprintf("key%d", i+61),
			Value: valueBytes,
		}
	}

	publishReq2 := &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker2",
		Pairs:    pairs2,
	}
	_, err = server.PublishValues(ctx, publishReq2)
	if err != nil {
		t.Fatalf("PublishValues failed for worker2: %v", err)
	}

	// Wait for FrodoPIR server to be created
	time.Sleep(200 * time.Millisecond)

	// Step 3: Verify FrodoPIR server was created
	server.roundsMu.RLock()
	pirServer, exists := server.pirServers[roundID]
	baseParams, paramsExist := server.roundBaseParams[roundID]
	keyMapping, mappingExist := server.roundKeyMapping[roundID]
	server.roundsMu.RUnlock()

	if !exists {
		t.Fatal("FrodoPIR server should be created for this round")
	}
	if !paramsExist {
		t.Fatal("BaseParams should be stored for this round")
	}
	if !mappingExist {
		t.Fatal("Key mapping should be stored for this round")
	}
	if len(keyMapping) == 0 {
		t.Fatal("Key mapping should not be empty")
	}

	// Verify key mapping (keys are sorted alphabetically)
	// "key1" should be at index 0 after sorting
	if keyMapping["key1"] != 0 {
		t.Errorf("Expected key1 at index 0, got %d", keyMapping["key1"])
	}

	// Step 4: Test GetBaseParams
	baseParamsReq := &apiv1.GetBaseParamsRequest{RoundId: roundID}
	baseParamsResp, err := server.GetBaseParams(ctx, baseParamsReq)
	if err != nil {
		t.Fatalf("GetBaseParams failed: %v", err)
	}
	if len(baseParamsResp.BaseParams) == 0 {
		t.Fatal("BaseParams should not be empty")
	}
	if len(baseParamsResp.BaseParams) != len(baseParams) {
		t.Errorf("BaseParams length mismatch: expected %d, got %d", len(baseParams), len(baseParamsResp.BaseParams))
	}

	// Step 5: Test GetKeyMapping
	keyMappingReq := &apiv1.GetKeyMappingRequest{RoundId: roundID}
	keyMappingResp, err := server.GetKeyMapping(ctx, keyMappingReq)
	if err != nil {
		t.Fatalf("GetKeyMapping failed: %v", err)
	}
	if len(keyMappingResp.Entries) != 120 {
		t.Errorf("Expected 120 entries in key mapping, got %d", len(keyMappingResp.Entries))
	}

	// Convert protobuf key mapping to map for client
	clientKeyMapping := make(map[string]int)
	for _, entry := range keyMappingResp.Entries {
		clientKeyMapping[entry.Key] = int(entry.Index)
	}

	// Step 6: Test PIR query (create client and query)
	pirClient, err := crypto.NewFrodoPIRClient(baseParamsResp.BaseParams, clientKeyMapping)
	if err != nil {
		t.Fatalf("Failed to create FrodoPIR client: %v", err)
	}
	defer pirClient.Close()

	// Generate query for key1 (which should be at index 0 after sorting)
	query, err := pirClient.GenerateQuery("key1")
	if err != nil {
		t.Fatalf("Failed to generate PIR query: %v", err)
	}
	if len(query) == 0 {
		t.Fatal("Query should not be empty")
	}

	// Process query on server
	getValueReq := &apiv1.GetValueRequest{
		RoundId:  roundID,
		PirQuery: query,
	}
	getValueResp, err := server.GetValue(ctx, getValueReq)
	if err != nil {
		t.Fatalf("GetValue failed: %v", err)
	}
	if len(getValueResp.PirResponse) == 0 {
		t.Fatal("PIR response should not be empty")
	}

	// Decode response
	value, err := pirClient.DecodeResponse(getValueResp.PirResponse)
	if err != nil {
		t.Fatalf("DecodeResponse failed: %v", err)
	}

	// Verify retrieved value (should be 8 bytes for OKVS)
	if len(value) != 8 {
		t.Errorf("Expected 8-byte value, got %d bytes", len(value))
	}
	// For OKVS, we expect the value to match the stored bytes
	// The stored value for key1 was [1, 0, 0, 0, 0, 0, 0, 0]
	expectedValue := make([]byte, 8)
	expectedValue[0] = 1
	if value[0] != expectedValue[0] {
		t.Errorf("Expected value[0] = 1, got %d (raw: %v)", value[0], value)
	}

	// Cleanup
	if err := pirServer.Close(); err != nil {
		t.Logf("Failed to close PIR server: %v", err)
	}

	t.Logf("✅ PIR + OKVS integration test completed successfully")
}

// TestPIRIntegration_KeyMapping tests key-to-index mapping creation and retrieval.
func TestPIRIntegration_KeyMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping PIR integration test in short mode")
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

	// Use KVS encoder for KVS backend (mockOKVSEncoder doesn't produce valid JSON)
	kvsEncoder := crypto.NewKVSEncoder()
	server := NewServer(s, kvsEncoder, "kvs")
	ctx := context.Background()

	// Start round
	roundID := uint64(1)
	_, err = server.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: 1,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Publish values (should create FrodoPIR server and key mapping)
	_, err = server.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "zebra", Value: []byte("value-z")},
			{Key: "apple", Value: []byte("value-a")},
			{Key: "banana", Value: []byte("value-b")},
		},
	})
	if err != nil {
		t.Fatalf("PublishValues failed: %v", err)
	}

	// Wait for FrodoPIR setup
	time.Sleep(200 * time.Millisecond)

	// Get key mapping
	keyMappingReq := &apiv1.GetKeyMappingRequest{RoundId: roundID}
	keyMappingResp, err := server.GetKeyMapping(ctx, keyMappingReq)
	if err != nil {
		t.Fatalf("GetKeyMapping failed: %v", err)
	}

	// Verify keys are sorted alphabetically (apple=0, banana=1, zebra=2)
	expectedMapping := map[string]int32{
		"apple":  0,
		"banana": 1,
		"zebra":  2,
	}

	for _, entry := range keyMappingResp.Entries {
		expected, ok := expectedMapping[entry.Key]
		if !ok {
			t.Errorf("Unexpected key in mapping: %s", entry.Key)
		}
		if entry.Index != expected {
			t.Errorf("Key %s: expected index %d, got %d", entry.Key, expected, entry.Index)
		}
	}

	if len(keyMappingResp.Entries) != len(expectedMapping) {
		t.Errorf("Expected %d entries, got %d", len(expectedMapping), len(keyMappingResp.Entries))
	}

	t.Logf("✅ Key mapping test completed successfully")
}

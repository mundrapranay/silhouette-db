//go:build cgo
// +build cgo

package server

import (
	"context"
	"encoding/binary"
	"testing"
	"time"
	"unsafe"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"

	"github.com/mundrapranay/silhouette-db/pkg/client"
)

// TestPIR_OKVSIntegration tests the complete flow: OKVS encoding + PIR queries
func TestPIR_OKVSIntegration(t *testing.T) {
	grpcSrv, server, store, addr := setupTestServerWithRBOKVS(t)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	ctx := context.Background()
	roundID := uint64(500)
	expectedWorkers := int32(1)

	// Step 1: Start round
	_, err := server.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: expectedWorkers,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Step 2: Create 100+ pairs with float64 values
	testPairs := make([]*apiv1.KeyValuePair, 100)
	testValues := make(map[string]float64)

	for i := 0; i < 100; i++ {
		key := "pir-test-key-" + string(rune('0'+(i%10))) + string(rune('0'+(i/10)))
		valueF64 := float64(i) * 0.789
		testValues[key] = valueF64
		valueBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(valueBytes, *(*uint64)(unsafe.Pointer(&valueF64)))

		testPairs[i] = &apiv1.KeyValuePair{
			Key:   key,
			Value: valueBytes,
		}
	}

	// Step 3: Publish values (should trigger OKVS encoding)
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
	roundKey := "round_500_results"
	okvsBlob, exists := store.Get(roundKey)
	if !exists {
		t.Fatal("OKVS blob should be stored in Raft")
	}
	if len(okvsBlob) == 0 {
		t.Fatal("OKVS blob should not be empty")
	}
	t.Logf("✅ OKVS blob stored: %d bytes", len(okvsBlob))

	// Step 5: Create client and test PIR queries with OKVS-encoded data
	grpcClient, err := client.NewClient(addr, nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer grpcClient.Close()

	// Step 6: Test PIR query for a few keys
	testKeys := []string{"pir-test-key-00", "pir-test-key-50", "pir-test-key-99"}
	for _, key := range testKeys {
		expectedValue, ok := testValues[key]
		if !ok {
			t.Fatalf("Test key %s not found in test values", key)
		}

		// Get value using PIR
		retrievedBytes, err := grpcClient.GetValue(ctx, roundID, key)
		if err != nil {
			t.Fatalf("GetValue failed for key %s: %v", key, err)
		}

		if len(retrievedBytes) != 8 {
			t.Fatalf("Retrieved value should be 8 bytes, got %d for key %s", len(retrievedBytes), key)
		}

		// Convert bytes to float64
		retrievedValue := *(*float64)(unsafe.Pointer(&retrievedBytes[0]))

		// Use approximate equality for floating point comparison
		epsilon := 1e-10
		diff := retrievedValue - expectedValue
		if diff < 0 {
			diff = -diff
		}
		if diff > epsilon && retrievedValue != expectedValue {
			t.Errorf("Retrieved value mismatch for key %s: expected %f, got %f (diff: %e)",
				key, expectedValue, retrievedValue, diff)
		} else {
			t.Logf("✅ PIR query for key %s: retrieved %f (matches expected)", key, retrievedValue)
		}
	}
}

// TestPIR_OKVSIntegration_MultipleQueries tests multiple PIR queries on OKVS-encoded data
func TestPIR_OKVSIntegration_MultipleQueries(t *testing.T) {
	grpcSrv, server, store, addr := setupTestServerWithRBOKVS(t)
	defer grpcSrv.Stop()
	defer store.Shutdown()

	ctx := context.Background()
	roundID := uint64(600)
	expectedWorkers := int32(1)

	// Start round
	_, err := server.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: expectedWorkers,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Create 150 pairs
	testPairs := make([]*apiv1.KeyValuePair, 150)
	testValues := make(map[string]float64)

	for i := 0; i < 150; i++ {
		key := "multi-key-" + string(rune('0'+(i%10))) + string(rune('0'+(i/10)))
		valueF64 := float64(i) * 1.234
		testValues[key] = valueF64
		valueBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(valueBytes, *(*uint64)(unsafe.Pointer(&valueF64)))

		testPairs[i] = &apiv1.KeyValuePair{
			Key:   key,
			Value: valueBytes,
		}
	}

	// Publish values
	_, err = server.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "test-worker",
		Pairs:    testPairs,
	})
	if err != nil {
		t.Fatalf("PublishValues failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Create client
	grpcClient, err := client.NewClient(addr, nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer grpcClient.Close()

	// Query multiple random keys
	queryKeys := []string{"multi-key-00", "multi-key-25", "multi-key-50", "multi-key-75", "multi-key-99"}
	for _, key := range queryKeys {
		expectedValue, ok := testValues[key]
		if !ok {
			t.Fatalf("Test key %s not found", key)
		}

		retrievedBytes, err := grpcClient.GetValue(ctx, roundID, key)
		if err != nil {
			t.Fatalf("GetValue failed for key %s: %v", key, err)
		}

		retrievedValue := *(*float64)(unsafe.Pointer(&retrievedBytes[0]))

		epsilon := 1e-10
		diff := retrievedValue - expectedValue
		if diff < 0 {
			diff = -diff
		}
		if diff > epsilon && retrievedValue != expectedValue {
			t.Errorf("Value mismatch for key %s: expected %f, got %f", key, expectedValue, retrievedValue)
		}
	}

	t.Logf("✅ Successfully queried %d keys using PIR on OKVS-encoded data", len(queryKeys))
}

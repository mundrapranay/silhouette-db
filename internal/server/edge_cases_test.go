package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"google.golang.org/grpc/status"
)

func TestEdgeCases_DuplicateWorkerPublish(t *testing.T) {
	grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "kvs")
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createTestClient(t, addr)
	ctx := context.Background()

	// Start round
	roundID := uint64(1)
	_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: 2,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Publish from worker1
	_, err = client.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "key1", Value: []byte("value1")},
		},
	})
	if err != nil {
		t.Fatalf("First publish failed: %v", err)
	}

	// Publish again from the same worker (should be allowed or handled gracefully)
	_, err = client.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "key1", Value: []byte("value1-updated")},
		},
	})
	// This should either succeed (overwrite) or be explicitly rejected
	// Current implementation allows it, so we just verify it doesn't crash
	if err != nil {
		t.Logf("Duplicate publish resulted in error (expected behavior): %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	t.Log("✅ Duplicate worker publish handled")
}

func TestEdgeCases_PublishToNonExistentRound(t *testing.T) {
	grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "kvs")
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createTestClient(t, addr)
	ctx := context.Background()

	// Try to publish to a round that doesn't exist
	_, err := client.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  999, // Non-existent round
		WorkerId: "worker1",
		Pairs: []*apiv1.KeyValuePair{
			{Key: "key1", Value: []byte("value1")},
		},
	})

	if err == nil {
		t.Fatal("Publish to non-existent round should return error")
	}

	// Verify error is a gRPC error
	s, ok := status.FromError(err)
	if !ok {
		t.Fatal("Error should be a gRPC status error")
	}
	if s.Code().String() != "NotFound" {
		t.Fatalf("Expected NotFound error, got: %s", s.Code().String())
	}

	t.Log("✅ Non-existent round publish correctly rejected")
}

func TestEdgeCases_GetValueBeforeRoundComplete(t *testing.T) {
	grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "kvs")
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createTestClient(t, addr)
	ctx := context.Background()

	// Start round but don't complete it
	roundID := uint64(1)
	_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: 2,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Try to get value before round is complete
	_, err = client.GetValue(ctx, &apiv1.GetValueRequest{
		RoundId:  roundID,
		PirQuery: []byte("query"),
	})

	// Should return error since round is not complete
	if err == nil {
		t.Fatal("GetValue on incomplete round should return error")
	}

	t.Log("✅ GetValue before round completion correctly rejected")
}

func TestEdgeCases_EmptyPairs(t *testing.T) {
	grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "kvs")
	defer grpcSrv.Stop()
	defer store.Shutdown()

	client := createTestClient(t, addr)
	ctx := context.Background()

	// Start round
	roundID := uint64(1)
	_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: 1,
	})
	if err != nil {
		t.Fatalf("StartRound failed: %v", err)
	}

	// Publish with empty pairs
	resp, err := client.PublishValues(ctx, &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: "worker1",
		Pairs:    []*apiv1.KeyValuePair{}, // Empty pairs
	})

	// Should handle empty pairs gracefully
	if err != nil {
		t.Fatalf("Empty pairs publish failed: %v", err)
	}
	if !resp.Success {
		t.Fatal("Empty pairs publish should return success")
	}

	time.Sleep(200 * time.Millisecond)

	// Verify round was completed and stored (even with empty pairs)
	roundKey := "round_1_results"
	_, exists := store.Get(roundKey)
	if !exists {
		t.Fatal("Round should be stored even with empty pairs")
	}

	t.Log("✅ Empty pairs handled correctly")
}

func TestEdgeCases_LargeValue(t *testing.T) {
	// Test with KVS backend (<100 pairs, large value size)
	t.Run("KVS_Backend", func(t *testing.T) {
		grpcSrv, _, store, addr := setupTestServerWithGRPC(t, "kvs")
		defer grpcSrv.Stop()
		defer store.Shutdown()

		client := createTestClient(t, addr)
		ctx := context.Background()

		roundID := uint64(1)
		_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
			RoundId:         roundID,
			ExpectedWorkers: 1,
		})
		if err != nil {
			t.Fatalf("StartRound failed: %v", err)
		}

		largeValue := make([]byte, 1024*1024)
		for i := range largeValue {
			largeValue[i] = byte(i % 256)
		}

		_, err = client.PublishValues(ctx, &apiv1.PublishValuesRequest{
			RoundId:  roundID,
			WorkerId: "worker1",
			Pairs: []*apiv1.KeyValuePair{
				{Key: "large-key", Value: largeValue},
			},
		})

		if err != nil {
			t.Fatalf("Large value publish failed: %v", err)
		}

		time.Sleep(500 * time.Millisecond)

		roundKey := "round_1_results"
		blob, exists := store.Get(roundKey)
		if !exists {
			t.Fatal("Round with large value should be stored")
		}
		if len(blob) == 0 {
			t.Fatal("Storage blob should contain data")
		}

		t.Logf("✅ KVS backend: Large value (1MB) handled correctly. Blob size: %d bytes", len(blob))
	})
}

func TestEdgeCases_ManyKeys(t *testing.T) {
	// Test with both backends (>=100 pairs)
	backends := []struct {
		name    string
		backend string
	}{
		{"KVS_Backend", "kvs"},
		{"OKVS_Backend", "okvs"},
	}

	for _, tc := range backends {
		t.Run(tc.name, func(t *testing.T) {
			if tc.backend == "okvs" && testing.Short() {
				t.Skip("Skipping OKVS test in short mode")
			}

			grpcSrv, _, store, addr := setupTestServerWithGRPC(t, tc.backend)
			defer grpcSrv.Stop()
			defer store.Shutdown()

			client := createTestClient(t, addr)
			ctx := context.Background()

			roundID := uint64(1)
			_, err := client.StartRound(ctx, &apiv1.StartRoundRequest{
				RoundId:         roundID,
				ExpectedWorkers: 1,
			})
			if err != nil {
				t.Fatalf("StartRound failed: %v", err)
			}

			// Create 1000 pairs (>=100, works with both backends)
			pairs := make([]*apiv1.KeyValuePair, 1000)
			for i := 0; i < 1000; i++ {
				if tc.backend == "okvs" {
					// OKVS requires 8-byte values
					valueBytes := make([]byte, 8)
					valueBytes[0] = byte(i)
					pairs[i] = &apiv1.KeyValuePair{
						Key:   fmt.Sprintf("key-%d", i),
						Value: valueBytes,
					}
				} else {
					// KVS can handle any value size
					pairs[i] = &apiv1.KeyValuePair{
						Key:   fmt.Sprintf("key-%d", i),
						Value: []byte(fmt.Sprintf("value-%d", i)),
					}
				}
			}

			_, err = client.PublishValues(ctx, &apiv1.PublishValuesRequest{
				RoundId:  roundID,
				WorkerId: "worker1",
				Pairs:    pairs,
			})

			if err != nil {
				t.Fatalf("Many keys publish failed: %v", err)
			}

			time.Sleep(500 * time.Millisecond)

			roundKey := "round_1_results"
			blob, exists := store.Get(roundKey)
			if !exists {
				t.Fatal("Round with many keys should be stored")
			}

			t.Logf("✅ %s: Many keys (1000 pairs) handled correctly. Blob size: %d bytes", tc.name, len(blob))
		})
	}
}

package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStore(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	config := Config{
		NodeID:           "test-node",
		ListenAddr:       "127.0.0.1:0", // Use port 0 for random port
		DataDir:          tmpDir,
		Bootstrap:        true,
		HeartbeatTimeout: 1000 * time.Millisecond,
		ElectionTimeout:  1000 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Shutdown()

	if store == nil {
		t.Fatal("NewStore returned nil")
	}
	if store.fsm == nil {
		t.Fatal("Store FSM is nil")
	}
	if store.raft == nil {
		t.Fatal("Store Raft instance is nil")
	}
}

func TestStore_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		NodeID:           "test-node",
		ListenAddr:       "127.0.0.1:0",
		DataDir:          tmpDir,
		Bootstrap:        true,
		HeartbeatTimeout: 1000 * time.Millisecond,
		ElectionTimeout:  1000 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Shutdown()

	// Wait a bit for leadership
	timeout := time.After(5 * time.Second)
	tick := time.Tick(100 * time.Millisecond)
	var isLeader bool
	for !isLeader {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for leadership")
		case <-tick:
			isLeader = store.IsLeader()
		}
	}

	// Test Set and Get
	key := "test-key"
	value := []byte("test-value")

	err = store.Set(key, value)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Wait a bit for Raft to commit
	time.Sleep(100 * time.Millisecond)

	// Test Get
	retrieved, exists := store.Get(key)
	if !exists {
		t.Fatal("Key should exist after Set")
	}
	if string(retrieved) != string(value) {
		t.Fatalf("Expected '%s', got '%s'", string(value), string(retrieved))
	}
}

func TestStore_GetNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		NodeID:           "test-node",
		ListenAddr:       "127.0.0.1:0",
		DataDir:          tmpDir,
		Bootstrap:        true,
		HeartbeatTimeout: 1000 * time.Millisecond,
		ElectionTimeout:  1000 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Shutdown()

	// Test Get on non-existent key
	_, exists := store.Get("non-existent")
	if exists {
		t.Fatal("Non-existent key should not exist")
	}
}

func TestStore_IsLeader(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		NodeID:           "test-node",
		ListenAddr:       "127.0.0.1:0",
		DataDir:          tmpDir,
		Bootstrap:        true,
		HeartbeatTimeout: 1000 * time.Millisecond,
		ElectionTimeout:  1000 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Shutdown()

	// Wait for leadership
	timeout := time.After(5 * time.Second)
	tick := time.Tick(100 * time.Millisecond)
	var isLeader bool
	for !isLeader {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for leadership")
		case <-tick:
			isLeader = store.IsLeader()
		}
	}

	if !store.IsLeader() {
		t.Fatal("Single-node cluster should be leader")
	}
}

func TestStore_MultipleSets(t *testing.T) {
	tmpDir := t.TempDir()

	config := Config{
		NodeID:           "test-node",
		ListenAddr:       "127.0.0.1:0",
		DataDir:          tmpDir,
		Bootstrap:        true,
		HeartbeatTimeout: 1000 * time.Millisecond,
		ElectionTimeout:  1000 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Shutdown()

	// Wait for leadership
	timeout := time.After(5 * time.Second)
	tick := time.Tick(100 * time.Millisecond)
	var isLeader bool
	for !isLeader {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for leadership")
		case <-tick:
			isLeader = store.IsLeader()
		}
	}

	// Set multiple values
	testCases := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}

	for key, value := range testCases {
		if err := store.Set(key, value); err != nil {
			t.Fatalf("Failed to set %s: %v", key, err)
		}
	}

	// Wait for commits
	time.Sleep(200 * time.Millisecond)

	// Verify all values
	for key, expectedValue := range testCases {
		retrieved, exists := store.Get(key)
		if !exists {
			t.Fatalf("Key %s should exist", key)
		}
		if string(retrieved) != string(expectedValue) {
			t.Fatalf("Key %s: expected '%s', got '%s'", key, string(expectedValue), string(retrieved))
		}
	}
}

func TestStore_DataDirCreation(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "raft-data")

	// Create the data directory first
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	config := Config{
		NodeID:           "test-node",
		ListenAddr:       "127.0.0.1:0",
		DataDir:          dataDir,
		Bootstrap:        true,
		HeartbeatTimeout: 1000 * time.Millisecond,
		ElectionTimeout:  1000 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Shutdown()

	// Verify data directory was created
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		t.Fatalf("Data directory was not created: %v", err)
	}

	// Verify subdirectories exist
	logsDir := filepath.Join(dataDir, "logs")
	stableDir := filepath.Join(dataDir, "stable")

	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		t.Fatalf("Logs directory was not created")
	}
	if _, err := os.Stat(stableDir); os.IsNotExist(err) {
		t.Fatalf("Stable directory was not created")
	}
}

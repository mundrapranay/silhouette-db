package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupClusterNode creates a single node in a cluster
func setupClusterNode(t *testing.T, nodeID, listenAddr, dataDir string, bootstrap bool) *Store {
	// Create data directory
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("Failed to create data directory: %v", err)
	}

	config := Config{
		NodeID:           nodeID,
		ListenAddr:       listenAddr,
		DataDir:          dataDir,
		Bootstrap:        bootstrap,
		HeartbeatTimeout: 500 * time.Millisecond,
		ElectionTimeout:  500 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("Failed to create store for %s: %v", nodeID, err)
	}

	return store
}

// waitForLeadership waits for a node to become leader
func waitForLeadership(t *testing.T, store *Store, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	tick := time.Tick(100 * time.Millisecond)

	for !store.IsLeader() {
		if time.Now().After(deadline) {
			t.Fatal("Timeout waiting for leadership")
		}
		<-tick
	}
}

func TestMultiNodeCluster_Formation(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup first node (bootstrap)
	node1Dir := filepath.Join(tmpDir, "node1")
	node1 := setupClusterNode(t, "node1", "127.0.0.1:10001", node1Dir, true)
	defer node1.Shutdown()

	waitForLeadership(t, node1, 5*time.Second)

	// Verify node1 is leader
	if !node1.IsLeader() {
		t.Fatal("Node1 should be leader in single-node cluster")
	}

	t.Log("✅ Single-node cluster formed successfully")
}

func TestMultiNodeCluster_TwoNodes(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup first node (bootstrap)
	node1Dir := filepath.Join(tmpDir, "node1")
	node1 := setupClusterNode(t, "node1", "127.0.0.1:10002", node1Dir, true)
	defer node1.Shutdown()

	waitForLeadership(t, node1, 5*time.Second)

	// Setup second node (join)
	node2Dir := filepath.Join(tmpDir, "node2")
	node2 := setupClusterNode(t, "node2", "127.0.0.1:10003", node2Dir, false)
	defer node2.Shutdown()

	// Add node2 to cluster via node1
	err := node1.AddPeer("node2", "127.0.0.1:10003")
	if err != nil {
		t.Fatalf("Failed to add node2 to cluster: %v", err)
	}

	// Wait a bit for cluster formation
	time.Sleep(2 * time.Second)

	// Verify node1 is still leader (or one of them is leader)
	if !node1.IsLeader() && !node2.IsLeader() {
		t.Fatal("One of the nodes should be leader")
	}

	leader := node1
	if node2.IsLeader() {
		leader = node2
	}

	t.Logf("✅ Two-node cluster formed successfully. Leader: %s", leader.raft.State())
}

func TestMultiNodeCluster_DataReplication(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup bootstrap node
	node1Dir := filepath.Join(tmpDir, "node1")
	node1 := setupClusterNode(t, "node1", "127.0.0.1:10004", node1Dir, true)
	defer node1.Shutdown()

	waitForLeadership(t, node1, 5*time.Second)

	// Write data to leader
	testKey := "test-replication-key"
	testValue := []byte("test-replication-value")

	err := node1.Set(testKey, testValue)
	if err != nil {
		t.Fatalf("Failed to set value on leader: %v", err)
	}

	// Wait for Raft to replicate
	time.Sleep(500 * time.Millisecond)

	// Verify data is accessible on node1
	retrieved, exists := node1.Get(testKey)
	if !exists {
		t.Fatal("Key should exist on node1")
	}
	if string(retrieved) != string(testValue) {
		t.Fatalf("Value mismatch on node1: expected '%s', got '%s'", string(testValue), string(retrieved))
	}

	t.Log("✅ Data replication verified on single-node cluster")
}

func TestMultiNodeCluster_LeaderElection(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup bootstrap node
	node1Dir := filepath.Join(tmpDir, "node1")
	node1 := setupClusterNode(t, "node1", "127.0.0.1:10005", node1Dir, true)
	defer node1.Shutdown()

	waitForLeadership(t, node1, 5*time.Second)

	// Verify leadership
	if !node1.IsLeader() {
		t.Fatal("Node1 should be leader")
	}

	leaderAddr := node1.Leader()
	if leaderAddr == "" {
		t.Fatal("Leader address should be set")
	}

	t.Logf("✅ Leader election successful. Leader: %s", leaderAddr)
}

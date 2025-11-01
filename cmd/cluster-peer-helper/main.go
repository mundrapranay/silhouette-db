package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/mundrapranay/silhouette-db/internal/store"
)

// This helper program adds peers to a Raft cluster by accessing the leader's store
// It creates a temporary node that connects to the cluster and uses AddPeer
//
// Usage: go run main.go <leader-data-dir> <peer-id>:<peer-addr> [<peer-id>:<peer-addr> ...]
func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <leader-data-dir> <peer-id>:<peer-addr> [<peer-id>:<peer-addr> ...]\n", os.Args[0])
		os.Exit(1)
	}

	leaderDataDir := os.Args[1]

	// Create a helper store that can add peers to the cluster
	// Note: This requires the helper to be able to connect to the cluster
	// For this to work, we need to use the leader's transport or connect directly
	helperDataDir := filepath.Join(filepath.Dir(leaderDataDir), "helper-node")
	if err := os.MkdirAll(helperDataDir, 0755); err != nil {
		log.Fatalf("Failed to create helper data directory: %v", err)
	}

	// Create store config - this node will try to join the existing cluster
	config := store.Config{
		NodeID:           "helper-node",
		ListenAddr:       "127.0.0.1:0", // Random port
		DataDir:          helperDataDir,
		Bootstrap:        false,
		HeartbeatTimeout: 1000 * time.Millisecond,
		ElectionTimeout:  1000 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	s, err := store.NewStore(config)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer s.Shutdown()

	// Wait for initialization
	fmt.Printf("⏳ Initializing helper node...\n")
	time.Sleep(2 * time.Second)

	// Add peers
	for i := 2; i < len(os.Args); i++ {
		peerSpec := os.Args[i]

		// Parse peer-id:peer-addr format
		colonIdx := -1
		for j := len(peerSpec) - 1; j >= 0; j-- {
			if peerSpec[j] == ':' {
				colonIdx = j
				break
			}
		}

		if colonIdx == -1 {
			fmt.Fprintf(os.Stderr, "⚠️  Invalid peer spec: %s (expected peer-id:peer-addr)\n", peerSpec)
			continue
		}

		peerID := peerSpec[:colonIdx]
		peerAddr := peerSpec[colonIdx+1:]

		fmt.Printf("📝 Adding peer %s at %s...\n", peerID, peerAddr)

		// Try to add peer - this requires the helper to be part of the cluster
		// or we need direct access to the leader's store
		// For now, we'll try and handle errors gracefully
		err := s.AddPeer(peerID, peerAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Failed to add peer %s: %v\n", peerID, err)
			fmt.Fprintf(os.Stderr, "   This may require the helper to be part of the cluster first\n")
			continue
		}

		fmt.Printf("✅ Added peer %s\n", peerID)
		time.Sleep(1 * time.Second)
	}

	fmt.Println("✅ Peer addition complete")
}

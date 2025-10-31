package store

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// Store wraps a Raft instance and provides a clean API for operations.
type Store struct {
	raft *raft.Raft
	fsm  *FSM
}

// Config holds configuration for initializing a Raft store.
type Config struct {
	NodeID           string
	ListenAddr       string
	DataDir          string
	Bootstrap        bool
	PeerAddresses    []string
	HeartbeatTimeout time.Duration
	ElectionTimeout  time.Duration
	CommitTimeout    time.Duration
}

// NewStore creates and initializes a new Raft store.
func NewStore(config Config) (*Store, error) {
	fsm := NewFSM()

	// Create Raft configuration
	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(config.NodeID)
	raftConfig.HeartbeatTimeout = config.HeartbeatTimeout
	raftConfig.ElectionTimeout = config.ElectionTimeout
	raftConfig.CommitTimeout = config.CommitTimeout

	// Create log store
	logStore, err := raftboltdb.NewBoltStore(fmt.Sprintf("%s/logs", config.DataDir))
	if err != nil {
		return nil, fmt.Errorf("failed to create log store: %w", err)
	}

	// Create stable store
	stableStore, err := raftboltdb.NewBoltStore(fmt.Sprintf("%s/stable", config.DataDir))
	if err != nil {
		return nil, fmt.Errorf("failed to create stable store: %w", err)
	}

	// Create snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(config.DataDir, 3, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot store: %w", err)
	}

	// Create transport
	addr, err := net.ResolveTCPAddr("tcp", config.ListenAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve address: %w", err)
	}

	transport, err := raft.NewTCPTransport(config.ListenAddr, addr, 3, 10*time.Second, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Create Raft instance
	r, err := raft.NewRaft(raftConfig, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return nil, fmt.Errorf("failed to create raft: %w", err)
	}

	// Bootstrap if this is the first node
	if config.Bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(config.NodeID),
					Address: raft.ServerAddress(config.ListenAddr),
				},
			},
		}
		r.BootstrapCluster(configuration)
	}

	return &Store{
		raft: r,
		fsm:  fsm,
	}, nil
}

// Set writes a key-value pair to the store via Raft consensus.
func (s *Store) Set(key string, value []byte) error {
	if s.raft.State() != raft.Leader {
		return fmt.Errorf("not the leader")
	}

	cmd := Command{
		Op:    "SET",
		Key:   key,
		Value: value,
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	future := s.raft.Apply(data, 10*time.Second)
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply command: %w", err)
	}

	return nil
}

// Get retrieves a value from the local FSM.
// Note: This reads from the local state, which should be consistent
// after Raft has applied all operations.
func (s *Store) Get(key string) ([]byte, bool) {
	return s.fsm.Get(key)
}

// IsLeader returns whether this node is currently the Raft leader.
func (s *Store) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

// Leader returns the address of the current leader.
func (s *Store) Leader() raft.ServerAddress {
	return s.raft.Leader()
}

// AddPeer adds a new peer to the cluster.
func (s *Store) AddPeer(peerID, peerAddr string) error {
	return s.raft.AddVoter(raft.ServerID(peerID), raft.ServerAddress(peerAddr), 0, 0).Error()
}

// RemovePeer removes a peer from the cluster.
func (s *Store) RemovePeer(peerID string) error {
	return s.raft.RemoveServer(raft.ServerID(peerID), 0, 0).Error()
}

// Shutdown gracefully shuts down the Raft instance.
func (s *Store) Shutdown() error {
	future := s.raft.Shutdown()
	return future.Error()
}

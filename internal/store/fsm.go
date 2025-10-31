package store

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hashicorp/raft"
)

// Command represents a single operation to be applied to the FSM.
type Command struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value []byte `json:"value"`
}

// FSM is our simple key-value store state machine.
// It stores opaque data blobs, which are the OKVS structures.
type FSM struct {
	mu   sync.RWMutex
	data map[string][]byte // The key-value store
}

// NewFSM creates a new FSM instance.
func NewFSM() *FSM {
	return &FSM{
		data: make(map[string][]byte),
	}
}

// Apply applies a Raft log entry to the FSM.
// This is the primary way the FSM is modified.
func (f *FSM) Apply(log *raft.Log) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Deserialize the command from log.Data
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return fmt.Errorf("failed to deserialize command: %w", err)
	}

	switch cmd.Op {
	case "SET":
		f.data[cmd.Key] = cmd.Value
		return nil
	case "DELETE":
		delete(f.data, cmd.Key)
		return nil
	default:
		return fmt.Errorf("unrecognized command op: %s", cmd.Op)
	}
}

// Get retrieves a value from the FSM by key.
func (f *FSM) Get(key string) ([]byte, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	value, exists := f.data[key]
	return value, exists
}

// Snapshot is used to support log compaction. It captures a snapshot
// of the FSM state.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Clone the data map to avoid race conditions
	clone := make(map[string][]byte)
	for k, v := range f.data {
		// Deep copy the value
		valueCopy := make([]byte, len(v))
		copy(valueCopy, v)
		clone[k] = valueCopy
	}

	return &FSMSnapshot{data: clone}, nil
}

// Restore is used to restore the FSM from a snapshot.
func (f *FSM) Restore(rc io.ReadCloser) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Clear existing data
	f.data = make(map[string][]byte)

	// Read and decode the snapshot
	var snapshotData map[string][]byte
	decoder := json.NewDecoder(rc)
	if err := decoder.Decode(&snapshotData); err != nil {
		return fmt.Errorf("failed to decode snapshot: %w", err)
	}

	// Restore the data
	for k, v := range snapshotData {
		valueCopy := make([]byte, len(v))
		copy(valueCopy, v)
		f.data[k] = valueCopy
	}

	return nil
}

// FSMSnapshot represents a snapshot of the FSM state.
type FSMSnapshot struct {
	data map[string][]byte
}

// Persist writes the snapshot to the given sink.
func (s *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	encoder := json.NewEncoder(sink)
	if err := encoder.Encode(s.data); err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}
	return sink.Close()
}

// Release is called when the snapshot is no longer needed.
func (s *FSMSnapshot) Release() {
	// Nothing to release for our simple in-memory snapshot
}

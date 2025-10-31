package store

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/hashicorp/raft"
)

func TestNewFSM(t *testing.T) {
	fsm := NewFSM()
	if fsm == nil {
		t.Fatal("NewFSM returned nil")
	}
	if fsm.data == nil {
		t.Fatal("FSM data map is nil")
	}
	if len(fsm.data) != 0 {
		t.Fatal("FSM should start with empty data")
	}
}

func TestFSM_Apply_SET(t *testing.T) {
	fsm := NewFSM()

	// Create a SET command
	cmd := Command{
		Op:    "SET",
		Key:   "test-key",
		Value: []byte("test-value"),
	}
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	// Create a Raft log entry
	log := &raft.Log{
		Data: data,
	}

	// Apply the command
	result := fsm.Apply(log)
	if result != nil {
		t.Fatalf("Apply returned error: %v", result)
	}

	// Verify the value was stored
	value, exists := fsm.Get("test-key")
	if !exists {
		t.Fatal("Key was not stored")
	}
	if string(value) != "test-value" {
		t.Fatalf("Expected 'test-value', got '%s'", string(value))
	}
}

func TestFSM_Apply_DELETE(t *testing.T) {
	fsm := NewFSM()

	// First set a value
	setCmd := Command{
		Op:    "SET",
		Key:   "test-key",
		Value: []byte("test-value"),
	}
	setData, _ := json.Marshal(setCmd)
	fsm.Apply(&raft.Log{Data: setData})

	// Verify it exists
	if _, exists := fsm.Get("test-key"); !exists {
		t.Fatal("Key should exist before delete")
	}

	// Now delete it
	delCmd := Command{
		Op:  "DELETE",
		Key: "test-key",
	}
	delData, err := json.Marshal(delCmd)
	if err != nil {
		t.Fatalf("Failed to marshal delete command: %v", err)
	}

	result := fsm.Apply(&raft.Log{Data: delData})
	if result != nil {
		t.Fatalf("Delete returned error: %v", result)
	}

	// Verify it's gone
	if _, exists := fsm.Get("test-key"); exists {
		t.Fatal("Key should not exist after delete")
	}
}

func TestFSM_Apply_InvalidOperation(t *testing.T) {
	fsm := NewFSM()

	cmd := Command{
		Op:  "INVALID",
		Key: "test-key",
	}
	data, _ := json.Marshal(cmd)

	result := fsm.Apply(&raft.Log{Data: data})
	if result == nil {
		t.Fatal("Apply should return error for invalid operation")
	}

	err, ok := result.(error)
	if !ok {
		t.Fatal("Result should be an error")
	}
	if err.Error() != "unrecognized command op: INVALID" {
		t.Fatalf("Expected error message about invalid op, got: %v", err)
	}
}

func TestFSM_Get(t *testing.T) {
	fsm := NewFSM()

	// Test getting non-existent key
	_, exists := fsm.Get("non-existent")
	if exists {
		t.Fatal("Non-existent key should not exist")
	}

	// Set a value
	cmd := Command{
		Op:    "SET",
		Key:   "existing-key",
		Value: []byte("existing-value"),
	}
	data, _ := json.Marshal(cmd)
	fsm.Apply(&raft.Log{Data: data})

	// Test getting existing key
	value, exists := fsm.Get("existing-key")
	if !exists {
		t.Fatal("Key should exist")
	}
	if string(value) != "existing-value" {
		t.Fatalf("Expected 'existing-value', got '%s'", string(value))
	}
}

func TestFSM_Snapshot(t *testing.T) {
	fsm := NewFSM()

	// Set some values
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		cmd := Command{
			Op:    "SET",
			Key:   key,
			Value: []byte("value-" + key),
		}
		data, _ := json.Marshal(cmd)
		fsm.Apply(&raft.Log{Data: data})
	}

	// Create snapshot
	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Verify snapshot is not nil
	if snapshot == nil {
		t.Fatal("Snapshot should not be nil")
	}
}

func TestFSMSnapshot_Persist(t *testing.T) {
	fsm := NewFSM()

	// Set some values
	cmd1 := Command{Op: "SET", Key: "key1", Value: []byte("value1")}
	data1, _ := json.Marshal(cmd1)
	fsm.Apply(&raft.Log{Data: data1})

	cmd2 := Command{Op: "SET", Key: "key2", Value: []byte("value2")}
	data2, _ := json.Marshal(cmd2)
	fsm.Apply(&raft.Log{Data: data2})

	// Create snapshot
	snapshot, _ := fsm.Snapshot()

	// Create a buffer to act as the sink
	var buf bytes.Buffer
	sink := &mockSnapshotSink{buf: &buf}

	// Persist the snapshot
	err := snapshot.Persist(sink)
	if err != nil {
		t.Fatalf("Failed to persist snapshot: %v", err)
	}

	// Verify data was written
	if buf.Len() == 0 {
		t.Fatal("Snapshot should have written data")
	}
}

func TestFSM_Restore(t *testing.T) {
	fsm := NewFSM()

	// Create some snapshot data
	snapshotData := map[string][]byte{
		"restored-key1": []byte("restored-value1"),
		"restored-key2": []byte("restored-value2"),
	}

	// Encode to JSON
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	if err := encoder.Encode(snapshotData); err != nil {
		t.Fatalf("Failed to encode snapshot data: %v", err)
	}

	// Restore from snapshot
	reader := bytes.NewReader(buf.Bytes())
	rc := &mockReadCloser{reader: reader}
	err := fsm.Restore(rc)
	if err != nil {
		t.Fatalf("Failed to restore snapshot: %v", err)
	}

	// Verify restored data
	value1, exists1 := fsm.Get("restored-key1")
	if !exists1 || string(value1) != "restored-value1" {
		t.Fatalf("Failed to restore key1: exists=%v, value=%s", exists1, string(value1))
	}

	value2, exists2 := fsm.Get("restored-key2")
	if !exists2 || string(value2) != "restored-value2" {
		t.Fatalf("Failed to restore key2: exists=%v, value=%s", exists2, string(value2))
	}
}

// Helper types for testing

type mockSnapshotSink struct {
	buf *bytes.Buffer
}

func (m *mockSnapshotSink) Write(p []byte) (int, error) {
	return m.buf.Write(p)
}

func (m *mockSnapshotSink) Close() error {
	return nil
}

func (m *mockSnapshotSink) ID() string {
	return "test-snapshot"
}

func (m *mockSnapshotSink) Cancel() error {
	return nil
}

type mockReadCloser struct {
	reader *bytes.Reader
}

func (m *mockReadCloser) Read(p []byte) (int, error) {
	return m.reader.Read(p)
}

func (m *mockReadCloser) Close() error {
	return nil
}

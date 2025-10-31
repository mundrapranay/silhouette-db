package store

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/raft"
)

func BenchmarkFSM_Apply_SET(b *testing.B) {
	fsm := NewFSM()

	cmd := Command{
		Op:    "SET",
		Key:   "bench-key",
		Value: []byte("bench-value"),
	}
	data, _ := json.Marshal(cmd)
	log := &raft.Log{Data: data}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fsm.Apply(log)
	}
}

func BenchmarkFSM_Get(b *testing.B) {
	fsm := NewFSM()

	// Pre-populate
	cmd := Command{Op: "SET", Key: "bench-key", Value: []byte("bench-value")}
	data, _ := json.Marshal(cmd)
	fsm.Apply(&raft.Log{Data: data})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fsm.Get("bench-key")
	}
}

func BenchmarkFSM_Snapshot(b *testing.B) {
	fsm := NewFSM()

	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		cmd := Command{
			Op:    "SET",
			Key:   "key-" + string(rune(i)),
			Value: []byte("value-" + string(rune(i))),
		}
		data, _ := json.Marshal(cmd)
		fsm.Apply(&raft.Log{Data: data})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fsm.Snapshot()
	}
}

func BenchmarkFSM_MultipleSets(b *testing.B) {
	fsm := NewFSM()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := Command{
			Op:    "SET",
			Key:   "key-" + string(rune(i)),
			Value: []byte("value-" + string(rune(i))),
		}
		data, _ := json.Marshal(cmd)
		fsm.Apply(&raft.Log{Data: data})
	}
}

func BenchmarkFSM_ConcurrentReads(b *testing.B) {
	fsm := NewFSM()

	// Pre-populate
	cmd := Command{Op: "SET", Key: "bench-key", Value: []byte("bench-value")}
	data, _ := json.Marshal(cmd)
	fsm.Apply(&raft.Log{Data: data})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			fsm.Get("bench-key")
		}
	})
}

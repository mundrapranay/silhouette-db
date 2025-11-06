# silhouette-db: Complete Architecture and Component Guide

This document provides a comprehensive, self-sufficient guide to the `silhouette-db` framework. It covers the entire codebase, component interactions, workflows, and implementation details. This guide is intended for developers, researchers, and anyone who needs to understand the complete system.

## Table of Contents

1. [Overview](#overview)
2. [System Architecture](#system-architecture)
3. [Component Details](#component-details)
4. [Data Flow and Workflows](#data-flow-and-workflows)
5. [Cryptographic Primitives](#cryptographic-primitives)
6. [Algorithm Framework](#algorithm-framework)
7. [API Reference](#api-reference)
8. [Build System](#build-system)
9. [Deployment and Operations](#deployment-and-operations)

## Overview

### What is silhouette-db?

`silhouette-db` is a fault-tolerant, distributed, and oblivious coordination layer designed for testing Local Edge Differentially Private (LEDP) algorithms. It replaces centralized coordinator models with a distributed, peer-to-peer architecture built upon a custom oblivious key-value store.

### Core Principles

1. **Fault Tolerance**: Uses Raft consensus for crash fault tolerance (CFT)
2. **Oblivious Storage**: OKVS (Oblivious Key-Value Store) hides storage access patterns
3. **Private Queries**: PIR (Private Information Retrieval) ensures query privacy
4. **Round-Based Coordination**: Synchronous rounds for algorithm execution
5. **Distributed Consensus**: All nodes agree on an ordered log of operations

### Key Features

- ✅ **Distributed Consensus**: Raft-based replication across cluster nodes
- ✅ **Oblivious Storage**: RB-OKVS encoding hides which keys are stored
- ✅ **Private Information Retrieval**: FrodoPIR enables private queries
- ✅ **Graph Algorithm Framework**: Round-based synchronous algorithm execution
- ✅ **Local Testing Support**: Partitioned graph files for local development
- ✅ **Production Ready**: Tested and verified with comprehensive test suite

## System Architecture

### High-Level Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         LEDP Workers (Clients)                          │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐                 │
│  │ Worker-0 │  │ Worker-1 │  │ Worker-2 │  │ Worker-N │                 │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘                 │
│       │             │             │             │                       │
│       └─────────────┴─────────────┴─────────────┘                       │
│                          │                                              │
│                    gRPC API (CoordinationService)                       │
└──────────────────────────┼──────────────────────────────────────────────┘
                           │
┌──────────────────────────┼──────────────────────────────────────────────┐
│                  silhouette-db Coordination Layer                       │
│                                                                         │
│  ┌────────────────────────────────────────────────────────────────┐     │
│  │                      gRPC Server Layer                         │     │
│  │  - StartRound: Initialize synchronous rounds                   │     │
│  │  - PublishValues: Aggregate worker contributions               │     │
│  │  - GetValue: Process PIR queries                               │     │
│  │  - GetBaseParams: Distribute PIR parameters                    │     │
│  │  - GetKeyMapping: Provide key-to-index mappings                │     │
│  └────────────────┬─────────────────────────────────────────────--┘     │
│                   │                                                     │
│       ┌───────────┴───────────┐                                         │
│       │                       │                                         │
│  ┌────▼─────┐        ┌────────▼────────┐                                │
│  │ Round    │        │  Cryptographic  │                                │
│  │ Manager  │        │     Layer       │                                │
│  │          │        │                 │                                │
│  │ Tracks   │        │ ┌─────────────┐ │                                │
│  │ worker   │        │ │ Storage     │ │                                │
│  │ state    │        │ │ Backends    │ │                                │
│  │ per      │        │ │             │ │                                │
│  │ round    │        │ │ • OKVS      │ │                                │
│  └────┬─────┘        │ │   (Oblivious│ │                                │
│       │              │ │    Storage) │ │                                │
│       │              │ │ • KVS       │ │                                │
│       │              │ │   (Simple   │ │                                │
│       │              │ │    Storage) │ │                                │
│       │              │ └─────────────┘ │                                │
│       │              │                 │                                │
│       │              │ ┌─────────────┐ │                                │ 
│       │              │ │ FrodoPIR    │ │                                │
│       │              │ │ Server/     │ │                                │
│       │              │ │ Client      │ │                                │
│       │              │ │ (Private    │ │                                │
│       │              │ │  Queries)   │ │                                │
│       │              │ └─────────────┘ │                                │
│       └──────────────┴─────────┬───────┘                                │
│                                │                                        │
│                       ┌────────▼────────┐                               │
│                       │   Raft Layer    │                               │
│                       │                 │                               │
│                       │ ┌─────────────┐ │                               │
│                       │ │ FSM         │ │                               │
│                       │ │ (Key-Value  │ │                               │
│                       │ │  Store)     │ │                               │
│                       │ └─────────────┘ │                               │
│                       │                 │                               │
│                       │ ┌─────────────┐ │                               │
│                       │ │ Raft        │ │                               │
│                       │ │ Consensus   │ │                               │
│                       │ │ (Log Repli  │ │                               │
│                       │ │  cation)    │ │                               │
│                       │ └─────────────┘ │                               │
│                       └────────┬────────┘                               │
└───────────────────────────────┼────────────────────────────────────────-┘
                                │
                    ┌───────────┴───────────┐
                    │                       │
              ┌─────▼─────┐         ┌──────▼─────┐
              │ Node 1    │         │ Node N     │
              │ (Leader)  │◄───────►│ (Follower) │
              └───────────┘         └────────────┘
```

### Component Interaction Flow

The system operates in synchronous rounds:

1. **Round Initialization**: Leader accepts `StartRound` RPC
2. **Worker Publishing**: Workers send `PublishValues` to leader
3. **Aggregation**: Leader collects all worker contributions
4. **Oblivious Encoding**: OKVS encodes aggregated pairs (if ≥100 pairs)
5. **PIR Setup**: FrodoPIR server created from OKVS-decoded values
6. **Storage**: OKVS blob stored in Raft cluster (replicated)
7. **Query Phase**: Workers query via PIR (`GetValue`)
8. **Response**: Server processes PIR query, returns oblivious response

## Component Details

### 1. Raft Layer (`internal/store/`)

The Raft layer provides distributed consensus and fault tolerance.

#### 1.1 Finite State Machine (`internal/store/fsm.go`)

**Purpose**: Implements the replicated state machine that stores oblivious data structures.

**Structure**:
```go
type FSM struct {
    mu   sync.RWMutex
    data map[string][]byte  // Key-value store for OKVS blobs
}
```

**Key Methods**:
- `Apply(log *raft.Log)`: Applies Raft log entries to the state machine
  - Handles `SET` and `DELETE` operations
  - Stores OKVS-encoded blobs or raw pairs
- `Get(key string)`: Retrieves values from the state machine
- `Snapshot()`: Creates snapshots for log compaction
- `Restore(rc io.ReadCloser)`: Restores FSM from snapshot

**Data Format**:
- Keys: `"round_{roundID}_results"` (e.g., `"round_1_results"`)
- Values: OKVS-encoded blobs (if ≥100 pairs) or serialized raw pairs

#### 1.2 Store Wrapper (`internal/store/store.go`)

**Purpose**: Wraps HashiCorp Raft library and provides a clean API.

**Structure**:
```go
type Store struct {
    raft *raft.Raft
    fsm  *FSM
}
```

**Key Methods**:
- `NewStore(config Config)`: Initializes Raft node with FSM, log store, stable store, snapshot store, and transport
- `Set(key, value)`: Proposes write operation to Raft cluster
- `Get(key)`: Reads value from local FSM (eventually consistent)
- `IsLeader()`: Checks if this node is the Raft leader
- `Shutdown()`: Gracefully shuts down Raft node

**Initialization Flow**:
1. Create FSM instance
2. Configure Raft parameters (heartbeat timeout, election timeout, etc.)
3. Create log store (BoltDB for persistence)
4. Create stable store (BoltDB for Raft metadata)
5. Create snapshot store (file-based for log compaction)
6. Create TCP transport (for Raft node communication)
7. Bootstrap cluster if first node, or join existing cluster
8. Return Store instance

**Configuration**:
- `NodeID`: Unique identifier for this node
- `ListenAddr`: Address for Raft P2P communication (e.g., `127.0.0.1:8080`)
- `DataDir`: Directory for Raft logs, snapshots, stable data
- `Bootstrap`: Whether to bootstrap a new cluster
- `HeartbeatTimeout`: Leader heartbeat interval
- `ElectionTimeout`: Follower election timeout

### 2. Cryptographic Layer (`internal/crypto/`)

The cryptographic layer provides storage backends (OKVS or KVS) and private queries (PIR).

#### 2.1 Storage Backends

**Available Backends**:
- **OKVS** (`internal/crypto/okvs_impl.go`): Oblivious key-value store (requires 100+ pairs)
- **KVS** (`internal/crypto/kvs.go`): Simple key-value store (works with any number of pairs)

**Selection**: Server chooses backend based on `--storage-backend` flag:
- `--storage-backend=okvs`: Use OKVS backend (default)
- `--storage-backend=kvs`: Use KVS backend
- Default: OKVS if not specified

**Backend Requirements**:
- **OKVS**: Requires `len(allPairs) >= 100` (enforced at publish time)
- **KVS**: Works with any number of pairs (no minimum)

#### 2.2 OKVS Implementation (`internal/crypto/okvs_impl.go`)

**Purpose**: Encodes key-value pairs into oblivious data structures.

**Algorithm**: RB-OKVS (Random Band Matrix OKVS)

**Interface**:
```go
type OKVSEncoder interface {
    Encode(pairs map[string][]byte) ([]byte, error)
}

type OKVSDecoder interface {
    Decode(blob []byte, key string) ([]byte, error)
}
```

**Implementation**: `RBOKVSEncoder` and `RBOKVSDecoder`

**Requirements**:
- Minimum 100 key-value pairs for reliable operation
- Values must be exactly 8 bytes (float64, little-endian)
- Keys are hashed to 8-byte `OkvsKey` using BLAKE2b512
- Encoding overhead: ~10-20% (epsilon = 0.1)
- CGO required (Rust FFI integration)

**FFI Integration**:
- Uses `cgo` to call Rust FFI library (`librbokvsffi.a`)
- FFI wrapper: `third_party/rb-okvs-ffi/`
- Source library: `third_party/rb-okvs/` (git submodule)

**Memory Management**:
- All C-allocated memory properly freed
- Go slices copied to avoid memory issues
- Thread-safe (no shared state)

#### 2.3 KVS Implementation (`internal/crypto/kvs.go`)

**Purpose**: Provides simple, fast storage without oblivious properties.

**Interface**: Implements `OKVSEncoder` and `OKVSDecoder` interfaces (same as OKVS).

**Implementation**: `KVSEncoder` and `KVSDecoder`

**Properties**:
- Works with any number of pairs (no minimum requirement)
- Values can be any size (not limited to 8 bytes)
- Fast encoding/decoding (JSON serialization)
- No CGO required (pure Go implementation)
- No oblivious properties (storage format reveals keys)

**Serialization Format**:
- JSON map with base64-encoded values
- Keys stored as strings
- Values stored as base64-encoded strings

**When to Use**:
- Testing and development
- Algorithms with <100 pairs
- Non-privacy-sensitive applications
- Performance-critical scenarios

#### 2.4 PIR Implementation (`internal/crypto/pir.go`)

**Purpose**: Enables private queries without revealing which key is requested.

**Algorithm**: FrodoPIR

**Structure**:
```go
// Server-side
type FrodoPIRServer struct {
    shard         C.struct_FrodoPIRShard  // Rust FFI shard
    lweDim        uintptr                  // LWE dimension
    m             uintptr                  // Number of elements
    elemSize      uintptr                  // Element size in bits
    plaintextBits uintptr                  // Plaintext bits
}

// Client-side
type FrodoPIRClient struct {
    params        C.struct_FrodoPIRQueryParams  // Query parameters
    keyToIndex    map[string]int                 // Key → index mapping
    elemSize      uintptr                        // Element size
}
```

**Key Methods**:
- `NewFrodoPIRServer(pairs, lweDim, elemSize, plaintextBits)`: Creates server from key-value pairs
- `ProcessQuery(db, query)`: Processes PIR query, returns oblivious response
- `NewFrodoPIRClient(baseParams, keyToIndex)`: Creates client for query generation
- `GenerateQuery(key)`: Generates PIR query for a key (retries on overflow)
- `DecodeResponse(response, queryParams)`: Decodes server response

**Parameters**:
- `lweDim`: LWE dimension (512 for small, 1024 for medium, 1572 for large databases)
- `elemSize`: Element size in bits (typically 512-8192 bits)
- `plaintextBits`: Plaintext bits per matrix element (10 for most cases)

**FFI Integration**:
- Uses `cgo` to call Rust FFI library (`libfrodopirffi.a`)
- FFI wrapper: `third_party/frodo-pir-ffi/`
- Source library: `third_party/frodo-pir/` (git submodule)

**Error Handling**:
- `OverflownAdd` error: Probabilistic overflow during query generation (automatic retry)
- `QueryParamsReused`: Non-retryable error (QueryParams used more than once)
- Automatic retry logic (up to 3 attempts) for overflow errors

### 3. gRPC Server Layer (`internal/server/server.go`)

The server layer orchestrates Raft, OKVS, and PIR components.

#### 3.1 Server Structure

```go
type Server struct {
    store          *store.Store
    okvsEncoder    crypto.OKVSEncoder  // OKVSEncoder interface (OKVS or KVS)
    storageBackend string              // "okvs" or "kvs"

    // Round management
    roundsMu        sync.RWMutex
    roundData       map[uint64]*roundState
    expectedWorkers map[uint64]int32

    // FrodoPIR servers per round
    pirServers      map[uint64]*crypto.FrodoPIRServer
    roundBaseParams map[uint64][]byte
    roundKeyMapping map[uint64]map[string]int

    // Storage per round (OKVS or KVS)
    storageBlobs    map[uint64][]byte             // Encoded blobs (OKVS or KVS)
    storageDecoders map[uint64]crypto.OKVSDecoder // Decoders (interface type)
}
```

**Round State**:
```go
type roundState struct {
    mu         sync.Mutex
    workerData map[string][]apiv1.KeyValuePair  // worker_id → pairs
    complete   bool
}
```

#### 3.2 RPC Handlers

**StartRound**:
- Only leader can start rounds
- Initializes round state and tracks expected workers
- Returns success when round is ready

**PublishValues**:
- Only leader accepts publishes
- Records worker contributions in `roundState.workerData[workerID]`
- Tracks submission count
- When all workers have published (`len(roundState.workerData) >= expected`):
  
  **Aggregation Phase**:
  1. Collects all pairs from all workers
  2. Deduplicates by key (last write wins if duplicates)
  3. Creates `allPairs` map
  
  **Key-to-Index Mapping**:
  4. Extracts all keys, sorts alphabetically for determinism
  5. Creates `keyToIndex` mapping: `key → index` (0-based)
  
  **Empty Round Handling**:
  6. If `len(allPairs) == 0`:
     - Stores empty data in Raft
     - Marks round complete (synchronization-only round)
     - Skips PIR server creation
     - Returns success
  
  **Storage Backend Selection**:
  7. Uses backend specified by `--storage-backend` flag (default: "okvs")
     - If `storageBackend == "okvs"`:
       - Requires `len(allPairs) >= 100` (RB-OKVS requirement)
       - If <100 pairs, returns error: "OKVS requires at least 100 pairs"
     - If `storageBackend == "kvs"`:
       - Works with any number of pairs (no minimum)
  
  **Storage Encoding**:
  8. Encodes via `okvsEncoder.Encode(allPairs)` → `storageBlob`
     - If OKVS backend: Uses `RBOKVSEncoder` (Rust FFI)
     - If KVS backend: Uses `KVSEncoder` (pure Go, JSON serialization)
  
  **Storage Decoder Creation**:
  9. Creates decoder based on backend:
     - If OKVS: `storageDecoder = NewRBOKVSDecoder(storageBlob)`
     - If KVS: `storageDecoder = NewKVSDecoder(storageBlob)`
  
  **PIR Database Preparation**:
  10. Decodes all values from storage to create PIR database:
      - For each key in sorted order:
        - `decodedValue = storageDecoder.Decode(storageBlob, key)`
      - Creates `pirPairs` map with decoded values
      - Note: Values are same as original, but decoded from storage backend
  
  **PIR Server Creation**:
  11. Calculates `elemSize` (max value size, rounded to power of 2)
  12. Converts `pirPairs` to base64-encoded strings array
  13. Creates FrodoPIR server: `NewFrodoPIRServer(pirPairs, lweDim, elemSizeBits, plaintextBits)`
  14. Extracts BaseParams (serialized)
  
  **Metadata Storage**:
  15. Stores in server's per-round maps:
      - `pirServers[roundID]` = PIR server
      - `roundBaseParams[roundID]` = BaseParams
      - `roundKeyMapping[roundID]` = keyToIndex
      - `storageBlobs[roundID]` = storage blob (OKVS or KVS)
      - `storageDecoders[roundID]` = storage decoder (interface type)
  
  **Persistent Storage**:
  16. Stores in Raft: `store.Set("round_{roundID}_results", storageBlob)`
     - Storage blob format depends on backend:
       - OKVS: RB-OKVS encoded blob
       - KVS: JSON-serialized map with base64-encoded values
  17. Raft replicates to all nodes
  18. All nodes' FSMs store the data
  
  **Completion**:
  19. Marks `roundState.complete = true`
  20. Returns success to all workers

**GetValue**:
- Only leader processes queries
- Verifies round exists and has data
- Uses round's FrodoPIR server to process query
- Returns opaque PIR response

**GetBaseParams**:
- Returns serialized BaseParams for a round
- Clients use this to initialize FrodoPIR clients

**GetKeyMapping**:
- Returns key-to-index mapping for a round
- Clients use this to map keys to database indices

### 4. Client Library (`pkg/client/client.go`)

The client library provides a Go interface for workers to interact with silhouette-db.

#### 4.1 Client Structure

```go
type Client struct {
    conn        *grpc.ClientConn
    service     apiv1.CoordinationServiceClient
    pirClients  map[uint64]PIRClient           // Per-round PIR clients
    keyMappings map[uint64]map[string]int      // Per-round key mappings
    mu          sync.RWMutex                   // Thread-safe access
}
```

**Key Features**:
- Per-round PIR clients (prevents race conditions)
- Thread-safe initialization (double-check locking)
- Automatic PIR client initialization on first query

#### 4.2 Client Methods

**StartRound(roundID, expectedWorkers)**:
- Sends `StartRound` RPC to server
- Waits for round to be initialized
- Only leader can start rounds

**PublishValues(roundID, workerID, pairs)**:
- Converts `map[string][]byte` to `[]*apiv1.KeyValuePair`
- Sends `PublishValues` RPC to server
- Pairs are aggregated with other workers
- Returns when all workers have published

**GetValue(roundID, key)**:
- Checks if PIR client exists for `roundID`
- If not, automatically calls `InitializePIRClient(roundID)`
- Generates PIR query for `key` using per-round client
- Sends query to server via `GetValue` RPC
- Decodes PIR response using query parameters
- Returns decoded value

**InitializePIRClient(roundID)**:
- Thread-safe initialization (double-check locking)
- Fetches BaseParams via `GetBaseParams` RPC
- Fetches key mapping via `GetKeyMapping` RPC
- Creates FrodoPIR client using fetched parameters
- Stores client and mapping in per-round maps
- Handles concurrent initialization (one goroutine wins)

**Close()**:
- Closes all per-round PIR clients
- Closes gRPC connection
- Thread-safe cleanup

### 5. Algorithm Framework (`algorithms/`)

The algorithm framework enables round-based synchronous graph algorithm execution.

#### 5.1 Algorithm Interface (`algorithms/common/algorithm.go`)

```go
type GraphAlgorithm interface {
    Name() string
    Type() AlgorithmType  // "exact" or "ledp"
    Initialize(ctx, graphData, config) error
    Execute(ctx, client, numRounds) (*AlgorithmResult, error)
    GetRoundData(roundID) *RoundData
    ProcessRound(roundID, results) error
    GetResult() *AlgorithmResult
}
```

#### 5.2 Algorithm Runner (`cmd/algorithm-runner/main.go`)

**Flow**:
1. Loads configuration from YAML file
2. Gets algorithm instance from registry
3. Loads graph data (with local testing support)
4. Initializes algorithm with graph and config
5. Connects to silhouette-db server
6. Executes algorithm for specified rounds
7. Prints results

**Configuration Format**:
- Algorithm name and type
- Server address
- Worker configuration (ID, count, vertex assignment)
- Graph configuration (format, file path, local testing mode)
- Algorithm-specific parameters

#### 5.3 Graph Loading (`algorithms/common/graph.go`)

**Local Testing Mode** (`local_testing: true`):
- Loads from partitioned files: `data/{worker_index+1}.txt`
- Worker-0 → `data/1.txt`
- Worker-1 → `data/2.txt`
- Worker-N → `data/{N+1}.txt`

**Deployment Mode** (`local_testing: false`):
- Loads from same file path (all workers use same file)
- Each server/worker has its own copy of the file

**Vertex Assignment**:
- Read from config if provided (auto-generated by graph script)
- Otherwise computed deterministically (round-robin: `vertexID % numWorkers`)
- Ensures consistency across workers

## Data Flow and Workflows

### Complete Round Workflow Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Round Execution Workflow                         │
└─────────────────────────────────────────────────────────────────────┘

Phase 1: Round Initialization
────────────────────────────────
Worker-0              Worker-1              Worker-2              Leader
    │                    │                    │                    │
    │ StartRound(R=1, W=3)                   │                    │
    ├────────────────────────────────────────┼───────────────────►│
    │                                          │                    │
    │                    StartRound(R=1, W=3)  │                    │
    │                    ├─────────────────────┼───────────────────►│
    │                    │                     │                    │
    │                    │          StartRound(R=1, W=3)            │
    │                    │          ├──────────┼───────────────────►│
    │                    │          │           │                    │
    │                    │          │           │                    │
    │                    │          │           │  Initialize round  │
    │                    │          │           │  state, expected=3 │
    │                    │          │           │◄───────────────────┤
    │                    │          │◄───────────────────────────────┤
    │                    │◄─────────────────────────────────────────┤
    │◄───────────────────────────────────────────────────────────────┤
    │ Success            │ Success            │ Success             │

Phase 2: Worker Publishing (Aggregation)
─────────────────────────────────────────
Worker-0              Worker-1              Worker-2              Leader
    │                    │                    │                    │
    │ pairs:             │                    │                    │
    │ {k1:v1, k2:v2}     │                    │                    │
    │ PublishValues(R=1) │                    │                    │
    ├─────────────────────────────────────────┼───────────────────►│
    │                    │                    │                    │
    │                    │ pairs:             │                    │
    │                    │ {k3:v3, k4:v4}     │                    │
    │                    │ PublishValues(R=1) │                    │
    │                    ├─────────────────────┼───────────────────►│
    │                    │                    │                    │
    │                    │                    │ pairs:             │
    │                    │                    │ {k5:v5}            │
    │                    │                    │ PublishValues(R=1) │
    │                    │                    ├─────────────────────►│
    │                    │                    │                    │
    │                    │                    │  All workers submitted│
    │                    │                    │                    │
    │                    │                    │  Aggregation:       │
    │                    │                    │  {k1:v1, k2:v2,    │
    │                    │                    │   k3:v3, k4:v4,     │
    │                    │                    │   k5:v5}            │
    │                    │                    │                    │
    │                    │                    │  OKVS Encode?      │
    │                    │                    │  (if ≥100 pairs)   │
    │                    │                    │                    │
    │                    │                    │  Create PIR Server │
    │                    │                    │                    │
    │                    │                    │  Store in Raft     │
    │                    │                    │                    │
    │                    │                    │◄───────────────────┤
    │                    │◄───────────────────────────────────────┤
    │◄───────────────────────────────────────────────────────────────┤
    │ Success            │ Success            │ Success             │

Phase 3: Query Phase (PIR)
───────────────────────────
Worker-0              Worker-1              Worker-2              Leader
    │                    │                    │                    │
    │ Needs value for k3  │                    │                    │
    │                    │                    │                    │
    │ GetValue(R=1, "k3") │                    │                    │
    ├─────────────────────────────────────────┼───────────────────►│
    │                    │                    │                    │
    │  Check PIR client  │                    │                    │
    │  (not initialized)  │                    │                    │
    │                    │                    │                    │
    │  InitializePIRClient(R=1)                │                    │
    │                    │                    │                    │
    │  GetBaseParams(R=1) │                    │                    │
    ├─────────────────────────────────────────┼───────────────────►│
    │                    │                    │                    │
    │  GetKeyMapping(R=1) │                    │                    │
    ├─────────────────────────────────────────┼───────────────────►│
    │                    │                    │                    │
    │  Create PIR Client │                    │                    │
    │                    │                    │                    │
    │  GenerateQuery(k3) │                    │                    │
    │  (PIR query bytes) │                    │                    │
    │                    │                    │                    │
    │  GetValue(R=1, query)                   │                    │
    ├─────────────────────────────────────────┼───────────────────►│
    │                    │                    │                    │
    │                    │                    │  Process query    │
    │                    │                    │  via PIR server    │
    │                    │                    │                    │
    │                    │                    │◄───────────────────┤
    │                    │                    │  PIR response      │
    │◄───────────────────────────────────────────────────────────────┤
    │  Decode response   │                    │                    │
    │  Value: v3         │                    │                    │
```

### OKVS + PIR Integration Flow (≥100 pairs)

```
┌─────────────────────────────────────────────────────────────────────┐
│              OKVS + PIR Integration Workflow (≥100 pairs)           │
└─────────────────────────────────────────────────────────────────────┘

Step 1: Worker Publishing and Aggregation
──────────────────────────────────────────
All Workers → Leader
    {k1:v1, k2:v2, ..., k150:v150}
            ↓
    Leader aggregates all pairs in roundState.workerData
            ↓
    allPairs = {k1:v1, k2:v2, ..., k150:v150}
            ↓
    (Key deduplication: last write wins)

Step 2: Key-to-Index Mapping
─────────────────────────────
allPairs
            ↓
    Extract all keys: [k1, k2, ..., k150]
            ↓
    Sort keys deterministically (alphabetical)
            ↓
    Create mapping:
    keyToIndex = {
        k1: 0,
        k2: 1,
        ...
        k150: 149
    }

Step 3: Storage Backend Selection
───────────────────────────────────
Backend determined by --storage-backend flag (default: "okvs")
            ↓
    If storageBackend == "okvs":
        Check: len(allPairs) >= 100?
        - YES → Use OKVS encoding
        - NO → Error: "OKVS requires at least 100 pairs"
    If storageBackend == "kvs":
        Use KVS encoding (any number of pairs)

Step 4: Storage Encoding
─────────────────────────
allPairs (map[string][]byte)
            ↓
    If OKVS backend:
        Values: All must be 8 bytes (float64, little-endian)
        RBOKVSEncoder.Encode(allPairs)
        Process:
        1. Hash keys to 8-byte OkvsKey (BLAKE2b512)
        2. Convert values to float64
        3. Encode via RB-OKVS algorithm
        → OKVS Blob (opaque byte slice)
        Size: ~1.1-1.2x original size
        Properties: Oblivious (hides which keys)
    
    If KVS backend:
        KVSEncoder.Encode(allPairs)
        Process:
        1. Serialize as JSON map
        2. Base64-encode values
        → KVS Blob (JSON-serialized map)
        Properties: Fast, but reveals keys

Step 5: Storage Decoding for PIR Database
───────────────────────────────────────────
Storage Blob (OKVS or KVS)
            ↓
    If OKVS: storageDecoder = NewRBOKVSDecoder(storageBlob)
    If KVS: storageDecoder = NewKVSDecoder(storageBlob)
            ↓
    For each key in sorted order:
        decodedValue = storageDecoder.Decode(storageBlob, key)
            ↓
    pirPairs = {k1:v1_decoded, k2:v2_decoded, ..., k150:v150_decoded}
            ↓
    Note: Values are same as original, but decoded from storage backend
    This ensures PIR operates on storage-encoded data

Step 6: PIR Server Creation
───────────────────────────
pirPairs (map[string][]byte)
            ↓
    Convert to base64-encoded strings:
    1. Sort keys (must match keyToIndex)
    2. For each value:
       - Pad/truncate to elemSize bytes
       - Encode as base64 string
            ↓
    dbElements = [
        base64(v1_padded),
        base64(v2_padded),
        ...
        base64(v150_padded)
    ]
            ↓
    NewFrodoPIRServer(
        pairs: pirPairs,
        lweDim: 512,
        elemSizeBits: 8192,
        plaintextBits: 10
    )
            ↓
    Creates:
    - FrodoPIR Shard (in-memory, for query processing)
    - BaseParams (serialized, for client distribution)

Step 7: Metadata Storage
─────────────────────────
Server stores (in-memory, per round):
    - pirServers[roundID] = FrodoPIR server shard
    - roundBaseParams[roundID] = BaseParams (bytes)
    - roundKeyMapping[roundID] = keyToIndex map
    - storageBlobs[roundID] = Storage blob (OKVS or KVS)
    - storageDecoders[roundID] = Storage decoder (interface type)

Step 8: Persistent Storage
───────────────────────────
Storage Blob (OKVS or KVS)
            ↓
    Store.Set("round_{id}_results", storageBlob)
    Format depends on backend:
    - OKVS: RB-OKVS encoded blob
    - KVS: JSON-serialized map with base64-encoded values
            ↓
    Raft proposes command:
    {
        Op: "SET",
        Key: "round_1_results",
        Value: okvsBlob
    }
            ↓
    Raft replicates to all nodes
            ↓
    Each node's FSM applies:
    fsm.data["round_1_results"] = okvsBlob
            ↓
    Data persisted and replicated

Step 9: Query Processing (Client-side)
────────────────────────────────────────
Client wants value for key "k50"
            ↓
    1. GetValue(R=1, "k50")
            ↓
    2. Check if PIR client initialized for R=1
       (If not, InitializePIRClient)
            ↓
    3. InitializePIRClient(R=1):
       a. GetBaseParams(R=1) → baseParams
       b. GetKeyMapping(R=1) → keyToIndex
       c. NewFrodoPIRClient(baseParams, keyToIndex)
            ↓
    4. GenerateQuery("k50"):
       a. Lookup index: keyToIndex["k50"] = 49
       b. Generate PIR query for index 49
       c. Returns: (queryBytes, queryParams)
            ↓
    5. Send GetValue(R=1, queryBytes) to server

Step 10: Query Processing (Server-side)
────────────────────────────────────────
Server receives GetValue(R=1, queryBytes)
            ↓
    1. Lookup PIR server: pirServers[R=1]
            ↓
    2. ProcessQuery(nil, queryBytes):
       a. Server processes query over entire database
       b. Server learns nothing about index 49
       c. Returns encrypted response
            ↓
    3. Response sent to client

Step 11: Response Decoding (Client-side)
─────────────────────────────────────────
Client receives PIR response
            ↓
    DecodeResponse(response, queryParams)
            ↓
    Decrypts response using queryParams
            ↓
    Returns: v50 (value for k50)
            ↓
    Note: Value retrieved obliviously
```

### Direct PIR Flow (<100 pairs)

```
┌─────────────────────────────────────────────────────────────────────┐
│              Direct PIR Flow (No OKVS, <100 pairs)                  │
└─────────────────────────────────────────────────────────────────────┘

All Workers → Leader
    {k1:v1, k2:v2, ..., k50:v50}
            ↓
    Aggregation: allPairs = {k1:v1, ..., k50:v50}
            ↓
    Check: len(allPairs) = 50 < 100
            ↓
    Skip OKVS encoding
            ↓
    Use raw pairs directly for PIR:
    pirPairs = allPairs
            ↓
    Create FrodoPIR server from raw pairs
            ↓
    Store raw pairs (serialized) in Raft:
    "round_{id}_results" = serializedPairs
            ↓
    Queries work same as OKVS flow (but without OKVS layer)
```

### Empty Round Handling

When all workers publish empty pairs:
1. Round is initialized normally
2. Aggregation yields empty `allPairs`
3. Server detects empty round
4. Skips PIR server creation
5. Stores empty data in Raft
6. Marks round as complete (synchronization-only round)

### KVS Backend Flow (<100 pairs or explicit selection)

When KVS backend is selected (via `--storage-backend=kvs` or <100 pairs with OKVS):
1. KVS encoding used (JSON serialization with base64-encoded values)
2. Decoded pairs used for PIR database
3. PIR server created from decoded pairs
4. KVS blob (JSON-serialized) stored in Raft

## Cryptographic Primitives

### Storage Backends

`silhouette-db` supports two storage backends for encoding key-value pairs:

1. **OKVS (Oblivious Key-Value Store)**: Provides oblivious storage properties
2. **KVS (Simple Key-Value Store)**: Fast, simple storage without oblivious properties

Both backends work seamlessly with PIR (Private Information Retrieval) for query privacy, but they differ in their storage encoding and privacy guarantees.

#### Storage Backend Comparison

| Feature | OKVS | KVS |
|---------|------|-----|
| **Oblivious Storage** | ✅ Yes | ❌ No |
| **Minimum Pairs** | 100+ pairs required | Any number |
| **Encoding Overhead** | ~10-20% | None (just JSON serialization) |
| **Performance** | Slower (encoding/decoding) | Faster (direct map lookup) |
| **Use Case** | Privacy-sensitive applications | Testing, development, non-private algorithms |
| **CGO Required** | ✅ Yes (Rust FFI) | ❌ No |
| **Storage Format** | RB-OKVS encoded blob | JSON-serialized map |

#### OKVS (Oblivious Key-Value Store)

**Purpose**: Hide which keys are stored in the database.

**Algorithm**: RB-OKVS (Random Band Matrix)

**How It Works**:
1. Takes a set of `(key, value)` pairs
2. Encodes them into a single compact blob
3. The blob reveals no information about which keys were encoded
4. Any key can be decoded from the blob (even keys not originally encoded)

**Properties**:
- **Obliviousness**: The blob reveals nothing about stored keys
- **Decodability**: Any key can be decoded (with high probability)
- **Compactness**: Size is ~1.1-1.2x the original data size
- **Privacy**: Hides which keys are stored in the database

**Implementation Details**:
- Uses Rust FFI library (`rb-okvs`)
- Keys hashed to 8-byte `OkvsKey` via BLAKE2b512
- Values must be 8 bytes (float64, little-endian)
- Minimum 100 pairs for reliable operation
- Encoding rate: ~10-20% overhead
- CGO required (Rust FFI integration)

**When to Use**:
- ✅ Privacy-sensitive applications requiring oblivious storage
- ✅ Production deployments where storage privacy is important
- ✅ Algorithms with 100+ key-value pairs per round
- ✅ When you need to hide which keys are stored

#### KVS (Simple Key-Value Store)

**Purpose**: Fast, simple storage without oblivious properties.

**How It Works**:
1. Takes a set of `(key, value)` pairs
2. Serializes them as JSON with base64-encoded values
3. Stores the serialized blob
4. Decodes by deserializing and looking up keys

**Properties**:
- **Speed**: Fast encoding/decoding (just JSON serialization)
- **Flexibility**: Works with any number of pairs and value sizes
- **Simplicity**: Easy to understand and debug
- **No Obliviousness**: Storage format reveals which keys are stored

**Implementation Details**:
- Pure Go implementation (no CGO)
- JSON map with base64-encoded values
- O(1) lookup after deserialization
- No minimum pair requirement
- No encoding overhead

**When to Use**:
- ✅ Testing and development
- ✅ Algorithms with fewer than 100 pairs
- ✅ Non-privacy-sensitive applications
- ✅ When performance is critical
- ✅ When CGO is not available or desired
- ✅ Exact algorithms (non-private)

#### Choosing the Right Backend

**Decision Tree**:
```
Do you need oblivious storage?
├─ Yes → Use OKVS (if you have 100+ pairs)
│         └─ If <100 pairs → Use KVS (OKVS not available)
└─ No → Use KVS (faster, simpler)
```

**Recommendations**:
1. **Privacy-sensitive production**: Use OKVS
2. **Testing/development**: Use KVS
3. **<100 pairs**: Use KVS (OKVS requires 100+)
4. **Performance-critical**: Use KVS
5. **No CGO available**: Use KVS

**Server Configuration**:
- Command-line flag: `--storage-backend=okvs` or `--storage-backend=kvs`
- Default: OKVS (if not specified)
- Environment variable: `STORAGE_BACKEND=kvs` (for test scripts)

See [Storage Backends Guide](./storage-backends.md) for detailed comparison, usage examples, and troubleshooting.

### PIR (Private Information Retrieval)

**Purpose**: Enable queries without revealing which key is requested.

**Algorithm**: FrodoPIR

**How It Works**:
1. Client generates encrypted query for a specific database index
2. Server processes query over entire database
3. Server learns nothing about which index was queried
4. Client decrypts response to get desired value

**Properties**:
- **Query Privacy**: Server cannot determine which item was queried
- **Correctness**: Client receives correct value
- **Efficiency**: Sub-second response times for practical database sizes

**Implementation Details**:
- Uses Rust FFI library (`frodo-pir`)
- LWE-based cryptography (post-quantum secure)
- Database elements are base64-encoded strings
- Query generation may fail with overflow (automatic retry)
- Parameters configurable (lweDim, elemSize, plaintextBits)

### Combined Privacy

**OKVS + PIR**:
1. **Storage Privacy**: ✅ OKVS hides which keys are stored
2. **Query Privacy**: ✅ PIR hides which key is queried
3. **Complete Obliviousness**: Neither storage patterns nor query patterns are revealed

**KVS + PIR**:
1. **Storage Privacy**: ❌ Not provided (storage format reveals keys)
2. **Query Privacy**: ✅ PIR hides which key is queried
3. **Partial Privacy**: Only query patterns are hidden, not storage patterns

**Recommendation**: Use OKVS + PIR for complete privacy, or KVS + PIR when storage privacy is not required.

## Algorithm Framework

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Algorithm Execution                          │
└─────────────────────────────────────────────────────────────────┘

Algorithm Runner (main.go)
    │
    ├─► Load Config (YAML)
    │   ├─► Algorithm name/type
    │   ├─► Server address
    │   ├─► Worker config
    │   └─► Graph config
    │
    ├─► Get Algorithm (from registry)
    │   ├─► algorithms/exact/degree_collector.go
    │   ├─► algorithms/exact/shortest_path.go
    │   ├─► algorithms/ledp/pagerank_ledp.go
    │   └─► ...
    │
    ├─► Load Graph Data
    │   ├─► Local testing: data/{worker_index+1}.txt
    │   └─► Deployment: file_path (same for all)
    │
    ├─► Initialize Algorithm
    │   ├─► Graph data
    │   ├─► Worker ID
    │   ├─► Number of workers
    │   └─► Vertex assignment
    │
    ├─► Connect to silhouette-db
    │   └─► pkg/client/client.go
    │
    └─► Execute Algorithm
        ├─► Round 1: Publish local results
        ├─► Round 1: Query needed values
        ├─► Round 2: Publish local results
        ├─► Round 2: Query needed values
        └─► ... (until convergence or max rounds)
```

### Example: Degree Collector Algorithm

**Purpose**: Collect vertex degrees using 2-round algorithm.

**Round 1**:
1. Each worker computes degrees for its assigned vertices
2. Workers publish `(vertex-{id}, degree)` pairs
3. All workers synchronize (wait for completion)

**Round 2**:
1. Each worker queries degrees of its neighbors
2. Workers write results to file
3. Workers publish empty pairs (synchronization only)

**Implementation** (`algorithms/exact/degree_collector.go`):
- Initializes with graph data and worker assignment
- Executes 2 rounds
- Uses OKVS/PIR for oblivious degree queries
- Writes results to file

## API Reference

### Protocol Buffers Definition (`api/v1/silhouette.proto`)

```protobuf
service CoordinationService {
    rpc StartRound(StartRoundRequest) returns (StartRoundResponse);
    rpc PublishValues(PublishValuesRequest) returns (PublishValuesResponse);
    rpc GetValue(GetValueRequest) returns (GetValueResponse);
    rpc GetBaseParams(GetBaseParamsRequest) returns (GetBaseParamsResponse);
    rpc GetKeyMapping(GetKeyMappingRequest) returns (GetKeyMappingResponse);
}
```

### gRPC Endpoints

**StartRound**:
- **Request**: `round_id`, `expected_workers`
- **Response**: `success`
- **Behavior**: Initializes round on leader, tracks expected worker count

**PublishValues**:
- **Request**: `round_id`, `worker_id`, `pairs[]`
- **Response**: `success`
- **Behavior**: Records worker contribution, aggregates when all workers submit, creates OKVS/PIR if needed

**GetValue**:
- **Request**: `round_id`, `pir_query`
- **Response**: `pir_response`
- **Behavior**: Processes PIR query, returns oblivious response

**GetBaseParams**:
- **Request**: `round_id`
- **Response**: `base_params` (serialized)
- **Behavior**: Returns FrodoPIR BaseParams for client initialization

**GetKeyMapping**:
- **Request**: `round_id`
- **Response**: `entries[]` (key → index mapping)
- **Behavior**: Returns key-to-index mapping for PIR queries

### Go Client API (`pkg/client/client.go`)

```go
// Connect to server
client := client.NewClient(serverAddr, nil)

// Start a round
err := client.StartRound(ctx, roundID, expectedWorkers)

// Publish key-value pairs
err := client.PublishValues(ctx, roundID, workerID, pairs)

// Query a value (automatically initializes PIR client if needed)
value, err := client.GetValue(ctx, roundID, key)

// Manual PIR client initialization
err := client.InitializePIRClient(ctx, roundID)
```

## Build System

### Makefile Targets

**Core Build**:
- `make build`: Build all binaries (server, clients, tests)
- `make proto`: Generate Protocol Buffer code
- `make test`: Run all tests
- `make clean`: Clean build artifacts

**Algorithm Framework**:
- `make build-algorithm-runner`: Build algorithm runner

**Cryptographic Libraries**:
- `make build-pir`: Build FrodoPIR FFI library
- `make build-okvs`: Build RB-OKVS FFI library
- `make test-pir`: Test FrodoPIR FFI
- `make bench-pir`: Benchmark PIR operations

**Testing**:
- `make test-runtime`: Single-node runtime tests
- `make test-cluster`: Multi-node cluster tests
- `make test-multi-worker`: Multi-worker aggregation tests
- `make test-load`: Load testing with configurable QPS

### Dependencies

**Go**:
- Go 1.21 or later
- gRPC v1.76.0+
- Protocol Buffers v4.25.0+
- HashiCorp Raft

**Rust**:
- Rust 1.61.0 or later (for cryptographic libraries)
- `cbindgen` (for C header generation)

**System**:
- Protocol Buffer compiler (`protoc`)
- Go plugins: `protoc-gen-go`, `protoc-gen-go-grpc`

## Deployment and Operations

### Single-Node Deployment

**With OKVS Backend (default)**:
```bash
# Start server with OKVS backend
./bin/silhouette-server \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./data/node1 \
    -bootstrap \
    -storage-backend=okvs
```

**With KVS Backend**:
```bash
# Start server with KVS backend
./bin/silhouette-server \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./data/node1 \
    -bootstrap \
    -storage-backend=kvs
```

### Multi-Node Cluster Deployment

**Bootstrap Node**:
```bash
./bin/silhouette-server \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./data/node1 \
    -bootstrap \
    -storage-backend=okvs  # or kvs
```

**Additional Nodes**:
```bash
./bin/silhouette-server \
    -node-id=node2 \
    -listen-addr=127.0.0.1:8081 \
    -grpc-addr=127.0.0.1:9091 \
    -data-dir=./data/node2 \
    -join=127.0.0.1:8080 \
    -storage-backend=okvs  # Must match bootstrap node
```

**Note**: All nodes in a cluster should use the same storage backend for consistency.

### Running Algorithms

**Local Testing Mode**:
```bash
# Step 1: Generate partitioned graph
python3 data-generation/generate_graph.py \
    --config configs/degree_collector.yaml \
    --num-vertices 20 \
    --num-edges 30 \
    --seed 42

# Step 2: Run workers (each with different worker_id)
./bin/algorithm-runner -config configs/degree_collector_worker-0.yaml &
./bin/algorithm-runner -config configs/degree_collector_worker-1.yaml &
./bin/algorithm-runner -config configs/degree_collector_worker-2.yaml &
```

**Deployment Mode**:
```bash
# Each worker loads from same file path
./bin/algorithm-runner -config configs/algorithm_worker-0.yaml
```

### Testing

See [Testing Guide](./testing.md) for comprehensive testing procedures including:
- Unit tests
- Integration tests
- Runtime tests
- Cluster tests
- Load tests
- Algorithm-specific tests

## Additional Resources

- **Setup Guide**: [guides/setup.md](./setup.md)
- **Algorithm Guide**: [guides/algorithms.md](./algorithms.md)
- **Testing Guide**: [guides/testing.md](./testing.md)
- **PIR Integration**: [guides/pir-integration.md](./pir-integration.md)
- **OKVS Integration**: [guides/okvs-integration-plan.md](./okvs-integration-plan.md)
- **Implementation Plan**: [guides/implementation-plan.md](./implementation-plan.md)
- **Benchmarks**: [guides/benchmarks.md](./benchmarks.md)

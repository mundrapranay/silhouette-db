Of course. Here is a detailed software engineering design guide for `silhouette-db`. This document outlines the architecture, components, APIs, and implementation strategy necessary to build the framework.

-----

## `silhouette-db`: A Software Engineering Design Guide

This document provides a comprehensive engineering blueprint for the `silhouette-db` framework. The goal is to create a fault-tolerant, distributed, and oblivious coordination layer for testing Local Edge Differentially Private (LEDP) algorithms. This guide is intended for software engineers and can be used as a direct specification for implementation.

### 1\. High-Level Architecture

The system replaces the centralized coordinator model from the original research with a distributed, peer-to-peer architecture built upon a custom, oblivious key-value store.[1]

  * **LEDP Workers (Clients):** These processes execute the LEDP algorithm logic. They are the clients of the `silhouette-db` system.
  * **Coordination Layer (`silhouette-db`):** A cluster of server nodes that collectively provide a fault-tolerant and oblivious key-value store. This layer exposes a gRPC API for workers to interact with.

The core interaction model is round-based. In each synchronous round of an algorithm, workers publish their results to the Coordination Layer and retrieve the results they need for the next round, all while preserving the privacy of both the data's storage pattern and the queries themselves.

### 2\. Coordination Layer: Detailed Design

The Coordination Layer is a purpose-built, replicated state machine that seamlessly integrates a consensus protocol with advanced cryptographic primitives.

#### 2.1 Consensus and State Management: The Raft Layer

The foundation of the system is the Raft consensus algorithm, which provides strong consistency and crash fault tolerance (CFT).[2, 3, 4]

  * **Function:** The Raft module ensures that all nodes in the `silhouette-db` cluster agree on an identical, ordered log of operations.[5, 6] This replicated log is used to build a replicated state machine—in our case, the key-value store.

  * **Implementation:** We will use **HashiCorp's Raft library** (`hashicorp/raft`), a mature, production-grade Go implementation.[7] This library allows us to focus on the application logic by providing the core consensus mechanism.

  * **Finite State Machine (FSM):** The application-specific logic is implemented as a Finite State Machine. Our FSM will be a simple in-memory key-value map that stores the oblivious data structures for each round. It must implement the `raft.FSM` interface from the HashiCorp library [7]:

    ```go
    // FSM is our simple key-value store state machine.
    // It stores opaque data blobs, which are the OKVS structures.
    type FSM struct {
        mu   sync.RWMutex
        data map[string]byte // The key-value store
    }

    // Apply applies a Raft log entry to the FSM.
    // This is the primary way the FSM is modified.
    func (f *FSM) Apply(log *raft.Log) interface{} {
        f.mu.Lock()
        defer f.mu.Unlock()
        
        // Assuming log.Data is a serialized command like {Op: "SET", Key: "...", Value: "..."}
        // We deserialize and apply it to our map.
        var cmd Command
        if err := deserialize(log.Data, &cmd); err!= nil {
            return err // Return error to be captured by the ApplyFuture
        }

        switch cmd.Op {
        case "SET":
            f.data[cmd.Key] = cmd.Value
            return nil
        default:
            return fmt.Errorf("unrecognized command op: %s", cmd.Op)
        }
    }

    // Snapshot is used to support log compaction. It captures a snapshot
    // of the FSM state.
    func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
        f.mu.RLock()
        defer f.mu.RUnlock()

        // Clone the data map to avoid race conditions
        clone := make(map[string]byte)
        for k, v := range f.data {
            clone[k] = v
        }
        return &FSMSnapshot{data: clone}, nil
    }

    // Restore is used to restore the FSM from a snapshot.
    func (f *FSM) Restore(rc io.ReadCloser) error {
        // Read the snapshot data and replace the FSM's state.
        //... implementation details...
    }
    ```

#### 2.2 Privacy Preservation: The Cryptographic Layer

This layer is responsible for providing access-pattern and query-content obliviousness. It will wrap highly optimized, third-party cryptographic libraries using Go's `cgo` interface.

  * **Oblivious Key-Value Store (OKVS):** Used to encode the set of all `(key, value)` pairs from a round into a single, compact data structure that hides the original keys.[8, 9]

      * **Recommended Algorithm:** **RB-OKVS** (Random Band Matrix OKVS) for its near-optimal encoding rate and efficiency.[8]
      * **Go Interface (`internal/crypto/okvs.go`):**
        ```go
        package crypto

        // OKVSEncoder defines the interface for encoding key-value pairs.
        type OKVSEncoder interface {
            // Encode takes a map of key-value pairs and returns an opaque,
            // oblivious data structure as a byte slice.
            Encode(pairs map[string]byte) (byte, error)
        }
        ```

  * **Private Information Retrieval (PIR):** Used to query the OKVS data structure without revealing the key being requested.[10, 11]

      * **Recommended Algorithm:** **FrodoPIR** for its practical performance, sub-second response times, and open-source Rust implementation.[12]
      * **Go Interface (`internal/crypto/pir.go`):**
        ```go
        package crypto

        // PIRServer defines the server-side interface for a PIR scheme.
        type PIRServer interface {
            // ProcessQuery takes the database (the OKVS blob) and an opaque
            // client query, and returns an opaque server response.
            // The server learns nothing about the item being queried.
            ProcessQuery(dbbyte, querybyte) (byte, error)
        }
        ```
    Based on my findings, the reference implementation for FrodoPIR is actually written in Rust, not Python.[1] You can find the open-source code provided by Brave's research team on their GitHub repository:

    *   **FrodoPIR GitHub Repository:** [https://github.com/brave-experiments/frodo-pir](https://github.com/brave-experiments/frodo-pir) [1]

    This aligns perfectly with the integration strategy outlined in the software engineering guide. The plan to use Go's foreign function interface, `cgo`, is not limited to C/C++ libraries. Rust can expose a C-compatible API, which allows the Go-based `silhouette-db` framework to call the high-performance FrodoPIR Rust code directly.

    Therefore, the recommended approach remains the same:
    1.  Use the official Rust implementation of FrodoPIR for its performance and correctness.
    2.  Create a C-style wrapper around the necessary Rust functions.
    3.  Use `cgo` within the `silhouette-db` Go project to call these wrapped functions.

    This strategy leverages the best of both worlds: Go's excellent support for building networked services and the performance of the specialized, optimized Rust cryptography library.[1, 2]

#### 2.3 Network Interface: The gRPC Layer

This layer exposes the `silhouette-db` functionality to the LEDP Workers.

  * **Framework:** gRPC with Protocol Buffers for a strongly-typed, high-performance, language-agnostic API.
  * **Service Definition:** The service definition will be formalized in a `.proto` file, as detailed in Section 4.
  * **Handler Logic:** The gRPC server handlers will orchestrate the interaction between the Raft and Cryptographic layers. For example, the `GetValue` handler will receive a PIR query, read the corresponding OKVS blob from the local FSM, pass both to the `PIRServer.ProcessQuery` method, and return the result. Write operations like `PublishValues` will be proposed to the Raft cluster using `raft.Apply()`.

### 3\. Protocol Walkthrough: A Single Algorithmic Round

1.  **Start Round:** A worker calls the `StartRound` RPC. The leader node initializes any necessary state for the new round, such as a temporary map to track incoming data from workers.

2.  **Publish Phase (Oblivious Write):**
    a. Each LEDP worker computes its local set of `(key, value)` pairs for the round.
    b. Each worker calls the `PublishValues` RPC on the leader, sending its batch of pairs.
    c. The leader's gRPC handler collects these pairs from all expected workers. Once all data is received, it invokes the OKVS encoder: `okvs_blob, err := okvsEncoder.Encode(all_pairs)`.
    d. The leader serializes a `SET` command containing the round ID as the key and `okvs_blob` as the value.
    e. The leader submits this command to the Raft cluster: `future := raft.Apply(serialized_cmd, timeout)`.
    f. The Raft protocol replicates the command. Once committed, each node in the cluster applies it to its FSM, durably storing the oblivious data structure for that round.

3.  **Retrieve Phase (Oblivious Read):**
    a. A worker needing the value for a specific `key` generates a PIR query for it using its client-side PIR library.
    b. The worker calls the `GetValue` RPC, sending the round ID and the opaque PIR query. The request is automatically forwarded to the current Raft leader.
    c. The leader's `GetValue` handler reads the `okvs_blob` for the requested round from its local FSM (e.g., `okvs_blob := fsm.data["round_k_results"]`).
    d. The handler calls the PIR processing function: `pir_response, err := pirServer.ProcessQuery(okvs_blob, pir_query)`.
    e. The leader returns the opaque `pir_response` to the worker.
    f. The worker decodes the response locally to retrieve the desired value.

### 4\. API Specification (`api/v1/silhouette.proto`)

The following Protocol Buffers definition specifies the gRPC service contract.

```protobuf
syntax = "proto3";

package silhouette.v1;

option go_package = "github.com/your-org/silhouette-db/api/v1;apiv1";

// CoordinationService provides the API for LEDP workers to interact with
// the oblivious, distributed coordination layer.
service CoordinationService {
    // StartRound initializes a new synchronous round for data submission.
    // It tells the system to expect data from a certain number of workers.
    rpc StartRound(StartRoundRequest) returns (StartRoundResponse);

    // PublishValues allows a worker to submit its key-value pairs for a given round.
    // The server will aggregate contributions from all workers before encoding.
    rpc PublishValues(PublishValuesRequest) returns (PublishValuesResponse);

    // GetValue allows a worker to privately retrieve a value for a specific key
    // from a completed round using a PIR query.
    rpc GetValue(GetValueRequest) returns (GetValueResponse);
}

// A single key-value pair. Keys are strings for simplicity, values are opaque bytes.
message KeyValuePair {
    string key = 1;
    bytes value = 2;
}

message StartRoundRequest {
    // A unique identifier for the algorithmic round.
    uint64 round_id = 1;
    // The number of workers that will be submitting data in this round.
    // The leader will wait for this many PublishValues calls before proceeding.
    int32 expected_workers = 2;
}

message StartRoundResponse {
    // Acknowledges that the round has been initiated on the server.
    bool success = 1;
}

message PublishValuesRequest {
    uint64 round_id = 1;
    // A unique ID for the calling worker, to track submissions.
    string worker_id = 2;
    // The set of key-value pairs this worker is contributing.
    repeated KeyValuePair pairs = 3;
}

message PublishValuesResponse {
    // Acknowledges receipt of the worker's contribution.
    bool success = 1;
}

message GetValueRequest {
    uint64 round_id = 1;
    // The opaque PIR query generated by the client's PIR library for a specific key.
    bytes pir_query = 2;
}

message GetValueResponse {
    // The opaque PIR response generated by the server, to be decoded by the client.
    bytes pir_response = 1;
}
```

### 5\. Implementation Plan

#### 5.1 Technology Stack

  * **Language:** **Go**.[13] Its concurrency model, robust standard library, and performance are ideal for this networked service.
  * **Consensus:** **`hashicorp/raft`**.[7]
  * **RPC Framework:** **gRPC** and **Protocol Buffers**.
  * **Cryptographic Libraries:**
      * **OKVS:** Wrap a C++ implementation of **RB-OKVS** [8] using `cgo`.
      * **PIR:** Wrap the Rust implementation of **FrodoPIR** [12] using `cgo`.

#### 5.2 Project Structure

A recommended directory layout for the Go project:

```
/silhouette-db
├── api/v1/                  #.proto file and generated Go code
│   ├── silhouette.proto
│   └── silhouette.pb.go
├── cmd/
│   └── silhouette-server/   # Main application for the server node
│       └── main.go
├── internal/
│   ├── crypto/              # cgo wrappers for cryptographic libraries
│   │   ├── okvs.go
│   │   └── pir.go
│   ├── server/              # gRPC server implementation and handlers
│   │   └── server.go
│   └── store/               # Raft FSM implementation and setup
│       ├── fsm.go
│       └── store.go
├── pkg/
│   └── client/              # Go client library for workers
│       └── client.go
├── configs/                 # Example configuration files
└── go.mod
```

#### 5.3 Component Implementation Details

  * **`internal/store/store.go`:** This package will be responsible for initializing a Raft node. It will configure the `raft.Config`, set up the log and stable stores (e.g., using `raft-boltdb`), initialize the network transport, and instantiate the FSM. It will expose a `Store` struct that wraps the `*raft.Raft` instance and provides a clean API for proposing writes (e.g., `Store.Set(key, value)`).
  * **`internal/server/server.go`:** This will contain the gRPC server logic. It will implement the `CoordinationServiceServer` interface. Its handlers will use the `Store` from the `store` package to interact with the Raft cluster and the interfaces from the `crypto` package to perform oblivious operations.
  * **`cmd/silhouette-server/main.go`:** This is the entry point. It will handle command-line flags and configuration files (e.g., node ID, listen addresses, data directory, peer addresses for joining). It will initialize the `Store`, the gRPC `Server`, and tie them together.

### 6\. Configuration and Deployment

  * **Configuration:** Each server node will start with a configuration file (e.g., `config.hcl` or `config.yaml`) specifying:
      * `node_id`: A unique ID for the node in the cluster.
      * `data_dir`: Path to store Raft logs and snapshots.
      * `raft_listen_addr`: The address for Raft's internal P2P communication.
      * `grpc_listen_addr`: The address for the public-facing gRPC API.
      * `join_addr` (optional): The address of an existing cluster member to join.
  * **Cluster Bootstrapping:**
    1.  The first node is started without a `join_addr`. It bootstraps itself as the first member and leader of a new cluster.
    2.  Subsequent nodes are started with the `join_addr` of any existing member. They will automatically join the cluster as followers via the Raft protocol. This process is handled by the HashiCorp Raft library.[7]
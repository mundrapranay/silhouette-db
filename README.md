# silhouette-db

A fault-tolerant, distributed, and oblivious coordination layer for testing Local Edge Differentially Private (LEDP) algorithms.

## Overview

`silhouette-db` replaces the centralized coordinator model with a distributed, peer-to-peer architecture built upon a custom, oblivious key-value store. It provides:

- **Distributed Consensus**: Raft-based consensus for fault tolerance
- **Oblivious Storage**: OKVS (Oblivious Key-Value Store) for hiding storage patterns
- **Private Queries**: PIR (Private Information Retrieval) for query privacy

## Architecture

The system consists of:

- **LEDP Workers (Clients)**: Execute LEDP algorithm logic
- **Coordination Layer**: A cluster of server nodes providing the oblivious key-value store via gRPC

## Project Structure

```
/silhouette-db
├── algorithms/              # Graph algorithms framework
│   ├── common/              # Shared interfaces and utilities
│   ├── exact/               # Exact (non-private) algorithms
│   └── ledp/                # LEDP (private) algorithms
├── api/v1/                  # Protocol Buffers definitions
├── cmd/
│   ├── algorithm-runner/    # Algorithm execution entry point
│   ├── silhouette-server/   # Main server application
│   └── ...                  # Other command-line tools
├── internal/
│   ├── crypto/              # Cryptographic layer (OKVS, PIR)
│   ├── server/              # gRPC server implementation
│   └── store/               # Raft FSM and store
├── pkg/client/              # Client library for workers
├── configs/                 # Configuration files
└── GUIDE.md                 # Detailed design guide
```

## Quick Start

### Prerequisites

- Go 1.21 or later
- Protocol Buffer compiler (`protoc`)
- Go plugins for protoc (`protoc-gen-go`, `protoc-gen-go-grpc`)
- Rust and Cargo (for cryptographic libraries)

### Building

```bash
# Generate Protocol Buffer code
make proto

# Build the server
make build

# Build algorithm runner
make build-algorithm-runner

# Run the server
./bin/silhouette-server -config configs/node1.hcl

# Run an algorithm
./bin/algorithm-runner -config configs/example_algorithm.yaml
```

### Running Algorithms

```bash
# Build the algorithm runner
make build-algorithm-runner

# Run with config file
./bin/algorithm-runner -config configs/example_algorithm.yaml -verbose
```

## Documentation

See [GUIDE.md](./GUIDE.md) for the complete software engineering design guide.

## License

[Specify your license here]


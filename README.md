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
├── api/v1/                  # Protocol Buffers definitions
├── cmd/silhouette-server/   # Main server application
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

### Building

```bash
# Generate Protocol Buffer code
make proto

# Build the server
make build

# Run the server
./bin/silhouette-server -config configs/node1.hcl
```

## Documentation

See [GUIDE.md](./GUIDE.md) for the complete software engineering design guide.

## License

[Specify your license here]


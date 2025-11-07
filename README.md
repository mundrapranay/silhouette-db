# silhouette-db

<div align="center">

![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?style=for-the-badge&logo=go)
![License](https://img.shields.io/badge/license-MIT-blue.svg?style=for-the-badge&logo=github)
![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg?style=for-the-badge)
![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg?style=for-the-badge)
![Coverage](https://img.shields.io/badge/coverage-TBD-lightgrey.svg?style=for-the-badge)

**A fault-tolerant, distributed, and oblivious coordination layer for testing Local Edge Differentially Private (LEDP) algorithms.**

[Features](#features) • [Quick Start](#quick-start) • [Documentation](#documentation) • [Architecture](#architecture)

</div>

---

## Overview

`silhouette-db` replaces the centralized coordinator model with a distributed, peer-to-peer architecture built upon a custom, oblivious key-value store. It provides:

- **Distributed Consensus**: Raft-based consensus for fault tolerance
- **Flexible Storage**: Choose between OKVS (Oblivious Key-Value Store) or KVS (Simple Key-Value Store)
- **Oblivious Storage**: OKVS encoding hides which keys are stored (optional)
- **Private Queries**: PIR (Private Information Retrieval) for query privacy

## Features

✨ **Core Capabilities**

- 🏗️ **Fault-Tolerant**: Raft consensus ensures system availability even with node failures
- 🔒 **Flexible Storage**: Choose OKVS for oblivious storage or KVS for simple, fast storage
- 🔐 **Private Queries**: PIR enables querying without revealing which key was requested
- 📊 **Graph Algorithms**: Round-based synchronous framework for exact and LEDP algorithms
- 🌐 **Distributed**: Multi-node cluster support with automatic replication
- ⚡ **High Performance**: Sub-second PIR query responses for practical database sizes

## Architecture

The system consists of:

- **LEDP Workers (Clients)**: Execute LEDP algorithm logic
- **Coordination Layer**: A cluster of server nodes providing the oblivious key-value store via gRPC

### High-Level Architecture

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
├── guides/                  # Documentation guides
│   ├── guide.md             # Complete software engineering design guide
│   ├── setup.md             # Setup and installation guide
│   ├── algorithms.md        # Algorithms framework documentation
│   ├── testing.md           # Testing guide (manual, automated, algorithms)
│   ├── pir-integration.md   # FrodoPIR integration guide
│   ├── okvs-integration-plan.md  # OKVS integration plan and status
│   ├── storage-backends.md  # Storage backend comparison and usage guide
│   ├── benchmarks.md        # Performance benchmarks
│   ├── implementation-plan.md    # Implementation plan and roadmap
│   └── next-steps.md        # Next steps and future work
└── README.md                # This file
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

# Run the server (default: OKVS backend)
./bin/silhouette-server \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./data/node1 \
    -bootstrap \
    -storage-backend=okvs

# Or use KVS backend for simple, fast storage
./bin/silhouette-server \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./data/node1 \
    -bootstrap \
    -storage-backend=kvs

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

Comprehensive documentation is available in the [`guides/`](./guides/) directory:

- **[Complete Design Guide](./guides/guide.md)** - Software engineering design guide and architecture overview
- **[Setup Guide](./guides/setup.md)** - Installation and setup instructions
- **[Algorithms Guide](./guides/algorithms.md)** - Graph algorithms framework documentation
- **[Testing Guide](./guides/testing.md)** - Comprehensive testing guide including:
  - Manual testing procedures
  - Automated test scripts
  - Algorithm-specific testing (e.g., degree-collector)
  - Test coverage and status
- **[PIR Integration Guide](./guides/pir-integration.md)** - FrodoPIR integration documentation
- **[OKVS Integration Plan](./guides/okvs-integration-plan.md)** - OKVS integration plan and status
- **[Storage Backends Guide](./guides/storage-backends.md)** - Storage backend comparison and usage (OKVS vs KVS)
- **[Benchmarks](./guides/benchmarks.md)** - Performance benchmarks and results
- **[Implementation Plan](./guides/implementation-plan.md)** - Implementation roadmap and progress
- **[Next Steps](./guides/next-steps.md)** - Future work and next steps

For quick reference, see the [Complete Design Guide](./guides/guide.md).

## Components

### Storage Backends

- **OKVS** (Oblivious Key-Value Store): Random Band Matrix OKVS encoding
  - Provides oblivious storage (hides which keys are stored)
  - Minimum 100 pairs required
  - ~10-20% encoding overhead
  - Recommended for privacy-sensitive applications

- **KVS** (Simple Key-Value Store): Direct JSON-based storage
  - Fast and simple (no encoding overhead)
  - Works with any number of pairs (no minimum)
  - No oblivious properties
  - Recommended for testing or when privacy is not required

Both backends work seamlessly with PIR for query privacy.

### Cryptographic Primitives

- **FrodoPIR**: Private Information Retrieval scheme
  - LWE-based cryptography (post-quantum secure)
  - Sub-second query responses
  - Hides which key is queried
  - Works with both OKVS and KVS backends

### Consensus Layer

- **Raft Consensus**: Distributed consensus algorithm
  - Crash fault tolerance (CFT)
  - Automatic leader election
  - Log replication across nodes

### Algorithm Framework

- **Round-Based Execution**: Synchronous rounds for graph algorithms
- **Exact Algorithms**: Non-private graph algorithms (e.g., degree-collector)
- **LEDP Algorithms**: Local Edge Differentially Private algorithms

## Testing

Run tests with:

```bash
# Run all tests
make test

# Run specific test suites
go test ./internal/server/...
go test ./internal/store/...
go test ./internal/crypto/...
go test ./algorithms/...

# Run integration tests
./scripts/test-runtime.sh
./scripts/test-cluster.sh 3
./scripts/test-multi-worker.sh
./scripts/test-load.sh

# Test storage backends
make test-kvs              # Test KVS backend
make test-kvs-integration  # KVS integration tests
make test-okvs             # Test OKVS backend (requires cgo)
make test-e2e-backends     # End-to-end tests with both backends
```

See the [Testing Guide](./guides/testing.md) for comprehensive testing documentation.

## Contributing

Contributions are welcome! Please see our contributing guidelines (to be added).

## References

### Research Papers

*Papers will be added here*

### Related Work

- **FrodoPIR**: [GitHub Repository](https://github.com/brave-experiments/frodo-pir)
- **RB-OKVS**: [GitHub Repository](https://github.com/felicityin/rb-okvs)
- **HashiCorp Raft**: [Documentation](https://github.com/hashicorp/raft)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

Copyright (c) 2025 Pranay Mundra

---

<div align="center">

**Built with ❤️ for privacy-preserving distributed systems**

[Report Bug](https://github.com/mundrapranay/silhouette-db/issues) • [Request Feature](https://github.com/mundrapranay/silhouette-db/issues)

</div>
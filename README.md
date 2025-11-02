# silhouette-db

<div align="center">

![Go Version](https://img.shields.io/badge/go-1.24+-00ADD8?style=for-the-badge&logo=go)
![License](https://img.shields.io/badge/license-MIT-blue.svg?style=for-the-badge)
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
- **Oblivious Storage**: OKVS (Oblivious Key-Value Store) for hiding storage patterns
- **Private Queries**: PIR (Private Information Retrieval) for query privacy

## Features

✨ **Core Capabilities**

- 🏗️ **Fault-Tolerant**: Raft consensus ensures system availability even with node failures
- 🔒 **Oblivious Storage**: OKVS encoding hides which keys are stored
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
┌─────────────────────┐
│  LEDP Workers       │  Execute graph algorithms
│  (Clients)          │
└──────────┬──────────┘
           │ gRPC
           ▼
┌─────────────────────┐
│  Coordination Layer │  Distributed, oblivious key-value store
│  (silhouette-db)    │
│  - Raft Consensus   │
│  - OKVS Encoding    │
│  - PIR Queries      │
└──────────┬──────────┘
           │
           ▼
┌─────────────────────┐
│  Raft Cluster       │  Replicated state machine
│  (Node 1...N)       │
└─────────────────────┘
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

# Run the server
./bin/silhouette-server \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr=127.0.0.1:9090 \
    -data-dir=./data/node1 \
    -bootstrap

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
- **[Benchmarks](./guides/benchmarks.md)** - Performance benchmarks and results
- **[Implementation Plan](./guides/implementation-plan.md)** - Implementation roadmap and progress
- **[Next Steps](./guides/next-steps.md)** - Future work and next steps

For quick reference, see the [Complete Design Guide](./guides/guide.md).

## Components

### Cryptographic Primitives

- **RB-OKVS** (Random Band Matrix OKVS): Oblivious key-value store encoding
  - Minimum 100 pairs required
  - ~10-20% encoding overhead
  - Hides which keys are stored

- **FrodoPIR**: Private Information Retrieval scheme
  - LWE-based cryptography (post-quantum secure)
  - Sub-second query responses
  - Hides which key is queried

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

[Specify your license here]

---

<div align="center">

**Built with ❤️ for privacy-preserving distributed systems**

[Report Bug](https://github.com/mundrapranay/silhouette-db/issues) • [Request Feature](https://github.com/mundrapranay/silhouette-db/issues)

</div>
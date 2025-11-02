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

## License

[Specify your license here]


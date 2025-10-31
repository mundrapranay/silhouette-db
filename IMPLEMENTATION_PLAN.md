# Implementation Plan for silhouette-db

This document outlines the step-by-step implementation plan for building the `silhouette-db` framework as specified in [GUIDE.md](./GUIDE.md).

## Current Status

✅ **Completed:**
- Project structure and directory layout
- Protocol Buffers definition (API specification)
- FSM implementation with Raft integration
- Raft store wrapper
- gRPC server with basic handlers
- Client library interface
- Mock implementations for crypto components
- Configuration files and Makefile

## Implementation Phases

### Phase 1: Core Raft Infrastructure ✅ (In Progress)

**Goal:** Get basic Raft-based key-value store working.

**Tasks:**
- [x] Implement FSM (Finite State Machine) for Raft
- [x] Implement Store wrapper around HashiCorp Raft
- [x] Create main server entry point
- [x] Test single-node cluster bootstrapping
- [ ] Test multi-node cluster formation
- [ ] Implement proper join mechanism
- [ ] Add logging and error handling
- [ ] Write unit tests for FSM and Store

**Next Steps:**
1. Generate Protocol Buffer code: `make proto`
2. Install dependencies: `make deps`
3. Test basic server startup
4. Test multi-node cluster

### Phase 2: gRPC API Integration

**Goal:** Complete gRPC server with all three RPCs working end-to-end.

**Tasks:**
- [x] Protocol Buffers definition
- [ ] Generate Go code from proto: `make proto`
- [x] Implement StartRound handler
- [x] Implement PublishValues handler with aggregation
- [x] Implement GetValue handler
- [ ] Add proper error handling and validation
- [ ] Add request forwarding for non-leader nodes
- [ ] Write integration tests for gRPC API
- [ ] Add request/response logging

**Testing:**
- Test round lifecycle (start → publish → retrieve)
- Test worker aggregation logic
- Test leader election and forwarding

### Phase 3: OKVS Integration

**Goal:** Integrate RB-OKVS for oblivious storage.

**Tasks:**
- [ ] Research and select RB-OKVS implementation
  - Option 1: Find existing C++ implementation
  - Option 2: Port reference implementation to C++/Rust
- [ ] Create C-compatible FFI wrapper
- [ ] Implement cgo bindings in `internal/crypto/okvs.go`
- [ ] Replace MockOKVSEncoder with real implementation
- [ ] Test encoding/decoding with various key-value sets
- [ ] Benchmark performance
- [ ] Verify obliviousness properties

**Resources:**
- Research papers on RB-OKVS
- Look for open-source implementations

### Phase 4: FrodoPIR Integration

**Goal:** Integrate FrodoPIR for private information retrieval.

**Tasks:**
- [x] Add frodo-pir as git submodule (see below)
- [ ] Study FrodoPIR Rust implementation
- [ ] Create Rust FFI wrapper for C-compatible API
- [ ] Build static library (.a) from Rust code
- [ ] Implement cgo bindings in `internal/crypto/pir.go`
- [ ] Replace MockPIRServer with real implementation
- [ ] Create client-side PIR library wrapper
- [ ] Test query generation and response decoding
- [ ] Benchmark query performance
- [ ] Verify privacy properties

**FrodoPIR Setup:**
```bash
# Add as submodule
git submodule add https://github.com/brave-experiments/frodo-pir.git third_party/frodo-pir

# Or clone if starting fresh
git clone https://github.com/brave-experiments/frodo-pir.git third_party/frodo-pir
```

**Rust FFI Wrapper Steps:**
1. Create `third_party/frodo-pir-ffi/` directory
2. Create C-compatible wrapper functions
3. Configure Cargo.toml to build as static library
4. Use `cbindgen` to generate C headers
5. Build `.a` file for linking

### Phase 5: End-to-End Testing

**Goal:** Verify complete system works correctly.

**Tasks:**
- [ ] Write end-to-end test for complete round (start → publish → retrieve)
- [ ] Test with multiple workers
- [ ] Test with multiple server nodes
- [ ] Test fault tolerance (node failures, leader election)
- [ ] Test with realistic data sizes
- [ ] Performance benchmarking
- [ ] Load testing

### Phase 6: Production Readiness

**Goal:** Make system production-ready.

**Tasks:**
- [ ] Comprehensive error handling
- [ ] Structured logging (consider using zap or logrus)
- [ ] Metrics and observability (Prometheus metrics)
- [ ] Configuration file parsing (HCL or YAML)
- [ ] Health check endpoints
- [ ] Graceful shutdown
- [ ] Documentation
  - API documentation
  - Deployment guide
  - Developer guide
  - Architecture diagrams

## Immediate Next Steps

### Step 1: Set Up Development Environment

```bash
# 1. Generate Protocol Buffer code
make proto

# 2. Install dependencies
make deps

# 3. Verify everything compiles
go build ./...
```

### Step 2: Test Basic Server

```bash
# Build and run single node
make build
make run

# In another terminal, test with client (once client tests are written)
```

### Step 3: Add FrodoPIR Submodule

```bash
# Initialize git submodules if not done
git submodule init
git submodule add https://github.com/brave-experiments/frodo-pir.git third_party/frodo-pir
git submodule update
```

### Step 4: Start Implementing Real Crypto Components

Begin with either OKVS or PIR (or both in parallel if you have multiple developers):
- Set up FFI wrappers
- Create cgo bindings
- Write unit tests
- Integrate with main server

## Technical Decisions Needed

1. **OKVS Implementation:**
   - Which specific RB-OKVS implementation to use?
   - Build from research papers or find existing library?

2. **Configuration Format:**
   - HCL (HashiCorp Config Language) - aligns with HashiCorp Raft
   - YAML - more common
   - JSON - simple but less readable

3. **Logging:**
   - Standard library log
   - Structured logging (zap, logrus)
   - What log levels and formats?

4. **Testing Strategy:**
   - Unit test coverage targets
   - Integration test framework
   - Benchmarking requirements

## Resources

- [HashiCorp Raft Documentation](https://github.com/hashicorp/raft)
- [FrodoPIR GitHub](https://github.com/brave-experiments/frodo-pir)
- [gRPC Go Documentation](https://grpc.io/docs/languages/go/)
- Protocol Buffers: [Go Tutorial](https://protobuf.dev/getting-started/gotutorial/)

## Notes

- The current implementation uses mock crypto components for testing the core Raft and gRPC logic
- Replace mocks with real implementations in Phases 3 and 4
- Consider adding integration tests early to catch breaking changes
- Document API contracts clearly as implementation progresses


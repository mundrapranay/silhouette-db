# Implementation Plan for silhouette-db

This document outlines the step-by-step implementation plan for building the `silhouette-db` framework as specified in [GUIDE.md](./GUIDE.md).

## Current Status

✅ **Completed:**
- Project structure and directory layout
- Protocol Buffers definition (API specification)
- Protocol Buffer code generation (`make proto`)
- FSM implementation with Raft integration
- Raft store wrapper
- gRPC server with all three RPC handlers (StartRound, PublishValues, GetValue)
- Client library interface
- Mock implementations for crypto components
- Configuration files and Makefile
- Build system setup and dependency management
- Code compiles successfully

## Implementation Phases

### Phase 1: Core Raft Infrastructure ✅ (Mostly Complete)

**Goal:** Get basic Raft-based key-value store working.

**Tasks:**
- [x] Implement FSM (Finite State Machine) for Raft
- [x] Implement Store wrapper around HashiCorp Raft
- [x] Create main server entry point
- [x] Generate Protocol Buffer code (`make proto`)
- [x] Install and update dependencies (gRPC v1.76.0)
- [x] Verify code compiles successfully
- [x] Write unit tests for FSM (8 tests passing)
- [x] Write unit tests for Store (5 tests passing)
- [x] Add FrodoPIR submodule
- [ ] Test single-node cluster bootstrapping (manual runtime testing)
- [ ] Test multi-node cluster formation
- [ ] Implement proper join mechanism
- [ ] Add structured logging and error handling

**Next Steps:**
1. Test basic server startup: `make build && make run`
2. Test multi-node cluster formation
3. Add unit tests for core components
4. Implement proper error handling and logging

### Phase 2: gRPC API Integration ✅ (Core Complete)

**Goal:** Complete gRPC server with all three RPCs working end-to-end.

**Tasks:**
- [x] Protocol Buffers definition
- [x] Generate Go code from proto (`make proto` works)
- [x] Fix import issues and verify compilation
- [x] Implement StartRound handler
- [x] Implement PublishValues handler with aggregation
- [x] Implement GetValue handler
- [x] Fix Protocol Buffer type conversion issues
- [x] Write integration tests for gRPC API (6 tests passing)
- [ ] Add proper error handling and validation
- [ ] Add request forwarding for non-leader nodes
- [ ] Add request/response logging
- [ ] Test end-to-end round lifecycle with real client

**Testing:**
- [ ] Test round lifecycle (start → publish → retrieve)
- [ ] Test worker aggregation logic
- [ ] Test leader election and forwarding

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

### Step 1: Runtime Testing ✅ (Complete)

**Completed:**
- ✅ Development environment setup
- ✅ Protocol Buffer code generation
- ✅ Dependencies installed and updated
- ✅ Code compiles successfully
- ✅ Build system tested (`make build` works)

**Next:**
```bash
# Test basic server startup manually
make run

# In another terminal, test with client
# (Need to create test client or use existing client library)
```

### Step 2: Add FrodoPIR Submodule ✅ (Complete)

**Completed:**
- ✅ Added FrodoPIR as git submodule (`third_party/frodo-pir`)

```bash
# Submodule already added, update if needed:
git submodule update --init --recursive
```

### Step 3: Unit and Integration Testing ✅ (Core Complete)

**Completed:**
- ✅ Unit tests for FSM operations (8 tests, all passing)
  - FSM creation, SET/DELETE operations, snapshots, restore
- ✅ Unit tests for Store operations (5 tests, all passing)
  - Store creation, Set/Get operations, leadership detection, multiple operations
- ✅ Integration tests for gRPC API (6 tests, all passing)
  - StartRound, PublishValues, GetValue, complete round lifecycle

**Next Steps:**
- Test round lifecycle end-to-end with real client
- Test multi-node cluster formation
- Add more edge case tests
- Add performance/benchmark tests

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

## Recent Progress

**Latest Updates:**
- ✅ Fixed Protocol Buffer import issues (`apiv1` package)
- ✅ Generated Protocol Buffer code successfully
- ✅ Upgraded gRPC from v1.60.1 to v1.76.0 (required for `SupportPackageIsVersion9`)
- ✅ Fixed type conversion issues (pointer slices to value slices)
- ✅ Fixed gRPC client creation (`grpc.Dial` instead of `grpc.NewClient`)
- ✅ All code compiles successfully

**Known Issues:**
- Runtime testing not yet performed (needs manual testing)
- Mock crypto components still in use (OKVS and PIR)
- No unit tests yet written

## Notes

- The current implementation uses mock crypto components for testing the core Raft and gRPC logic
- Replace mocks with real implementations in Phases 3 and 4
- Consider adding integration tests early to catch breaking changes
- Document API contracts clearly as implementation progresses
- **Build Status**: ✅ Code compiles successfully
- **Protocol Buffers**: ✅ Generated and working


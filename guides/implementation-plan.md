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
- Configuration files and Makefile
- Build system setup and dependency management
- Code compiles successfully
- **FrodoPIR Integration (Phase 4):** ✅ Complete
  - Rust FFI wrapper for FrodoPIR
  - Go cgo bindings and implementations
  - Static library built and tested
  - Complete documentation and build targets
  - Server and client integration
  - Key-to-index mapping
  - Integration tests and benchmarks
- **OKVS Integration (Phase 3):** ✅ Complete
  - RB-OKVS library selected and added as submodule
  - Rust FFI wrapper created (`rb-okvs-ffi`)
  - Go cgo bindings implemented
  - Server integration (replaced MockOKVSEncoder)
  - PIR integration (OKVS-encoded blobs work with PIR)
  - Complete test suite (all tests passing)

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

### Phase 3: OKVS Integration ✅ (Complete)

**Goal:** Integrate RB-OKVS for oblivious storage.

**Tasks:**
- [x] Research and select RB-OKVS implementation
  - ✅ Selected: `felicityin/rb-okvs` (Rust implementation)
  - ✅ Added as git submodule (`third_party/rb-okvs`)
  - ✅ Verified correctness with float64 use case tests (7 tests passing)
- [x] Create C-compatible FFI wrapper
  - ✅ Created `third_party/rb-okvs-ffi/` (Rust FFI wrapper)
  - ✅ Implemented `rb_okvs_encode` and `rb_okvs_decode` functions
  - ✅ Built static library (`librbokvsffi.a`)
  - ✅ Generated C header file (`rb_okvs_ffi.h`)
- [x] Implement cgo bindings in `internal/crypto/okvs_impl.go`
  - ✅ Created `RBOKVSEncoder` and `RBOKVSDecoder` implementations
  - ✅ Implemented memory management and error handling
  - ✅ Unit tests (6 tests, all passing)
- [x] Replace MockOKVSEncoder with real implementation
  - ✅ Updated `cmd/silhouette-server/main.go` to use `RBOKVSEncoder`
  - ✅ Server gracefully handles < 100 pairs (falls back to direct PIR)
- [x] Test encoding/decoding with various key-value sets
  - ✅ Integration tests (4 tests, all passing)
  - ✅ PIR + OKVS integration tests (2 tests, all passing)
- [x] PIR Integration
  - ✅ Integrated OKVS with PIR workflow
  - ✅ Temporary storage pattern implemented
  - ✅ Automatic OKVS encoding when 100+ pairs
  - ✅ OKVS-decoded values used for PIR database

**Key Features:**
- ✅ Handles float64 values (8 bytes, little-endian)
- ✅ Minimum 100 pairs requirement enforced
- ✅ Automatic fallback to direct PIR when < 100 pairs
- ✅ Key hashing using BLAKE2b512
- ✅ Complete memory management
- ✅ Error handling and validation

**Test Results:**
- ✅ 6 OKVS unit tests (all passing)
- ✅ 4 OKVS integration tests (all passing)
- ✅ 2 PIR + OKVS integration tests (all passing)

**Resources:**
- RB-OKVS Library: `third_party/rb-okvs/` (git submodule from `felicityin/rb-okvs`)
- FFI Wrapper: `third_party/rb-okvs-ffi/`
- Implementation: `internal/crypto/okvs_impl.go`
- Documentation: `OKVS_INTEGRATION_PLAN.md`

### Phase 4: FrodoPIR Integration ✅ (Complete)

**Goal:** Integrate FrodoPIR for private information retrieval.

**Tasks:**
- [x] Add frodo-pir as git submodule (see below)
- [x] Study FrodoPIR Rust implementation
- [x] Create Rust FFI wrapper for C-compatible API (`third_party/frodo-pir-ffi/`)
- [x] Build static library (.a) from Rust code
- [x] Generate C header file with `cbindgen`
- [x] Implement cgo bindings in `internal/crypto/pir.go`
- [x] Create `FrodoPIRServer` implementation
- [x] Create `FrodoPIRClient` implementation
- [x] Add Makefile targets for building PIR library (`build-pir`, `clean-pir`, `test-pir`)
- [x] Create integration documentation (`PIR_INTEGRATION_GUIDE.md`)
- [x] Add safety checks and error handling
- [x] Fix cgo build configuration
- [x] Replace MockPIRServer with real implementation in server code
- [x] Implement key-to-index mapping for server
- [x] Update client library to use FrodoPIR
- [x] Test query generation and response decoding end-to-end
- [x] Benchmark query performance
- [x] Implement retry logic for overflow errors
- [ ] Verify privacy properties (theoretical verification)

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
- [x] Write end-to-end test for complete round (start → publish → retrieve)
  - ✅ Integration tests for gRPC API (6 tests passing)
  - ✅ PIR integration tests (2 tests passing)
  - ✅ OKVS integration tests (4 tests passing)
  - ✅ PIR + OKVS integration tests (2 tests passing)
- [x] Test with multiple workers
  - ✅ Worker aggregation logic tested
- [x] Test OKVS + PIR integration
  - ✅ End-to-end tests with OKVS-encoded data and PIR queries (2 tests passing)
- [ ] Test with multiple server nodes (cluster testing)
- [ ] Test fault tolerance (node failures, leader election)
- [ ] Test with realistic data sizes (performance under load)
- [x] Performance benchmarking
  - ✅ PIR benchmarks (4 benchmarks)
  - ✅ Store benchmarks (5 benchmarks)
  - [ ] OKVS benchmarks (encoding/decoding performance)
- [ ] Load testing (concurrent workers, multiple rounds)
- [ ] Runtime testing (manual testing of complete system)

### Phase 6: Production Readiness

**Goal:** Make system production-ready.

**Tasks:**
- [ ] Comprehensive error handling
- [ ] Structured logging (consider using zap or logrus)
- [ ] Metrics and observability (Prometheus metrics)
- [ ] Configuration file parsing (HCL or YAML)
  - [ ] Implement HCL config file parser for server (`configs/node1.hcl`, `node2.hcl`, `node3.hcl`)
  - [ ] Add `-config` flag to `silhouette-server` to load config from file
  - [ ] Parse HCL files and map to existing command-line flags
  - [ ] Update documentation to reflect config file usage
  - [ ] Currently server only accepts command-line flags; HCL files exist but are unused
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

**Completed:**
- ✅ End-to-end round lifecycle test with real gRPC client (3 tests)
- ✅ Multi-node cluster formation tests (4 tests)
- ✅ Edge case tests (6 tests: duplicates, errors, empty data, large values)
- ✅ Performance/benchmark tests (9 benchmarks: FSM and server operations)

**Total Test Count:** 
- Core tests: 37 tests (all passing) + 9 benchmarks
- FrodoPIR tests: 2 integration tests (all passing) + 4 benchmarks
- OKVS tests: 10 tests (all passing)
  - 6 unit tests (`okvs_impl_test.go`)
  - 4 integration tests (`okvs_integration_test.go`)
- PIR + OKVS tests: 2 end-to-end tests (all passing)
- **Grand Total:** 51 tests (all passing) + 13 benchmarks

### Step 4: Start Implementing Real Crypto Components ✅ (Complete)

**FrodoPIR Integration Completed:** ✅
- ✅ Set up Rust FFI wrapper (`third_party/frodo-pir-ffi/`)
- ✅ Created cgo bindings (`internal/crypto/pir.go`)
- ✅ Built static library and C headers
- ✅ Implemented `FrodoPIRServer` and `FrodoPIRClient`
- ✅ Added Makefile targets and documentation
- ✅ Code compiles successfully
- ✅ **Server Integration:** Replaced MockPIRServer with FrodoPIRServer in server code
- ✅ **Key-to-Index Mapping:** Implemented deterministic mapping with sorted keys
- ✅ **Client Integration:** Updated client library with dynamic PIR client initialization
- ✅ **New gRPC Endpoints:** Added `GetBaseParams` and `GetKeyMapping` RPCs
- ✅ **Integration Tests:** Added `pir_integration_test.go` with end-to-end tests
- ✅ **Benchmarks:** Added `pir_benchmark_test.go` for performance testing
- ✅ **Retry Logic:** Implemented automatic retry for overflow errors
- ✅ **Documentation:** Comprehensive guide with benchmark interpretation

**OKVS Integration Completed:** ✅
- ✅ Research and selected RB-OKVS implementation (`felicityin/rb-okvs`)
- ✅ Added as git submodule (`third_party/rb-okvs`)
- ✅ Created Rust FFI wrapper (`third_party/rb-okvs-ffi/`)
- ✅ Built static library (`librbokvsffi.a` - 40MB)
- ✅ Generated C header file (`rb_okvs_ffi.h`)
- ✅ Implemented Go cgo bindings (`internal/crypto/okvs_impl.go`)
- ✅ Created `RBOKVSEncoder` and `RBOKVSDecoder` implementations
- ✅ **Server Integration:** Replaced MockOKVSEncoder with RBOKVSEncoder
- ✅ **PIR Integration:** Integrated OKVS with PIR workflow
  - ✅ Temporary storage pattern (pairs accumulated in `roundState.workerData`)
  - ✅ Automatic OKVS encoding when 100+ pairs
  - ✅ OKVS-decoded values used for PIR database
  - ✅ Graceful fallback to direct PIR when < 100 pairs
- ✅ **Unit Tests:** 6 tests for OKVS encoding/decoding (all passing)
- ✅ **Integration Tests:** 4 tests for OKVS server integration (all passing)
- ✅ **PIR + OKVS Tests:** 2 end-to-end tests with PIR queries (all passing)

**Next Steps for Crypto Components:** ✅ (All Complete)
- [x] Integrate FrodoPIR into server code
- [x] Integrate OKVS into server code
- [x] Implement key-to-index mapping
- [x] Update client library to use FrodoPIR
- [x] Integrate OKVS with PIR
- [x] Write integration tests
- [x] Benchmark performance

## Next Steps

### Immediate Next Steps

1. **Runtime Testing and Validation:**
   - [ ] Manual runtime testing of complete system
   - [ ] Test with multiple workers and nodes
   - [ ] Test OKVS + PIR integration end-to-end
   - [ ] Load testing and performance validation
   - [ ] Verify privacy properties under load
   - [ ] Test graceful degradation (< 100 pairs scenario)

2. **Production Readiness (Phase 6):**
   - [ ] Comprehensive error handling
   - [ ] Structured logging (zap or logrus)
   - [ ] Metrics and observability (Prometheus metrics)
   - [ ] Health check endpoints
   - [ ] Graceful shutdown
   - [ ] Configuration file parsing (HCL or YAML)
     - [ ] Implement HCL config file parser for server (`configs/node1.hcl`, `node2.hcl`, `node3.hcl`)
     - [ ] Add `-config` flag to `silhouette-server` to load config from file
     - [ ] Parse HCL files and map to existing command-line flags
     - [ ] Update documentation to reflect config file usage
     - [ ] Currently server only accepts command-line flags; HCL files exist but are unused
   - [ ] API rate limiting
   - [ ] Request/response validation

3. **Documentation and Testing:**
   - [ ] Complete API documentation
   - [ ] Deployment guide
   - [ ] Developer guide
   - [ ] Architecture diagrams
   - [ ] Performance benchmarking guide
   - [ ] Security considerations documentation

### Completed: FrodoPIR Integration

All FrodoPIR integration tasks are complete:
- ✅ Server integration with dynamic PIR server creation per round
- ✅ Client integration with dynamic PIR client initialization
- ✅ Key-to-index mapping implementation
- ✅ gRPC API extensions (`GetBaseParams`, `GetKeyMapping`)
- ✅ Comprehensive integration tests
- ✅ Performance benchmarks
- ✅ Error handling and retry logic for overflow errors
- ✅ Complete documentation

## Technical Decisions Needed

1. **OKVS Implementation:**
   - Which specific RB-OKVS implementation to use?
   - Build from research papers or find existing library?

2. **Configuration Format:**
   - HCL (HashiCorp Config Language) - aligns with HashiCorp Raft
     - ⚠️ **Note:** HCL config files (`configs/node1.hcl`, `node2.hcl`, `node3.hcl`) currently exist but are **not used** by the server
     - Server currently only accepts command-line flags (`-node-id`, `-listen-addr`, etc.)
     - TODO: Implement HCL parsing to load server config from files
   - YAML - more common (used for algorithm configs)
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
- ✅ **FrodoPIR Integration (Phase 4):** Complete
  - ✅ Created Rust FFI wrapper (`third_party/frodo-pir-ffi/`)
  - ✅ Built static library (`libfrodopirffi.a` - 17MB)
  - ✅ Generated C header file (`frodopir_ffi.h`)
  - ✅ Implemented Go cgo bindings (`internal/crypto/pir.go`)
  - ✅ Created `FrodoPIRServer` and `FrodoPIRClient` implementations
  - ✅ Added Makefile targets (`build-pir`, `clean-pir`, `test-pir`)
  - ✅ Created comprehensive documentation (`PIR_INTEGRATION_GUIDE.md`)
  - ✅ Fixed cgo build configuration and safety checks
  - ✅ Code compiles successfully with `-tags cgo`
  - ✅ **Server Integration:** Integrated FrodoPIR into server code
  - ✅ **Client Integration:** Updated client library to dynamically initialize PIR clients
  - ✅ **Key Mapping:** Implemented deterministic key-to-index mapping
  - ✅ **gRPC API:** Added `GetBaseParams` and `GetKeyMapping` endpoints
  - ✅ **Testing:** Integration tests and benchmarks for PIR operations
  - ✅ **Error Handling:** Implemented retry logic for overflow errors
- ✅ **OKVS Integration (Phase 3):** Complete
  - ✅ Selected RB-OKVS library (`felicityin/rb-okvs`)
  - ✅ Added as git submodule (`third_party/rb-okvs`)
  - ✅ Created Rust FFI wrapper (`third_party/rb-okvs-ffi/`)
  - ✅ Built static library (`librbokvsffi.a` - 40MB)
  - ✅ Generated C header file (`rb_okvs_ffi.h`)
  - ✅ Implemented Go cgo bindings (`internal/crypto/okvs_impl.go`)
  - ✅ Created `RBOKVSEncoder` and `RBOKVSDecoder` implementations
  - ✅ **Server Integration:** Replaced MockOKVSEncoder with RBOKVSEncoder
  - ✅ **PIR Integration:** OKVS-encoded blobs work seamlessly with PIR
  - ✅ **Temporary Storage:** Implemented accumulation pattern in `roundState`
  - ✅ **Automatic Encoding:** OKVS encoding when 100+ pairs, direct PIR when < 100
  - ✅ **Testing:** Complete test suite (12 tests total, all passing)
  - ✅ **Documentation:** Created `OKVS_INTEGRATION_PLAN.md`

**Known Issues:**
- Runtime testing not yet performed (needs manual testing)
- All mock crypto components replaced with real implementations ✅

## Notes

- ✅ **All crypto components integrated** - Both FrodoPIR and OKVS use real implementations
- ✅ **FrodoPIR fully integrated** - server and client code use real FrodoPIR implementation
- ✅ **OKVS fully integrated** - server uses real RB-OKVS encoder
- FrodoPIR server is created dynamically per round when all workers have submitted data
- OKVS encoding happens automatically when 100+ pairs are accumulated
- System gracefully falls back to direct PIR when < 100 pairs (OKVS requirement)
- Consider adding integration tests early to catch breaking changes
- Document API contracts clearly as implementation progresses
- **Build Status**: ✅ Code compiles successfully (with `-tags cgo` for PIR and OKVS)
- **Protocol Buffers**: ✅ Generated and working
- **FrodoPIR FFI**: ✅ Built and tested (static library: `libfrodopirffi.a`)
- **OKVS FFI**: ✅ Built and tested (static library: `librbokvsffi.a`)

## Files Created for FrodoPIR Integration

**Rust FFI Wrapper:**
- `third_party/frodo-pir-ffi/Cargo.toml` - Rust project configuration
- `third_party/frodo-pir-ffi/src/lib.rs` - FFI wrapper implementation
- `third_party/frodo-pir-ffi/build.rs` - Build script for header generation
- `third_party/frodo-pir-ffi/cbindgen.toml` - Header generation config
- `third_party/frodo-pir-ffi/frodopir_ffi.h` - Generated C header
- `third_party/frodo-pir-ffi/README.md` - FFI wrapper documentation

**Go Integration:**
- `internal/crypto/pir.go` - Go cgo bindings and implementations
- `.golangci.yml` - Linter configuration for cgo
- `PIR_INTEGRATION_GUIDE.md` - Comprehensive integration guide
- `PIR_INTEGRATION.md` - Architecture design document

**Build System:**
- Makefile targets: `build-pir`, `clean-pir`, `test-pir`
- Updated `build` target to include PIR library

## Files Created for OKVS Integration

**Rust FFI Wrapper:**
- `third_party/rb-okvs-ffi/Cargo.toml` - Rust project configuration
- `third_party/rb-okvs-ffi/src/lib.rs` - FFI wrapper implementation
- `third_party/rb-okvs-ffi/build.rs` - Build script for header generation
- `third_party/rb-okvs-ffi/cbindgen.toml` - Header generation config
- `third_party/rb-okvs-ffi/rb_okvs_ffi.h` - Generated C header
- `third_party/rb-okvs-ffi/README.md` - FFI wrapper documentation
- `third_party/rb-okvs-ffi/tests/ffi_test.rs` - FFI test suite

**Go Integration:**
- `internal/crypto/okvs_impl.go` - Go cgo bindings and implementations
- `internal/crypto/okvs_impl_test.go` - Unit tests (6 tests)
- `internal/server/okvs_integration_test.go` - Server integration tests (4 tests)
- `internal/server/pir_okvs_integration_test.go` - PIR + OKVS integration tests (2 tests)

**Documentation:**
- `OKVS_INTEGRATION_PLAN.md` - Comprehensive integration plan and status

**Build System:**
- OKVS FFI library built automatically when building with `-tags cgo`
- Static library: `librbokvsffi.a` (40MB)

## Current System Capabilities

✅ **Core Features Complete:**
- Raft-based distributed key-value store
- gRPC API with round-based coordination
- Worker aggregation and synchronization
- **FrodoPIR Integration:** Private information retrieval
- **OKVS Integration:** Oblivious key-value storage
- **OKVS + PIR Integration:** Combined oblivious storage and private queries

✅ **Architecture:**
- **Temporary Storage:** Pairs accumulated in `roundState.workerData` until all workers submit
- **OKVS Encoding:** Automatic when 100+ pairs (float64 values, 8 bytes)
- **PIR Database:** Created from OKVS-decoded values (maintains oblivious property)
- **Graceful Fallback:** Direct PIR when < 100 pairs (OKVS requirement not met)
- **Storage:** OKVS blob or raw pairs stored in Raft cluster


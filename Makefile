.PHONY: proto build build-client build-multi-worker-test build-load-test build-algorithm-runner build-tools test clean run deps \
	build-pir clean-pir test-pir \
	build-okvs clean-okvs test-okvs \
	test-pir-integration test-okvs-unit test-okvs-integration test-pir-okvs \
	bench bench-store bench-server bench-pir bench-okvs \
	test-no-cgo fmt test-runtime test-cluster test-multi-worker test-load \
	test-degree-collector

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Binary name
BINARY_NAME=silhouette-server
BINARY_DIR=bin

# Protocol buffer compiler
PROTOC=protoc
PROTO_DIR=api/v1
PROTO_FILE=$(PROTO_DIR)/silhouette.proto
PROTO_OUT=api/v1

# Generate Protocol Buffer code
proto:
	@echo "Generating Protocol Buffer code..."
	$(PROTOC) --go_out=$(PROTO_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_FILE)
	@echo "Protocol Buffer code generated"

# Rust/FrodoPIR build targets
PIR_FFI_DIR := third_party/frodo-pir-ffi

build-pir:
	@echo "Building FrodoPIR FFI library..."
	@cd $(PIR_FFI_DIR) && cargo build --release
	@echo "FrodoPIR FFI library built"

clean-pir:
	@echo "Cleaning FrodoPIR FFI library..."
	@cd $(PIR_FFI_DIR) && cargo clean
	@echo "FrodoPIR FFI library cleaned"

test-pir:
	@echo "Testing FrodoPIR FFI library..."
	@cd $(PIR_FFI_DIR) && cargo test --release
	@echo "FrodoPIR FFI library tests passed"

# Rust/OKVS build targets
OKVS_FFI_DIR := third_party/rb-okvs-ffi

build-okvs:
	@echo "Building OKVS FFI library..."
	@cd $(OKVS_FFI_DIR) && cargo build --release
	@echo "OKVS FFI library built"

clean-okvs:
	@echo "Cleaning OKVS FFI library..."
	@cd $(OKVS_FFI_DIR) && cargo clean
	@echo "OKVS FFI library cleaned"

test-okvs:
	@echo "Testing OKVS FFI library..."
	@cd $(OKVS_FFI_DIR) && cargo test --release
	@echo "OKVS FFI library tests passed"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Build the project (requires PIR and OKVS libraries)
build: proto build-pir build-okvs
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -tags cgo -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/silhouette-server
	@echo "Build complete: $(BINARY_DIR)/$(BINARY_NAME)"

# Build test client
build-client:
	@echo "Building test client..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -tags cgo -o $(BINARY_DIR)/test-client ./cmd/test-client
	@echo "Build complete: $(BINARY_DIR)/test-client"

# Build multi-worker test
build-multi-worker-test:
	@echo "Building multi-worker test..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -tags cgo -o $(BINARY_DIR)/multi-worker-test ./cmd/multi-worker-test
	@echo "Build complete: $(BINARY_DIR)/multi-worker-test"

# Build load test
build-load-test:
	@echo "Building load test..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -tags cgo -o $(BINARY_DIR)/load-test ./cmd/load-test
	@echo "Build complete: $(BINARY_DIR)/load-test"

# Build algorithm runner
build-algorithm-runner:
	@echo "Building algorithm runner..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -tags cgo -o $(BINARY_DIR)/algorithm-runner ./cmd/algorithm-runner
	@echo "Build complete: $(BINARY_DIR)/algorithm-runner"

# Run tests (with cgo for PIR)
test:
	@echo "Running tests..."
	$(GOTEST) -tags cgo -v ./...

# Run tests without cgo (faster, but skips PIR tests)
test-no-cgo:
	@echo "Running tests (no cgo)..."
	$(GOTEST) -v ./...

# Run PIR integration tests
test-pir-integration:
	@echo "Running PIR integration tests..."
	$(GOTEST) -tags cgo -v ./internal/server/... -run TestPIRIntegration

# Run OKVS unit tests
test-okvs-unit:
	@echo "Running OKVS unit tests..."
	$(GOTEST) -tags cgo -v ./internal/crypto/... -run TestRBOKVS

# Run OKVS integration tests
test-okvs-integration:
	@echo "Running OKVS integration tests..."
	$(GOTEST) -tags cgo -v ./internal/server/... -run TestRBOKVSIntegration

# Run PIR + OKVS integration tests
test-pir-okvs:
	@echo "Running PIR + OKVS integration tests..."
	$(GOTEST) -tags cgo -v ./internal/server/... -run TestPIR_OKVS

# Run benchmarks (with cgo for PIR)
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -tags cgo -bench=. -benchmem -run=^$$ ./...

# Run benchmarks for a specific package
bench-store:
	@echo "Running Store benchmarks..."
	$(GOTEST) -tags cgo -bench=. -benchmem -run=^$$ ./internal/store/...

bench-server:
	@echo "Running Server benchmarks..."
	$(GOTEST) -tags cgo -bench=. -benchmem -run=^$$ ./internal/server/...

# Run PIR benchmarks
bench-pir:
	@echo "Running PIR benchmarks..."
	$(GOTEST) -tags cgo -bench=BenchmarkPIR -benchmem -run=^$$ ./internal/server/...

# Run OKVS benchmarks
bench-okvs:
	@echo "Running OKVS benchmarks..."
	$(GOTEST) -tags cgo -bench=BenchmarkOKVS -benchmem -run=^$$ ./internal/crypto/...
	$(GOTEST) -tags cgo -bench=BenchmarkOKVS -benchmem -run=^$$ ./internal/server/...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Run the server (node1)
run: build
	@echo "Running server..."
	./$(BINARY_DIR)/$(BINARY_NAME) \
		-node-id=node1 \
		-listen-addr=127.0.0.1:8080 \
		-grpc-addr=127.0.0.1:9090 \
		-data-dir=./data/node1 \
		-bootstrap=true

# Run runtime tests (single node)
test-runtime:
	@echo "Running runtime tests..."
	@./scripts/test-runtime.sh

# Run cluster tests
# Usage: make test-cluster NUM_NODES=3
test-cluster:
	@echo "Running cluster tests with $(or $(NUM_NODES),3) nodes..."
	@./scripts/test-cluster.sh $(or $(NUM_NODES),3)

# Run multi-worker tests
# Usage: make test-multi-worker [SERVER] [NUM_WORKERS] [PAIRS_PER_WORKER] [ROUND_ID]
test-multi-worker:
	@echo "Running multi-worker tests..."
	@./scripts/test-multi-worker.sh $(or $(SERVER),127.0.0.1:9090) $(or $(NUM_WORKERS),10) $(or $(PAIRS_PER_WORKER),20) $(or $(ROUND_ID),100)

# Run load tests
# Usage: make test-load [SERVER] [NUM_ROUNDS] [PAIRS] [WORKERS] [QPS] [DURATION]
test-load:
	@echo "Running load tests..."
	@./scripts/test-load.sh $(or $(SERVER),127.0.0.1:9090) $(or $(NUM_ROUNDS),10) $(or $(PAIRS),150) $(or $(WORKERS),5) $(or $(QPS),10.0) $(or $(DURATION),30)

# Test degree-collector algorithm
test-degree-collector:
	@echo "Running degree-collector tests..."
	@./scripts/test-degree-collector.sh

# Clean build artifacts
clean: clean-pir clean-okvs
	@echo "Cleaning..."
	rm -rf $(BINARY_DIR)
	rm -rf ./data
	rm -rf ./test-runtime
	rm -rf ./test-cluster
	$(GOCMD) clean


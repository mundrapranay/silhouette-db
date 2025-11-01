.PHONY: proto build test clean run deps

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

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Build the project (requires PIR library)
build: proto build-pir
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -tags cgo -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/silhouette-server
	@echo "Build complete: $(BINARY_DIR)/$(BINARY_NAME)"

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

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BINARY_DIR)
	rm -rf ./data
	$(GOCMD) clean


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

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Build the project
build: proto
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd/silhouette-server
	@echo "Build complete: $(BINARY_DIR)/$(BINARY_NAME)"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

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


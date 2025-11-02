# Setup Instructions for silhouette-db

This document provides step-by-step instructions to set up the development environment for `silhouette-db`.

## Prerequisites

1. **Go 1.21 or later**
   ```bash
   go version
   ```

2. **Protocol Buffer Compiler (`protoc`)**
   ```bash
   # macOS
   brew install protobuf
   
   # Linux
   sudo apt-get install protobuf-compiler
   
   # Verify installation
   protoc --version
   ```

3. **Go Protocol Buffer plugins**
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/googleapis/cmd/protoc-gen-go-grpc@latest
   
   # Make sure $GOPATH/bin is in your PATH
   export PATH=$PATH:$(go env GOPATH)/bin
   ```

4. **Git** (for submodules)

## Initial Setup

### 1. Initialize Git Repository (if not already done)

```bash
cd /path/to/silhouette-db
git init
```

### 2. Download Dependencies

```bash
go mod download
go mod tidy
```

### 3. Generate Protocol Buffer Code

```bash
make proto
```

This will generate the Go code from `api/v1/silhouette.proto` into `api/v1/`.

### 4. Verify Build

```bash
go build ./...
```

If this succeeds, your setup is complete!

## Setting Up FrodoPIR Submodule

### Option 1: If Git Repository is Initialized

```bash
# Add frodo-pir as a submodule
git submodule add https://github.com/brave-experiments/frodo-pir.git third_party/frodo-pir

# Initialize and update submodules
git submodule update --init --recursive
```

### Option 2: Clone Without Submodule Support

```bash
# Create directory
mkdir -p third_party

# Clone the repository
git clone https://github.com/brave-experiments/frodo-pir.git third_party/frodo-pir
```

### Option 3: Manual Download

If you prefer not to use git submodules:

```bash
mkdir -p third_party
cd third_party
# Download and extract frodo-pir manually, or
wget https://github.com/brave-experiments/frodo-pir/archive/refs/heads/main.zip
unzip main.zip
mv frodo-pir-main frodo-pir
```

## Building the Project

```bash
# Generate proto code and build
make build

# This will create bin/silhouette-server
```

## Running the Server

### Single Node (Bootstrap)

```bash
make run

# Or manually:
./bin/silhouette-server \
  -node-id=node1 \
  -listen-addr=127.0.0.1:8080 \
  -grpc-addr=127.0.0.1:9090 \
  -data-dir=./data/node1 \
  -bootstrap=true
```

### Multiple Nodes

In separate terminals:

**Terminal 1 (Leader):**
```bash
./bin/silhouette-server \
  -node-id=node1 \
  -listen-addr=127.0.0.1:8080 \
  -grpc-addr=127.0.0.1:9090 \
  -data-dir=./data/node1 \
  -bootstrap=true
```

**Terminal 2 (Follower):**
```bash
./bin/silhouette-server \
  -node-id=node2 \
  -listen-addr=127.0.0.1:8081 \
  -grpc-addr=127.0.0.1:9091 \
  -data-dir=./data/node2 \
  -join=127.0.0.1:8080
```

## Troubleshooting

### Protocol Buffer Generation Fails

- Ensure `protoc` is installed and in PATH
- Ensure `protoc-gen-go` and `protoc-gen-go-grpc` are installed and in PATH
- Check that `$GOPATH/bin` is in your PATH

### Import Errors

If you see import errors:
1. Run `go mod download` and `go mod tidy`
2. Make sure you've run `make proto` to generate the Protocol Buffer code
3. Restart your IDE/editor to pick up the new files

### Build Errors Related to FrodoPIR

FrodoPIR integration is not yet complete. The current implementation uses mock crypto components. Errors related to FrodoPIR are expected until Phase 4 of the implementation plan is completed.

## Next Steps

See [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md) for the detailed implementation roadmap.


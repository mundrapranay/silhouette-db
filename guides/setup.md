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

### 1. Clone the Repository

```bash
git clone https://github.com/mundrapranay/silhouette-db.git
cd silhouette-db
```

### 2. Initialize Submodules and Apply Patches

**Important:** The `rb-okvs` submodule requires patches to be applied for the codebase to work correctly.

```bash
# Option 1: Use Makefile target (recommended)
make submodule-init

# Option 2: Manual initialization
git submodule update --init --recursive
./scripts/apply-patches.sh
```

The `submodule-init` target will:
1. Initialize all git submodules
2. Update them to the correct commits
3. Apply necessary patches (including feature gate fix for rb-okvs)

### 3. Download Go Dependencies

```bash
go mod download
go mod tidy
```

### 4. Generate Protocol Buffer Code

```bash
make proto
```

This will generate the Go code from `api/v1/silhouette.proto` into `api/v1/`.

### 5. Verify Build

```bash
make build
```

Or manually:

```bash
go build ./...
```

If this succeeds, your setup is complete!

## Submodule Management

The project uses git submodules for third-party dependencies:

- **FrodoPIR**: `third_party/frodo-pir` - Private Information Retrieval library
- **RB-OKVS**: `third_party/rb-okvs` - Oblivious Key-Value Store library

### Initializing Submodules

Submodules are automatically initialized when using `make submodule-init`, but you can also initialize them manually:

```bash
# Initialize all submodules
git submodule update --init --recursive

# Apply patches (required for rb-okvs)
./scripts/apply-patches.sh
```

### Applying Patches

Some submodules require patches to work with this project:

- **rb-okvs**: Requires feature gate fix and tests directory

Patches are stored in `patches/rb-okvs/` and are automatically applied by `scripts/apply-patches.sh`.

To manually apply patches:

```bash
cd third_party/rb-okvs
git apply ../../patches/rb-okvs/0001-feature-gate-fix-and-tests.patch
```

### Updating Submodules

To update submodules to the latest upstream versions:

```bash
git submodule update --remote --recursive
./scripts/apply-patches.sh  # Reapply patches after update
```

**Note:** After updating submodules, patches may need to be recreated if upstream changes conflict with local modifications.

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


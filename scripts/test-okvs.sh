#!/bin/bash
# Test script for OKVS (oblivious key-value store) backend

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Testing OKVS Backend ===${NC}"
echo ""

# Check if cgo is available
if ! go env CGO_ENABLED | grep -q "1"; then
    echo -e "${YELLOW}Warning: CGO is not enabled. OKVS requires CGO.${NC}"
    echo "Skipping OKVS test."
    exit 0
fi

# Build server with OKVS backend
echo -e "${YELLOW}Building server with OKVS backend...${NC}"
CGO_ENABLED=1 go build -o bin/silhouette-server-okvs ./cmd/silhouette-server/
echo -e "${GREEN}✓ Server built${NC}"
echo ""

# Test directory
TEST_DIR="test-okvs"
mkdir -p "$TEST_DIR"

# Start server
echo -e "${YELLOW}Starting server with OKVS backend...${NC}"
./bin/silhouette-server-okvs \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8082 \
    -grpc-addr=127.0.0.1:9092 \
    -data-dir=$TEST_DIR/data \
    -bootstrap \
    -storage-backend=okvs \
    > "$TEST_DIR/server.log" 2>&1 &
SERVER_PID=$!

sleep 2

# Check if server is running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo -e "${RED}Server failed to start${NC}"
    cat "$TEST_DIR/server.log"
    exit 1
fi

echo -e "${GREEN}✓ Server started (PID: $SERVER_PID)${NC}"
echo ""

# Run OKVS integration test
echo -e "${YELLOW}Running OKVS integration test...${NC}"
CGO_ENABLED=1 go test ./internal/server -run TestOKVS -v
TEST_RESULT=$?

echo ""

# Cleanup
echo -e "${YELLOW}Stopping server...${NC}"
kill $SERVER_PID 2>/dev/null || true
sleep 1
kill -9 $SERVER_PID 2>/dev/null || true

if [ $TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}=== OKVS Backend Test Passed ===${NC}"
    exit 0
else
    echo -e "${RED}=== OKVS Backend Test Failed ===${NC}"
    exit 1
fi


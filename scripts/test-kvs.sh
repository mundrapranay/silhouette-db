#!/bin/bash
# Test script for KVS (simple key-value store) backend

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}=== Testing KVS Backend ===${NC}"
echo ""

# Build server with KVS backend
echo -e "${YELLOW}Building server with KVS backend...${NC}"
go build -o bin/silhouette-server-kvs ./cmd/silhouette-server/
echo -e "${GREEN}✓ Server built${NC}"
echo ""

# Test directory
TEST_DIR="test-kvs"
mkdir -p "$TEST_DIR"

# Start server
echo -e "${YELLOW}Starting server with KVS backend...${NC}"
./bin/silhouette-server-kvs \
    -node-id=node1 \
    -listen-addr=127.0.0.1:8081 \
    -grpc-addr=127.0.0.1:9091 \
    -data-dir=$TEST_DIR/data \
    -bootstrap \
    -storage-backend=kvs \
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

# Run simple test with client
echo -e "${YELLOW}Running KVS integration test...${NC}"
go test ./internal/server -run TestServer_KVS_Integration -v
TEST_RESULT=$?

echo ""

# Cleanup
echo -e "${YELLOW}Stopping server...${NC}"
kill $SERVER_PID 2>/dev/null || true
sleep 1
kill -9 $SERVER_PID 2>/dev/null || true

if [ $TEST_RESULT -eq 0 ]; then
    echo -e "${GREEN}=== KVS Backend Test Passed ===${NC}"
    exit 0
else
    echo -e "${RED}=== KVS Backend Test Failed ===${NC}"
    exit 1
fi


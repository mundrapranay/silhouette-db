#!/bin/bash
# Manual Runtime Testing Script for silhouette-db
# This script helps test the complete system end-to-end

set -e

SERVER_ADDR="127.0.0.1:9090"
BINARY_DIR="./bin"
TEST_DIR="./test-runtime"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🚀 silhouette-db Manual Runtime Testing"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "🧹 Cleaning up..."
    if [ -n "$SERVER_PID" ]; then
        echo "   Stopping server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
    rm -rf "$TEST_DIR"
    echo "✅ Cleanup complete"
}

trap cleanup EXIT

# Check if binaries exist
if [ ! -f "$BINARY_DIR/silhouette-server" ]; then
    echo "❌ Server binary not found. Building..."
    make build
fi

if [ ! -f "$BINARY_DIR/test-client" ]; then
    echo "❌ Test client binary not found. Building..."
    make build-client
fi

# Create test directory
mkdir -p "$TEST_DIR"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Test 1: OKVS Encoding (150 pairs > 100 minimum)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Start server in background
echo "🔧 Starting server..."
"$BINARY_DIR/silhouette-server" \
    -node-id=test-node \
    -listen-addr=127.0.0.1:8080 \
    -grpc-addr="$SERVER_ADDR" \
    -data-dir="$TEST_DIR/node1" \
    -bootstrap=true > "$TEST_DIR/server.log" 2>&1 &
SERVER_PID=$!

echo "   Server started (PID: $SERVER_PID)"
echo "   Waiting for server to be ready..."

# Wait for server to start
sleep 3

# Check if server is still running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo "❌ Server failed to start!"
    echo "   Last 20 lines of server log:"
    tail -20 "$TEST_DIR/server.log"
    exit 1
fi

echo "✅ Server is ready!"
echo ""

# Test with 150 pairs (OKVS encoding)
echo "🧪 Running test client with 150 pairs (OKVS encoding)..."
"$BINARY_DIR/test-client" \
    -server="$SERVER_ADDR" \
    -pairs=150 \
    -round=1

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Test 1 PASSED: OKVS encoding works correctly!"
else
    echo ""
    echo "❌ Test 1 FAILED: OKVS encoding test failed"
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Test 2: Direct PIR Fallback (50 pairs < 100 minimum)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test with 50 pairs (direct PIR, no OKVS)
echo "🧪 Running test client with 50 pairs (direct PIR fallback)..."
"$BINARY_DIR/test-client" \
    -server="$SERVER_ADDR" \
    -pairs=50 \
    -round=2

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Test 2 PASSED: Direct PIR fallback works correctly!"
else
    echo ""
    echo "❌ Test 2 FAILED: Direct PIR fallback test failed"
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Test 3: Query Specific Key"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Test querying a specific key
echo "🧪 Running test client to query specific key..."
"$BINARY_DIR/test-client" \
    -server="$SERVER_ADDR" \
    -pairs=100 \
    -round=3 \
    -key="test-key-050"

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Test 3 PASSED: Specific key query works correctly!"
else
    echo ""
    echo "❌ Test 3 FAILED: Specific key query test failed"
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ All Runtime Tests PASSED!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""


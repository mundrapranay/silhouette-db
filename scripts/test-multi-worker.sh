#!/bin/bash
# Multi-Worker Testing Script for silhouette-db
# Tests the system with multiple concurrent workers

set -e

# Configuration
SERVER_ADDR=${1:-"127.0.0.1:9090"}
NUM_WORKERS=${2:-10}
PAIRS_PER_WORKER=${3:-20}
ROUND_ID=${4:-100}
BINARY_DIR="./bin"
TEST_DIR="./test-multi-worker"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🚀 silhouette-db Multi-Worker Testing"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📋 Configuration:"
echo "   Server address:   $SERVER_ADDR"
echo "   Number of workers: $NUM_WORKERS"
echo "   Pairs per worker:  $PAIRS_PER_WORKER"
echo "   Total pairs:      $((NUM_WORKERS * PAIRS_PER_WORKER))"
echo "   Round ID:         $ROUND_ID"
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

if [ ! -f "$BINARY_DIR/multi-worker-test" ]; then
    echo "❌ Multi-worker test binary not found. Building..."
    make build-multi-worker-test
fi

# Create test directory
mkdir -p "$TEST_DIR"

# Start server
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

# Run multi-worker test
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Test: Multi-Worker Aggregation"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

"$BINARY_DIR/multi-worker-test" \
    -server="$SERVER_ADDR" \
    -workers=$NUM_WORKERS \
    -pairs=$PAIRS_PER_WORKER \
    -round=$ROUND_ID

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Multi-worker test PASSED!"
else
    echo ""
    echo "❌ Multi-worker test FAILED"
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Multi-Worker Testing Complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""


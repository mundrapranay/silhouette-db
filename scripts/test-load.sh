#!/bin/bash
# Load Testing Script for silhouette-db
# Tests the system under load with concurrent rounds and high query rates

set -e

# Configuration
SERVER_ADDR=${1:-"127.0.0.1:9090"}
NUM_ROUNDS=${2:-10}
PAIRS_PER_ROUND=${3:-150}
WORKERS_PER_ROUND=${4:-5}
QPS=${5:-10.0}
DURATION=${6:-30}
BINARY_DIR="./bin"
TEST_DIR="./test-load"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🚀 silhouette-db Load Testing"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📋 Configuration:"
echo "   Server address:    $SERVER_ADDR"
echo "   Concurrent rounds: $NUM_ROUNDS"
echo "   Pairs per round:   $PAIRS_PER_ROUND"
echo "   Workers per round: $WORKERS_PER_ROUND"
echo "   Queries per sec:   $QPS"
echo "   Test duration:     ${DURATION}s"
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

if [ ! -f "$BINARY_DIR/load-test" ]; then
    echo "❌ Load test binary not found. Building..."
    make build-load-test
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

# Run load test
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Starting Load Test"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

"$BINARY_DIR/load-test" \
    -server="$SERVER_ADDR" \
    -rounds=$NUM_ROUNDS \
    -pairs=$PAIRS_PER_ROUND \
    -workers=$WORKERS_PER_ROUND \
    -qps=$QPS \
    -duration=${DURATION}s

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Load test PASSED!"
else
    echo ""
    echo "⚠️  Load test completed with warnings"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Load Testing Complete!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""


#!/bin/bash
# Test script for degree-collector algorithm in local testing mode
# This script sets up and runs the degree-collector algorithm with multiple workers

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
NUM_WORKERS=${NUM_WORKERS:-3}
NUM_VERTICES=${NUM_VERTICES:-20}
NUM_EDGES=${NUM_EDGES:-30}
SEED=${SEED:-42}
SERVER_ADDR=${SERVER_ADDR:-"127.0.0.1:9090"}
CONFIG_FILE="configs/degree_collector.yaml"
TEST_DIR="test-degree-collector"

echo -e "${GREEN}=== Degree Collector Local Testing ===${NC}"
echo "Workers: $NUM_WORKERS"
echo "Vertices: $NUM_VERTICES"
echo "Edges: $NUM_EDGES"
echo "Seed: $SEED"
echo ""

# Step 1: Clean up previous test
echo -e "${YELLOW}Step 1: Cleaning up previous test...${NC}"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"

# Step 2: Build algorithm runner
echo -e "${YELLOW}Step 2: Building algorithm runner...${NC}"
if ! make build-algorithm-runner > /dev/null 2>&1; then
    echo -e "${RED}Failed to build algorithm-runner${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Build complete${NC}"

# Step 3: Generate test graph
echo -e "${YELLOW}Step 3: Generating test graph...${NC}"
python3 data-generation/generate_graph.py \
    --config "$CONFIG_FILE" \
    --num-vertices "$NUM_VERTICES" \
    --num-edges "$NUM_EDGES" \
    --seed "$SEED" \
    --output-dir data \
    --global-graph "$TEST_DIR/graph.txt" 2>&1 | grep -E "(Generating|Generated|Writing|Worker|✓)" || true
echo -e "${GREEN}✓ Graph generated${NC}"

# Step 4: Start silhouette-db server
echo -e "${YELLOW}Step 4: Starting silhouette-db server...${NC}"

# Check if binaries exist
if [ ! -f ./bin/silhouette-server ]; then
    echo -e "${YELLOW}Building server...${NC}"
    make build > /dev/null 2>&1
fi

# Check if server is already running on port 9090
if lsof -Pi :9090 -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo -e "${YELLOW}Server already running on port 9090${NC}"
    SERVER_PID=""
else
    # Start server using command-line flags (like other test scripts)
    # Use KVS backend for degree-collector (works with any number of pairs)
    STORAGE_BACKEND=${STORAGE_BACKEND:-kvs}
    ./bin/silhouette-server \
        -node-id=test-node \
        -listen-addr=127.0.0.1:8080 \
        -grpc-addr="$SERVER_ADDR" \
        -data-dir="$TEST_DIR/node1" \
        -bootstrap=true \
        -storage-backend="$STORAGE_BACKEND" > "$TEST_DIR/server.log" 2>&1 &
    SERVER_PID=$!
    echo "Server started (PID: $SERVER_PID)"
    
    # Wait for server to start
    echo "Waiting for server to be ready..."
    sleep 3
    
    # Check if server is still running
    if ! kill -0 $SERVER_PID 2>/dev/null; then
        echo -e "${RED}Server failed to start!${NC}"
        echo "Last 20 lines of server log:"
        tail -20 "$TEST_DIR/server.log"
        exit 1
    fi
    
    # Additional check: try to connect to the server
    # Try multiple methods to verify server is ready
    server_ready=false
    for i in {1..10}; do
        # Check if process is still running
        if ! kill -0 $SERVER_PID 2>/dev/null; then
            echo -e "${RED}Server process died!${NC}"
            echo "Last 20 lines of server log:"
            tail -20 "$TEST_DIR/server.log"
            exit 1
        fi
        
        # Try to connect to gRPC port (9090)
        if command -v nc >/dev/null 2>&1; then
            if nc -z 127.0.0.1 9090 2>/dev/null; then
                server_ready=true
                break
            fi
        else
            # Fallback: if nc not available, just check process is alive
            # and wait a bit more
            if [ $i -ge 3 ]; then
                server_ready=true
                break
            fi
        fi
        sleep 1
    done
    
    if [ "$server_ready" = true ]; then
        echo -e "${GREEN}✓ Server ready${NC}"
    else
        echo -e "${RED}Server may not be ready (but process is running)${NC}"
        echo "Continuing anyway... (check logs if tests fail)"
    fi
fi

# Step 5: Run workers in parallel
echo -e "${YELLOW}Step 5: Running $NUM_WORKERS workers...${NC}"

# Create config files for each worker
for i in $(seq 0 $((NUM_WORKERS - 1))); do
    worker_config="$TEST_DIR/worker-${i}.yaml"
    cp "$CONFIG_FILE" "$worker_config"
    
    # Update worker_id in config using sed or Python
    python3 <<EOF
import yaml
with open('$CONFIG_FILE', 'r') as f:
    config = yaml.safe_load(f)
config['worker_config']['worker_id'] = 'worker-$i'
with open('$worker_config', 'w') as f:
    yaml.dump(config, f, default_flow_style=False, sort_keys=False)
EOF
done

# Run all workers
echo "Starting workers..."
for i in $(seq 0 $((NUM_WORKERS - 1))); do
    worker_config="$TEST_DIR/worker-${i}.yaml"
    echo "  Starting worker-$i..."
    ./bin/algorithm-runner -config "$worker_config" -verbose > "$TEST_DIR/worker-${i}.log" 2>&1 &
    WORKER_PIDS[$i]=$!
done

# Wait for all workers to complete
echo "Waiting for workers to complete..."
wait "${WORKER_PIDS[@]}"
echo -e "${GREEN}✓ All workers completed${NC}"

# Step 6: Check results
echo -e "${YELLOW}Step 6: Checking results...${NC}"

all_passed=true
for i in $(seq 0 $((NUM_WORKERS - 1))); do
    result_file="degree_collector_results_worker-${i}.txt"
    log_file="$TEST_DIR/worker-${i}.log"
    
    if [ -f "$result_file" ]; then
        num_results=$(grep -v '^#' "$result_file" | grep -v '^$' | wc -l | tr -d ' ')
        echo -e "  ${GREEN}✓ Worker-$i: $num_results results in $result_file${NC}"
        
        # Show first few results
        if [ "$num_results" -gt 0 ]; then
            echo "    Sample results:"
            head -3 "$result_file" | sed 's/^/      /'
        fi
    else
        echo -e "  ${RED}✗ Worker-$i: Result file not found${NC}"
        all_passed=false
    fi
    
    # Check for errors in log
    if grep -i "error\|failed\|fatal" "$log_file" > /dev/null 2>&1; then
        echo -e "  ${RED}✗ Worker-$i: Errors in log:${NC}"
        grep -i "error\|failed\|fatal" "$log_file" | head -3 | sed 's/^/      /'
        all_passed=false
    fi
done

# Step 7: Verify correctness (optional - basic check)
echo -e "${YELLOW}Step 7: Verifying correctness...${NC}"
python3 <<EOF
import glob
import sys

result_files = sorted(glob.glob('degree_collector_results_worker-*.txt'))
if not result_files:
    print("No result files found")
    sys.exit(1)

print(f"Found {len(result_files)} result files")
for f in result_files:
    with open(f, 'r') as rf:
        lines = [l for l in rf if not l.startswith('#') and l.strip()]
        print(f"  {f}: {len(lines)} results")
print("✓ Result files found")
EOF

# Final cleanup
if [ "$all_passed" = true ]; then
    echo -e "${GREEN}=== Test Passed ===${NC}"
    
    # Kill server if we started it (unless --keep flag)
    if [ "$1" != "--keep" ] && [ -n "$SERVER_PID" ] && [ "$SERVER_PID" != "" ]; then
        echo "Stopping server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        sleep 1
        kill -9 $SERVER_PID 2>/dev/null || true
    fi
    
    exit 0
else
    echo -e "${RED}=== Test Failed ===${NC}"
    
    # Keep server running on failure for debugging
    echo "Server logs: $TEST_DIR/server.log"
    echo "Worker logs: $TEST_DIR/worker-*.log"
    echo "To keep server running for debugging, use --keep flag"
    
    # Still cleanup unless --keep flag
    if [ "$1" != "--keep" ] && [ -n "$SERVER_PID" ] && [ "$SERVER_PID" != "" ]; then
        echo "Stopping server (PID: $SERVER_PID)..."
        kill $SERVER_PID 2>/dev/null || true
        sleep 1
        kill -9 $SERVER_PID 2>/dev/null || true
    fi
    
    exit 1
fi


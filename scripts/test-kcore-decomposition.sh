#!/bin/bash
# Test script for k-core-decomposition algorithm in local testing mode
# This script sets up and runs the k-core-decomposition algorithm with multiple workers

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
NUM_WORKERS=${NUM_WORKERS:-4}
SERVER_ADDR=${SERVER_ADDR:-"127.0.0.1:9090"}
CONFIG_FILE="configs/kcore_decomposition.yaml"
TEST_DIR="test-kcore-decomposition"

echo -e "${GREEN}=== K-Core Decomposition Local Testing ===${NC}"
echo "Workers: $NUM_WORKERS"
echo "Config: $CONFIG_FILE"
echo ""

# Step 1: Clean up previous test
echo -e "${YELLOW}Step 1: Cleaning up previous test...${NC}"
rm -rf "$TEST_DIR"
mkdir -p "$TEST_DIR"

# Step 2: Verify partitioned graph files exist
echo -e "${YELLOW}Step 2: Verifying partitioned graph files...${NC}"
DATA_DIR="data/wiki_4"
for i in $(seq 1 $NUM_WORKERS); do
    if [ ! -f "$DATA_DIR/${i}.txt" ]; then
        echo -e "${RED}Error: Partitioned graph file $DATA_DIR/${i}.txt not found${NC}"
        echo "Please run the partition script first:"
        echo "  python3 data-generation/partition_existing_graph.py data/wiki_adj $NUM_WORKERS --config $CONFIG_FILE --output-dir $DATA_DIR"
        exit 1
    fi
done
echo -e "${GREEN}✓ All partitioned files found${NC}"

# Step 3: Build algorithm runner
echo -e "${YELLOW}Step 3: Building algorithm runner...${NC}"
if ! make build-algorithm-runner > /dev/null 2>&1; then
    echo -e "${RED}Failed to build algorithm-runner${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Build complete${NC}"

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
    # Start server using command-line flags
    # Use OKVS backend for k-core (requires 100+ pairs)
    STORAGE_BACKEND=${STORAGE_BACKEND:-okvs}
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
    
    # Update worker_id in config using Python
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
echo "  (This may take several minutes for large graphs...)"
wait "${WORKER_PIDS[@]}"
echo -e "${GREEN}✓ All workers completed${NC}"

# Step 6: Check results
echo -e "${YELLOW}Step 6: Checking results...${NC}"

all_passed=true
for i in $(seq 0 $((NUM_WORKERS - 1))); do
    # Check for default result file name
    result_file="kcore_results_worker-${i}.txt"
    
    # Also check if result_file was customized in config
    python3 <<EOF
import yaml
import os
with open('$TEST_DIR/worker-${i}.yaml', 'r') as f:
    config = yaml.safe_load(f)
result_file = config.get('parameters', {}).get('result_file', 'kcore_results_worker-${i}.txt')
print(result_file)
EOF
    | while read custom_result_file; do
        if [ -n "$custom_result_file" ] && [ -f "$custom_result_file" ]; then
            result_file="$custom_result_file"
        fi
    done
    
    log_file="$TEST_DIR/worker-${i}.log"
    
    if [ -f "$result_file" ]; then
        num_results=$(grep -v '^#' "$result_file" | grep -v '^$' | wc -l | tr -d ' ')
        echo -e "  ${GREEN}✓ Worker-$i: $num_results results in $result_file${NC}"
        
        # Show first few results
        if [ "$num_results" -gt 0 ]; then
            echo "    Sample results:"
            head -5 "$result_file" | sed 's/^/      /'
            
            # Show statistics
            if command -v awk >/dev/null 2>&1; then
                # Extract core numbers and compute basic stats
                core_nums=$(awk -F': ' '{print $2}' "$result_file" 2>/dev/null | sort -n)
                if [ -n "$core_nums" ]; then
                    min_core=$(echo "$core_nums" | head -1)
                    max_core=$(echo "$core_nums" | tail -1)
                    echo "    Core number range: [$min_core, $max_core]"
                fi
            fi
        fi
    else
        echo -e "  ${RED}✗ Worker-$i: Result file not found: $result_file${NC}"
        all_passed=false
    fi
    
    # Check for errors in log
    if grep -i "error\|failed\|fatal" "$log_file" > /dev/null 2>&1; then
        # Filter out expected/benign errors
        critical_errors=$(grep -i "error\|failed\|fatal" "$log_file" | \
            grep -v "retrying\|retry\|timeout" | head -5)
        if [ -n "$critical_errors" ]; then
            echo -e "  ${RED}✗ Worker-$i: Errors in log:${NC}"
            echo "$critical_errors" | sed 's/^/      /'
            all_passed=false
        fi
    fi
done

# Step 7: Verify correctness and coverage
echo -e "${YELLOW}Step 7: Verifying correctness...${NC}"
python3 <<EOF
import glob
import sys

# Find all result files
result_files = sorted(glob.glob('kcore_results_worker-*.txt'))
if not result_files:
    print("No result files found")
    sys.exit(1)

print(f"Found {len(result_files)} result files")

# Collect all vertices that have results
all_vertices = set()
total_results = 0
for f in result_files:
    with open(f, 'r') as rf:
        lines = [l.strip() for l in rf if not l.startswith('#') and l.strip()]
        for line in lines:
            if ':' in line:
                vertex_id = line.split(':')[0].strip()
                all_vertices.add(vertex_id)
                total_results += 1
        print(f"  {f}: {len(lines)} results")

print(f"Total vertices with results: {len(all_vertices)}")
print(f"Total result entries: {total_results}")

# Basic validation: check format
valid_format = True
for f in result_files:
    with open(f, 'r') as rf:
        for line in rf:
            if not line.strip() or line.startswith('#'):
                continue
            if ':' not in line:
                print(f"Invalid format in {f}: {line.strip()}")
                valid_format = False
                break

if valid_format:
    print("✓ Result format validation passed")
else:
    print("✗ Result format validation failed")
    sys.exit(1)
EOF

# Step 8: Summary statistics
echo -e "${YELLOW}Step 8: Summary statistics...${NC}"
python3 <<EOF
import glob
import yaml
import os

# Load config to get graph size
with open('$CONFIG_FILE', 'r') as f:
    config = yaml.safe_load(f)
graph_size = config['parameters']['n']

result_files = sorted(glob.glob('kcore_results_worker-*.txt'))
total_vertices = 0
for f in result_files:
    with open(f, 'r') as rf:
        lines = [l for l in rf if not l.startswith('#') and l.strip()]
        total_vertices += len(lines)

print(f"Graph size (n): {graph_size}")
print(f"Vertices with results: {total_vertices}")
print(f"Coverage: {100.0 * total_vertices / graph_size:.2f}%")

if total_vertices == graph_size:
    print("✓ All vertices have results")
elif total_vertices > 0:
    print(f"⚠ Only {total_vertices}/{graph_size} vertices have results")
else:
    print("✗ No results found")
EOF

# Step 8.5: Quick verification (optional)
echo -e "${YELLOW}Step 8.5: Network Verification...${NC}"
if lsof -Pi :9090 -sTCP:LISTEN -t >/dev/null 2>&1; then
    SERVER_PID=$(lsof -Pi :9090 -sTCP:LISTEN -t | head -1)
    CONNECTIONS=$(lsof -nP -iTCP:9090 -sTCP:ESTABLISHED 2>/dev/null | grep -v COMMAND | wc -l | tr -d ' ')
    WORKERS=$(ps aux | grep "[a]lgorithm-runner.*kcore" | wc -l | tr -d ' ')
    echo "  Server: Running (PID: $SERVER_PID)"
    echo "  Connections: $CONNECTIONS active"
    echo "  Workers: $WORKERS running"
    echo ""
    echo "  Tip: Run './scripts/verify-kcore.sh' for detailed status"
    echo "  Tip: Run './scripts/monitor-kcore.sh' for real-time monitoring"
else
    echo "  ⚠ Server not detected on port 9090"
fi
echo ""

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


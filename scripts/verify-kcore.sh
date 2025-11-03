#!/bin/bash
# Quick verification script for k-core decomposition

set -e

PORT=${PORT:-9090}

echo "=== K-Core Decomposition Verification ==="
echo ""

# 1. Check server
echo "1. Server Status:"
if lsof -Pi :$PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
    SERVER_PID=$(lsof -Pi :$PORT -sTCP:LISTEN -t | head -1)
    echo "   ✓ Server running on port $PORT (PID: $SERVER_PID)"
    
    # Get server process info
    ps -p $SERVER_PID -o pid,comm,etime,rss,vsz 2>/dev/null | tail -1 | awk '{print "   Process: " $2 ", Runtime: " $3 ", Memory: " $4/1024 "MB"}'
else
    echo "   ✗ No server listening on port $PORT"
    exit 1
fi
echo ""

# 2. Check active connections
echo "2. Active Connections:"
CONNECTIONS=$(lsof -nP -iTCP:$PORT -sTCP:ESTABLISHED 2>/dev/null | grep -v COMMAND | wc -l | tr -d ' ')
if [ "$CONNECTIONS" -gt 0 ]; then
    echo "   ✓ $CONNECTIONS active connection(s):"
    lsof -nP -iTCP:$PORT -sTCP:ESTABLISHED 2>/dev/null | tail -n +2 | awk '{print "     " $1 " (PID " $2 ") -> " $9}'
else
    echo "   ⚠ No active connections"
fi
echo ""

# 3. Check worker processes
echo "3. Worker Processes:"
WORKER_PIDS=$(ps aux | grep "[a]lgorithm-runner.*kcore" | awk '{print $2}' | tr '\n' ' ')
if [ -n "$WORKER_PIDS" ]; then
    COUNT=$(echo $WORKER_PIDS | wc -w | tr -d ' ')
    echo "   ✓ $COUNT worker process(es) running:"
    for pid in $WORKER_PIDS; do
        ps -p $pid -o pid,comm,etime,args 2>/dev/null | tail -1 | awk '{print "     PID: " $1 ", Runtime: " $3 ", Config: " $4}'
    done
else
    echo "   ⚠ No worker processes found"
fi
echo ""

# 4. Check result files
echo "4. Result Files:"
RESULT_FILES=$(ls kcore_results_worker-*.txt 2>/dev/null | wc -l | tr -d ' ')
if [ "$RESULT_FILES" -gt 0 ]; then
    echo "   ✓ $RESULT_FILES result file(s) found:"
    for f in kcore_results_worker-*.txt; do
        if [ -f "$f" ]; then
            LINES=$(wc -l < "$f" | tr -d ' ')
            SIZE=$(ls -lh "$f" | awk '{print $5}')
            echo "     $f: $LINES results, $SIZE"
        fi
    done
else
    echo "   ⚠ No result files found yet"
fi
echo ""

# 5. Check recent server logs
if [ -f "test-kcore-decomposition/server.log" ]; then
    echo "5. Recent Server Log Activity:"
    echo "   Last 5 lines:"
    tail -5 test-kcore-decomposition/server.log 2>/dev/null | sed 's/^/     /' || echo "     (no activity)"
    echo ""
fi

# 6. Network traffic summary
echo "6. Network Traffic Summary:"
if command -v netstat >/dev/null 2>&1; then
    ESTABLISHED=$(netstat -an 2>/dev/null | grep ":$PORT" | grep ESTABLISHED | wc -l | tr -d ' ')
    LISTEN=$(netstat -an 2>/dev/null | grep ":$PORT" | grep LISTEN | wc -l | tr -d ' ')
    echo "   Listening: $LISTEN, Established: $ESTABLISHED"
fi
echo ""

# 7. Test connectivity (if grpcurl is available)
if command -v grpcurl >/dev/null 2>&1; then
    echo "7. gRPC Connectivity Test:"
    if timeout 2 grpcurl -plaintext localhost:$PORT list >/dev/null 2>&1; then
        echo "   ✓ Server is responding to gRPC requests"
        echo "   Available services:"
        grpcurl -plaintext localhost:$PORT list 2>/dev/null | sed 's/^/     /' || true
    else
        echo "   ⚠ Server not responding to gRPC requests"
    fi
    echo ""
fi

echo "=== Verification Complete ==="


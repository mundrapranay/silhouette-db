#!/bin/bash
# Monitor network activity for k-core decomposition algorithm

set -e

PORT=${PORT:-9090}
INTERVAL=${INTERVAL:-2}

echo "=== Monitoring Port $PORT (gRPC Server) ==="
echo "Press Ctrl+C to stop"
echo ""

while true; do
    clear
    echo "=== $(date) ==="
    echo ""
    
    # Check if server is running
    echo "1. Server Status:"
    if lsof -Pi :$PORT -sTCP:LISTEN -t >/dev/null 2>&1; then
        SERVER_PID=$(lsof -Pi :$PORT -sTCP:LISTEN -t | head -1)
        echo "   ✓ Server running (PID: $SERVER_PID)"
    else
        echo "   ✗ No server listening on port $PORT"
    fi
    echo ""
    
    # Active connections
    echo "2. Active Connections:"
    CONNECTIONS=$(lsof -nP -iTCP:$PORT -sTCP:ESTABLISHED 2>/dev/null | grep -v COMMAND | wc -l | tr -d ' ')
    echo "   Active client connections: $CONNECTIONS"
    if [ "$CONNECTIONS" -gt 0 ]; then
        echo "   Connection details:"
        lsof -nP -iTCP:$PORT -sTCP:ESTABLISHED 2>/dev/null | tail -n +2 | awk '{print "     " $1 " -> " $9}' | head -10
    fi
    echo ""
    
    # Check worker processes
    echo "3. Worker Processes:"
    WORKER_COUNT=$(ps aux | grep "[a]lgorithm-runner.*kcore" | wc -l | tr -d ' ')
    echo "   Active workers: $WORKER_COUNT"
    if [ "$WORKER_COUNT" -gt 0 ]; then
        ps aux | grep "[a]lgorithm-runner.*kcore" | awk '{print "     PID: " $2 " - " $11 " " $12}'
    fi
    echo ""
    
    # Network statistics (if available)
    if command -v netstat >/dev/null 2>&1; then
        echo "4. Network Statistics:"
        netstat -an | grep ":$PORT" | grep ESTABLISHED | wc -l | awk '{print "   Established connections: " $1}'
    fi
    echo ""
    
    # Recent server log activity (if available)
    if [ -f "test-kcore-decomposition/server.log" ]; then
        echo "5. Recent Server Activity (last 3 lines):"
        tail -3 test-kcore-decomposition/server.log 2>/dev/null | sed 's/^/   /' || echo "   (no recent activity)"
    fi
    echo ""
    
    echo "Refreshing in $INTERVAL seconds... (Ctrl+C to stop)"
    sleep $INTERVAL
done


#!/bin/bash
# Capture network traffic on port 9090 for k-core decomposition

set -e

PORT=${PORT:-9090}
OUTPUT=${OUTPUT:-kcore-traffic.pcap}

echo "=== Network Traffic Capture for Port $PORT ==="
echo ""
echo "This script will capture network traffic to analyze gRPC requests."
echo ""

# Check if tcpdump is available
if ! command -v tcpdump >/dev/null 2>&1; then
    echo "Error: tcpdump is not installed"
    echo ""
    echo "Install it with:"
    echo "  macOS: brew install tcpdump"
    echo "  Linux: sudo apt-get install tcpdump"
    exit 1
fi

# Check if running as root (required for tcpdump on some systems)
if [ "$EUID" -ne 0 ]; then
    echo "Warning: tcpdump may require root privileges"
    echo "Run with sudo if you get permission errors"
    echo ""
fi

echo "Capturing traffic on port $PORT..."
echo "Output file: $OUTPUT"
echo "Press Ctrl+C to stop"
echo ""

# Capture traffic
tcpdump -i any -w "$OUTPUT" port $PORT -v

echo ""
echo "Capture complete! Output saved to: $OUTPUT"
echo ""
echo "To analyze the capture:"
echo "  1. Install Wireshark: brew install wireshark (macOS) or sudo apt-get install wireshark (Linux)"
echo "  2. Open the .pcap file: wireshark $OUTPUT"
echo "  3. Filter for gRPC: tcp.port == $PORT"
echo ""


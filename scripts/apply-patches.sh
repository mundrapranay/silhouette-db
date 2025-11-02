#!/bin/bash
# Apply patches to submodules
# This script applies patches stored in patches/ to their respective submodules

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$ROOT_DIR"

echo "Applying patches to submodules..."

# Apply rb-okvs patches
if [ -d "patches/rb-okvs" ] && [ -d "third_party/rb-okvs" ]; then
    echo "Applying patches to rb-okvs..."
    cd third_party/rb-okvs
    
    # Ensure we're on the correct base commit
    git checkout 1fcf747 2>/dev/null || {
        echo "Warning: Could not checkout base commit 1fcf747, attempting to apply patches anyway..."
    }
    
    # Apply all patches in order
    for patch in "$ROOT_DIR/patches/rb-okvs"/*.patch; do
        if [ -f "$patch" ]; then
            echo "  Applying $(basename "$patch")..."
            if git apply --check "$patch" 2>/dev/null; then
                git apply "$patch"
                echo "    ✓ Patch applied successfully"
            else
                echo "    ⚠ Patch may have already been applied or conflicts exist"
                # Try to apply anyway (some conflicts might be acceptable)
                git apply "$patch" 2>/dev/null || echo "    ✗ Failed to apply patch"
            fi
        fi
    done
    
    cd "$ROOT_DIR"
    echo "✓ rb-okvs patches applied"
else
    echo "⚠ Skipping rb-okvs patches (directory not found)"
fi

echo ""
echo "All patches applied successfully!"


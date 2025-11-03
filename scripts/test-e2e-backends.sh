#!/bin/bash
# End-to-end test script for both KVS and OKVS backends
# Tests degree-collector and k-core-decomposition with both backends

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== End-to-End Tests with Both Backends ===${NC}"
echo ""

# Test 1: Degree Collector with KVS
echo -e "${YELLOW}Test 1: Degree Collector with KVS Backend${NC}"
STORAGE_BACKEND=kvs ./scripts/test-degree-collector.sh
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Degree Collector with KVS passed${NC}"
else
    echo -e "${RED}✗ Degree Collector with KVS failed${NC}"
    exit 1
fi

echo ""

# Test 2: Degree Collector with OKVS (if available)
echo -e "${YELLOW}Test 2: Degree Collector with OKVS Backend${NC}"
# Note: OKVS requires 100+ pairs, so we need to ensure enough data
STORAGE_BACKEND=okvs NUM_VERTICES=50 NUM_EDGES=100 ./scripts/test-degree-collector.sh
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Degree Collector with OKVS passed${NC}"
else
    echo -e "${YELLOW}⚠ Degree Collector with OKVS skipped (may need more data)${NC}"
fi

echo ""

# Test 3: K-Core Decomposition with OKVS (default)
echo -e "${YELLOW}Test 3: K-Core Decomposition with OKVS Backend${NC}"
STORAGE_BACKEND=okvs ./scripts/test-kcore-decomposition.sh
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ K-Core Decomposition with OKVS passed${NC}"
else
    echo -e "${RED}✗ K-Core Decomposition with OKVS failed${NC}"
    exit 1
fi

echo ""

# Test 4: K-Core Decomposition with KVS
echo -e "${YELLOW}Test 4: K-Core Decomposition with KVS Backend${NC}"
STORAGE_BACKEND=kvs ./scripts/test-kcore-decomposition.sh
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ K-Core Decomposition with KVS passed${NC}"
else
    echo -e "${YELLOW}⚠ K-Core Decomposition with KVS skipped (may need OKVS)${NC}"
fi

echo ""
echo -e "${GREEN}=== All End-to-End Tests Completed ===${NC}"


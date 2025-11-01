# FrodoPIR Integration Design

## Overview

This document outlines the integration strategy for FrodoPIR with silhouette-db. The challenge is bridging FrodoPIR's index-based queries with our key-value system.

## Architecture Challenge

**FrodoPIR:**
- Queries by row index (0, 1, 2, ...)
- Database is array of base64-encoded strings
- Each query targets a specific index

**silhouette-db:**
- Stores key-value pairs
- Uses OKVS for oblivious storage
- Queries by key (string), not index

## Solution: Two-Phase Storage

### Phase 1: Raw Key-Value Storage (for PIR)
Before OKVS encoding, we maintain a sequential array of key-value pairs:
- Index 0: (key1, value1)
- Index 1: (key2, value2)
- ...

This array serves as the FrodoPIR database.

### Phase 2: OKVS Encoding (for Oblivious Storage)
After PIR setup, encode into OKVS blob for compact storage.

## Integration Flow

### Server Side (Setup):
1. Collect all key-value pairs from workers
2. Create mapping: `key -> index`
3. Convert pairs to base64-encoded strings array
4. Initialize FrodoPIR Shard from this array
5. Extract BaseParams (serialized) for clients
6. Store:
   - FrodoPIR Shard (in memory for query processing)
   - BaseParams (serialized) for client distribution
   - OKVS blob (for compact storage)
   - Key-to-index mapping (for client queries)

### Client Side (Query):
1. Client downloads BaseParams from server
2. Client wants key "foo" - needs to know its index
3. Client requests index from server (or downloads mapping)
4. Client generates PIR query for that index
5. Server processes query using Shard
6. Client decodes response

## Implementation Steps

### Step 1: Modify PublishValues Handler
- Store raw pairs array (for FrodoPIR)
- Create key-to-index mapping
- Initialize FrodoPIR Shard

### Step 2: Server-Side PIR
- Store Shard per round
- Implement ProcessQuery using FrodoPIR Shard

### Step 3: Client-Side PIR
- Client downloads BaseParams when connecting
- Client maintains mapping or queries server for index
- Generate queries based on index

### Step 4: Alternative: Index Discovery
Instead of mapping, client could:
- Query server: "What is the index of key 'foo'?"
- Server returns index (privacy-preserving via PIR if needed)
- Client uses index for PIR query

## FFI Wrapper Design

The Rust FFI wrapper provides:
1. `frodopir_shard_create` - Create server database from pairs
2. `frodopir_shard_respond` - Process PIR query
3. `frodopir_client_create` - Initialize client from BaseParams
4. `frodopir_client_generate_query` - Generate query for index
5. `frodopir_client_decode_response` - Decode server response

## Memory Management

- Server Shard: Kept in memory per round
- Client BaseParams: Cached per client
- Queries/Responses: Allocated/deallocated via FFI

## Next Steps

1. Complete Rust FFI wrapper
2. Build static library
3. Integrate with Go via cgo
4. Modify server to maintain pairs array + Shard
5. Implement client index resolution
6. Test end-to-end flow


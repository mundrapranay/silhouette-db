# K-Core Decomposition Algorithm - Adaptation Plan

This document outlines how to adapt the existing k-core decomposition algorithm from channel-based coordination to silhouette-db framework with **OKVS storage** and **shared LDS in OKVS**.

## Requirements

1. **Synchronous algorithm** with rounds decided in initial exchange
2. **All send data messages stored in OKVS** (not local channels)
3. **LDS maintained in OKVS** (shared across all workers via queries)
4. **Number of rounds varies** based on max degree threshold (agreed upon initially)

## Architecture Comparison

### Current Architecture (Channel-Based)

```
┌─────────────────────────────────────────┐
│  KCoreCoordinator (Centralized)        │
│  - Shared LDS in-memory                 │
│  - Worker channels (in-memory)          │
│  - Direct updates to LDS                │
└──────────┬──────────────────────────────┘
           │ Channels (Go channels)
           ▼
┌─────────────────────────────────────────┐
│  Workers (Goroutines, same process)    │
│  - Send data via channels               │
│  - Query LDS from coordinator           │
└─────────────────────────────────────────┘
```

### New Architecture (silhouette-db + OKVS)

```
┌─────────────────────────────────────────┐
│  silhouette-db Server                  │
│  - OKVS-encoded LDS levels              │
│  - OKVS-encoded level increases        │
│  - PIR queries for oblivious access     │
│  - Round synchronization               │
└──────────┬──────────────────────────────┘
           │ gRPC + PIR
           ▼
┌─────────────────────────────────────────┐
│  K-Core Workers (Distributed)           │
│  - Query LDS levels via PIR            │
│  - Publish level increases → OKVS       │
│  - No local LDS copy                    │
│  - All state in OKVS                    │
└─────────────────────────────────────────┘
```

## Key Design Decisions

### 1. LDS Storage in OKVS

**Storage Format:**
- Key: `level-{vertexID}`
- Value: `float64(level)` (8 bytes, little-endian)
- Stored in OKVS for each round that updates levels

**Access Pattern:**
- Workers query levels via PIR (oblivious)
- Updates published and aggregated in OKVS
- No local LDS copies - OKVS is source of truth

### 2. Round Structure

**Initial Exchange (Round 0):**
- All workers publish their vertex degrees (with noise)
- Server aggregates: `degree-{vertexID}` → noised_degree
- Server computes max degree threshold
- Server computes number of rounds based on max threshold
- All workers agree on round count

**Algorithm Rounds (Round 1 to Round N):**
- Round r has two phases:
  - **Phase 1 (Publish)**: Publish level increases → OKVS
  - **Phase 2 (Query + Update)**: Query neighbor levels → compute next → publish updates → OKVS

### 3. Data Flow per Round

```
Round r:
  1. Query neighbor levels from round r-1 (via PIR)
     - Query key: "level-{neighborID}"
     - Returns: level value from OKVS
     
  2. Compute level increases locally
     - Based on neighbor levels and threshold
     
  3. Publish level increases → OKVS
     - Key: "level-increase-{vertexID}-round-{r}"
     - Value: 1.0 if increase, 0.0 if not
     
  4. Wait for round completion (synchronization)
     
  5. Query aggregated level increases from round r
     - Key: "level-increase-{vertexID}-round-{r}"
     - Returns: aggregated value (max of all workers)
     
  6. Compute new level = old level + increase
     
  7. Publish updated level → OKVS
     - Key: "level-{vertexID}"
     - Value: new level value
     - This becomes source for round r+1
```

### 4. Initial Exchange (Round 0)

**Purpose**: 
- Agree on number of rounds
- Initialize LDS with degrees
- Compute max round threshold

**Flow:**
1. Workers publish noised degrees: `degree-{vertexID}` → noised_degree
2. Server aggregates all degrees in OKVS
3. Server computes `maxPublicRoundThreshold` from max degree
4. Server computes `number_of_rounds = min(number_of_rounds-2, maxPublicRoundThreshold)`
5. Server stores round count: `rounds-total` → number_of_rounds
6. All workers query `rounds-total` to agree on round count

**Storage in OKVS:**
- Initial degrees: `degree-{vertexID}` → noised_degree (float64)
- Round count: `rounds-total` → number_of_rounds (float64)
- Initial levels: `level-{vertexID}` → 0.0 (all start at level 0)

## Implementation Structure

### Algorithm Structure

```go
type KCoreDecomposition struct {
    graphData *common.GraphData
    config    map[string]interface{}
    
    // Worker info
    workerID     string
    numWorkers   int
    myVertices   []int
    vertexAssign map[int]string
    
    // Algorithm parameters (from initial exchange)
    n                      int
    psi                    float64
    epsilon               float64
    lambda                float64
    levelsPerGroup        float64
    number_of_rounds      int        // Agreed upon in round 0
    maxPublicRoundThreshold int      // From initial exchange
    
    // Vertex state (local computation only)
    vertices map[int]*KCoreVertex
    
    // Results
    resultFile string
    mu         sync.Mutex
}
```

### Round 0: Initial Exchange

```go
func (a *KCoreDecomposition) executeRound0(ctx context.Context, dbClient *client.Client) error {
    roundID := uint64(0)
    
    // 1. Start round 0
    err := dbClient.StartRound(ctx, roundID, int32(a.numWorkers))
    
    // 2. Compute noised degrees for assigned vertices
    degreePairs := make(map[string][]byte)
    maxWorkerThreshold := 0
    
    for _, vertexID := range a.myVertices {
        vertex := a.vertices[vertexID]
        
        // Compute noised degree (same logic as original)
        degree := len(vertex.neighbours)
        noised_degree := int64(degree)
        
        if a.config["noise"].(bool) {
            geomDist := noise.NewGeomDistribution(a.lambda / 2.0)
            noise_sampled := geomDist.TwoSidedGeometric()
            noised_degree += noise_sampled
            // ... bias adjustment ...
        }
        
        // Compute round threshold
        threshold := math.Ceil(log_a_to_base_b(int(noised_degree), 2)) * a.levelsPerGroup
        vertex.round_threshold = int(threshold) + 1
        
        // Store degree in OKVS
        degreeKey := fmt.Sprintf("degree-%d", vertexID)
        degreeValue := float64ToBytes(float64(noised_degree))
        degreePairs[degreeKey] = degreeValue
        
        // Track max threshold
        if vertex.round_threshold > maxWorkerThreshold {
            maxWorkerThreshold = vertex.round_threshold
        }
    }
    
    // 3. Publish degrees → OKVS
    err = dbClient.PublishValues(ctx, roundID, a.workerID, degreePairs)
    
    // 4. Wait for round completion
    // (Poll PIR client initialization)
    
    // 5. Query max threshold from other workers
    // Server will aggregate and compute maxPublicRoundThreshold
    // Store it as: "max-threshold" → maxPublicRoundThreshold
    
    // 6. Query max threshold and round count
    maxThresholdBytes, err := dbClient.GetValue(ctx, roundID, "max-threshold")
    maxPublicRoundThreshold := int(bytesToFloat64(maxThresholdBytes))
    
    roundsBytes, err := dbClient.GetValue(ctx, roundID, "rounds-total")
    a.number_of_rounds = int(bytesToFloat64(roundsBytes))
    a.maxPublicRoundThreshold = maxPublicRoundThreshold
    
    // 7. Initialize levels in OKVS (all start at 0)
    // This happens in round 0 or round 1
    
    return nil
}
```

### Round r (Algorithm Rounds)

```go
func (a *KCoreDecomposition) executeRound(ctx context.Context, dbClient *client.Client, round int) error {
    roundID := uint64(round + 1) // Round 0 was initial, Round 1 is first algo round
    
    // Phase 1: Publish Level Increases
    err := dbClient.StartRound(ctx, roundID, int32(a.numWorkers))
    
    // Query neighbor levels from previous round (via PIR)
    neighborLevels := make(map[int]uint)
    for _, vertexID := range a.myVertices {
        vertex := a.vertices[vertexID]
        
        // Check if this vertex's threshold is reached
        if vertex.round_threshold == round {
            vertex.permanent_zero = 0
        }
        
        // Query current level from OKVS (round r-1)
        if round > 1 {
            levelKey := fmt.Sprintf("level-%d", vertexID)
            levelBytes, err := dbClient.GetValue(ctx, uint64(round), levelKey)
            if err == nil {
                currentLevel := uint(bytesToFloat64(levelBytes))
                vertex.current_level = currentLevel
            }
        } else {
            vertex.current_level = 0 // Round 1 starts at level 0
        }
        
        // Query neighbor levels via PIR
        if vertex.current_level == round && vertex.permanent_zero != 0 {
            for _, neighborID := range vertex.neighbours {
                neighborKey := fmt.Sprintf("level-%d", neighborID)
                neighborLevelBytes, err := dbClient.GetValue(ctx, uint64(round), neighborKey)
                if err != nil {
                    neighborLevels[neighborID] = 0 // Default to 0 if not found
                } else {
                    neighborLevels[neighborID] = uint(bytesToFloat64(neighborLevelBytes))
                }
            }
        }
    }
    
    // Compute level increases
    levelIncreases := make(map[string][]byte)
    for _, vertexID := range a.myVertices {
        vertex := a.vertices[vertexID]
        
        if vertex.current_level == round && vertex.permanent_zero != 0 {
            // Count neighbors at same level
            neighbor_count := 0
            for _, neighborID := range vertex.neighbours {
                if neighborLevels[neighborID] == uint(round) {
                    neighbor_count++
                }
            }
            
            // Apply noise
            noised_neighbor_count := int64(neighbor_count)
            if a.config["noise"].(bool) {
                scale := a.lambda / (2.0 * float64(vertex.round_threshold))
                geomDist := noise.NewGeomDistribution(scale)
                noise_sampled := geomDist.TwoSidedGeometric()
                extra_bias := int64(3 * (2 * math.Exp(scale)) / math.Pow((math.Exp(2*scale)-1), 3))
                noised_neighbor_count += noise_sampled
                noised_neighbor_count += extra_bias
            }
            
            // Compute group index
            group_index := float64(a.localLDS.GroupForLevel(uint(round)))
            
            // Decide if level should increase
            threshold := math.Pow(1.0+a.psi, group_index)
            if noised_neighbor_count > int64(threshold) {
                vertex.next_level = 1
                increaseKey := fmt.Sprintf("level-increase-%d-round-%d", vertexID, round+1)
                levelIncreases[increaseKey] = float64ToBytes(1.0)
            } else {
                vertex.permanent_zero = 0
                increaseKey := fmt.Sprintf("level-increase-%d-round-%d", vertexID, round+1)
                levelIncreases[increaseKey] = float64ToBytes(0.0)
            }
        }
    }
    
    // Publish level increases → OKVS
    err = dbClient.PublishValues(ctx, roundID, a.workerID, levelIncreases)
    
    // Wait for round completion
    // (Poll PIR client initialization)
    
    // Phase 2: Query Aggregated Increases and Update Levels
    
    // Query aggregated level increases
    levelUpdates := make(map[string][]byte)
    for _, vertexID := range a.myVertices {
        vertex := a.vertices[vertexID]
        
        increaseKey := fmt.Sprintf("level-increase-%d-round-%d", vertexID, round+1)
        increaseBytes, err := dbClient.GetValue(ctx, roundID, increaseKey)
        
        if err == nil {
            increase := bytesToFloat64(increaseBytes)
            
            // Compute new level
            newLevel := vertex.current_level
            if increase == 1.0 {
                newLevel++
            }
            
            // Store updated level → OKVS (for next round)
            levelKey := fmt.Sprintf("level-%d", vertexID)
            levelUpdates[levelKey] = float64ToBytes(float64(newLevel))
        }
    }
    
    // Publish updated levels → OKVS
    // Use same round or next round? Probably next round for clarity
    // Actually, we can publish in a synchronization round after increases
    
    return nil
}
```

## Two-Phase Round Structure

For clarity, each algorithm round becomes two rounds:

### Round 2r-1: Publish Level Increases
- Workers compute level increases
- Publish: `level-increase-{vertexID}-round-{r}` → 1.0 or 0.0
- Stored in OKVS

### Round 2r: Publish Updated Levels
- Workers query aggregated increases
- Compute new levels
- Publish: `level-{vertexID}` → new_level
- Stored in OKVS (source for round 2r+1)

## Initial Exchange Details

### Round 0: Degree Publication and Round Agreement

**Worker Actions:**
1. Compute noised degrees for assigned vertices
2. Publish: `degree-{vertexID}` → noised_degree
3. Wait for round completion
4. Query: `max-threshold` → maxPublicRoundThreshold
5. Query: `rounds-total` → number_of_rounds
6. Store agreed-upon round count locally

**Server Actions:**
1. Aggregate all degrees from workers
2. Compute max degree across all workers
3. Compute maxPublicRoundThreshold from max degree
4. Compute number_of_rounds = min(number_of_rounds-2, maxPublicRoundThreshold)
5. Store in OKVS:
   - `max-threshold` → maxPublicRoundThreshold
   - `rounds-total` → number_of_rounds
   - Initialize: `level-{vertexID}` → 0.0 for all vertices

**Key Format:**
- `degree-{vertexID}` → noised_degree (float64)
- `max-threshold` → maxPublicRoundThreshold (float64)
- `rounds-total` → number_of_rounds (float64)
- `level-{vertexID}` → level (float64, initialized to 0.0)

## Round Execution Pattern

```
Round 0 (Initial Exchange):
  - Publish: degree-{vertexID} → noised_degree
  - Query: max-threshold, rounds-total
  - Result: Agreed upon number_of_rounds

Round 1 (First Algorithm Round):
  Phase 1 (Round 1):
    - Query: level-{neighborID} (all 0.0 initially)
    - Compute: level increases
    - Publish: level-increase-{vertexID}-round-1 → 1.0 or 0.0
  
  Phase 2 (Round 2):
    - Query: level-increase-{vertexID}-round-1 (aggregated)
    - Compute: new_level = old_level + increase
    - Publish: level-{vertexID} → new_level

Round 2 (Second Algorithm Round):
  Phase 1 (Round 3):
    - Query: level-{neighborID} (from Round 2)
    - Compute: level increases
    - Publish: level-increase-{vertexID}-round-2 → 1.0 or 0.0
  
  Phase 2 (Round 4):
    - Query: level-increase-{vertexID}-round-2
    - Compute: new_level
    - Publish: level-{vertexID} → new_level

... continue until number_of_rounds
```

## LDS in OKVS

**Key-Value Storage:**
- `level-{vertexID}` → current level (float64)
- Updated each algorithm round
- Queried via PIR for oblivious access

**Benefits:**
- No local LDS copies
- Consistent across all workers
- Oblivious queries (PIR)
- OKVS hides which levels are stored

## Helper Functions

```go
// Convert float64 to 8-byte little-endian
func float64ToBytes(f float64) []byte {
    bytes := make([]byte, 8)
    binary.LittleEndian.PutUint64(bytes, math.Float64bits(f))
    return bytes
}

// Convert 8-byte little-endian to float64
func bytesToFloat64(bytes []byte) float64 {
    bits := binary.LittleEndian.Uint64(bytes[:8])
    return math.Float64frombits(bits)
}

// Convert uint to float64 for storage
func uintToBytes(u uint) []byte {
    return float64ToBytes(float64(u))
}

// Log base conversion
func log_a_to_base_b(a int, b float64) float64 {
    return math.Log2(float64(a)) / math.Log2(b)
}
```

## Complete Implementation Checklist

- [ ] Implement `GraphAlgorithm` interface
- [ ] Round 0: Initial exchange
  - [ ] Publish noised degrees
  - [ ] Query max threshold
  - [ ] Query round count
  - [ ] Initialize levels in OKVS
- [ ] Round execution loop
  - [ ] Phase 1: Query neighbor levels, compute increases, publish
  - [ ] Phase 2: Query aggregated increases, compute new levels, publish
- [ ] LDS operations via OKVS
  - [ ] Query levels via PIR
  - [ ] Update levels via publish
- [ ] Noise addition (geometric distribution)
- [ ] Round threshold logic
- [ ] Result computation and file writing
- [ ] Register algorithm in registry
- [ ] Create configuration file template

## Key Differences from Original

1. **No Local LDS**: All LDS operations via OKVS queries
2. **No Coordinator**: Server aggregates automatically
3. **No Channels**: Round-based publish/query pattern
4. **Initial Exchange**: Rounds determined upfront
5. **OKVS Storage**: All data in OKVS, not in-memory channels
6. **Distributed**: Workers can be on different machines

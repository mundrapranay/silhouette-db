# Issue: Fix K-Core Decomposition Level Update Logic

## Problem

The k-core decomposition algorithm is not correctly updating vertex levels, resulting in all vertices having a core number of 2.5 (which indicates level 0 for all vertices after all rounds).

## Symptoms

- All vertices in the result files have core number `2.5000`
- Levels remain at 0 throughout all algorithm rounds
- The algorithm completes successfully but doesn't converge to meaningful core numbers

## Root Cause

The level update logic in `algorithms/ledp/kcore_decomposition.go` had several issues:

1. **Wrong Round IDs for Level Queries**: Levels were being queried from the wrong rounds
   - Levels are published in "update rounds" (rounds 2, 4, 6, ...)
   - Increases are published in "increase rounds" (rounds 1, 3, 5, ...)
   - The code was mixing these up

2. **Level Not Queried Before Update**: In `executeRoundUpdateLevels()`, the code used `vertex.current_level` (which was set in the previous phase) instead of querying the actual current level from OKVS

3. **Neighbor Levels from Wrong Round**: Neighbor levels were queried from the wrong round ID

4. **Missing Level Increases**: Inactive vertices weren't publishing level increases, so the update phase didn't know they existed

## Solution

### Changes Made

1. **Fixed Level Querying in `executeRoundPublishIncreases()`**:
   - Now queries levels from previous update round: `roundID = 2 * algorithmRound`
   - For algorithm round 0, levels default to 0 (no previous round)

2. **Fixed Level Update Logic in `executeRoundUpdateLevels()`**:
   - First queries current level from OKVS (from previous update round)
   - Then queries level increase from previous increase round
   - Computes new level correctly based on both values

3. **Fixed Neighbor Level Querying**:
   - Now queries neighbor levels from previous update round (where levels were last published)
   - Uses correct round ID: `prevLevelRoundID = 2 * algorithmRound`

4. **Ensured All Vertices Publish Increases**:
   - Even inactive vertices now publish `0.0` increases
   - This ensures the update phase knows all vertices exist

### Round Structure

The algorithm uses a two-phase round structure:
- **Round 2r+1**: Publish level increases (e.g., rounds 1, 3, 5, ...)
- **Round 2r+2**: Update levels (e.g., rounds 2, 4, 6, ...)

When in algorithm round `r`:
- Query levels from round `2r` (previous update round)
- Query increases from round `2r+1` (previous increase round)
- Publish updated levels to round `2r+2` (current update round)

## Testing

After the fix, levels should:
- Start at 0 for all vertices
- Gradually increase based on neighbor counts and thresholds
- Result in varied core numbers (not all 2.5)

Run the test script:
```bash
make test-kcore-decomposition
```

Verify that:
- Result files show varied core numbers
- Levels increase over rounds
- Core numbers are within expected ranges

## Related Files

- `algorithms/ledp/kcore_decomposition.go` - Main algorithm implementation
- `scripts/test-kcore-decomposition.sh` - Test script
- `configs/kcore_decomposition.yaml` - Algorithm configuration

## Status

- [x] Identified root cause
- [x] Implemented fixes
- [x] Committed changes
- [ ] Verified fix with test run
- [ ] Update documentation if needed

## Next Steps

1. Run the k-core decomposition test to verify the fix
2. Check result files for varied core numbers
3. Verify levels are updating correctly across rounds
4. If issues persist, add debug logging to trace level updates


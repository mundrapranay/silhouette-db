# Running Benchmarks

This document explains how to run performance benchmarks for `silhouette-db`.

## Quick Start

### Run All Benchmarks

```bash
# Using Makefile
make bench

# Or directly with go test
go test ./... -bench=. -benchmem -run=^$
```

### Run Benchmarks for Specific Packages

```bash
# Store (FSM) benchmarks only
make bench-store

# Server (gRPC API) benchmarks only
make bench-server

# Or directly
go test ./internal/store/... -bench=. -benchmem -run=^$
go test ./internal/server/... -bench=. -benchmem -run=^$
```

## Available Benchmarks

### FSM Benchmarks (`internal/store/benchmark_test.go`)

- `BenchmarkFSM_Apply_SET` - Measures SET operation performance
- `BenchmarkFSM_Get` - Measures read performance
- `BenchmarkFSM_Snapshot` - Measures snapshot creation performance
- `BenchmarkFSM_MultipleSets` - Measures multiple SET operations
- `BenchmarkFSM_ConcurrentReads` - Measures concurrent read performance

### Server Benchmarks (`internal/server/benchmark_test.go`)

- `BenchmarkStartRound` - Measures StartRound RPC performance
- `BenchmarkPublishValues` - Measures PublishValues RPC performance
- `BenchmarkPublishValues_ManyPairs` - Measures PublishValues with 100 pairs
- `BenchmarkGetValue` - Measures GetValue RPC performance

## Benchmark Flags Explained

- `-bench=.` - Run all benchmarks (can specify pattern like `-bench=BenchmarkFSM`)
- `-benchmem` - Show memory allocation statistics
- `-run=^$` - Skip regular tests (only run benchmarks)
- `-benchtime=5s` - Run each benchmark for 5 seconds (default is 1s)
- `-count=3` - Run each benchmark 3 times for average

## Example Output

```bash
$ go test ./internal/store/... -bench=. -benchmem -run=^$
goos: darwin
goarch: arm64
pkg: github.com/mundrapranay/silhouette-db/internal/store
BenchmarkFSM_Apply_SET-8             1000000    1200 ns/op    512 B/op    5 allocs/op
BenchmarkFSM_Get-8                    5000000     245 ns/op      0 B/op    0 allocs/op
BenchmarkFSM_Snapshot-8                 10000  125000 ns/op  51200 B/op  1024 allocs/op
...
```

## Understanding Output

Let's break down what each value means using an example:

```
BenchmarkFSM_Get-11    150331503    8.011 ns/op    0 B/op    0 allocs/op
│                      │            │              │         │
│                      │            │              │         └─ Number of allocations per operation
│                      │            │              └─────────── Bytes allocated per operation
│                      │            └────────────────────────── Nanoseconds per operation
│                      └────────────────────────────────────── Number of iterations run
└─────────────────────────────────────────────────────────────── Benchmark name
```

### Detailed Explanation

1. **Benchmark Name** (`BenchmarkFSM_Get-11`)
   - Format: `BenchmarkFunctionName-CPUCores`
   - `-11` means it ran on 11 CPU cores

2. **Number of Iterations** (`150331503`)
   - How many times the operation was executed
   - Go automatically determines this to get stable timing (usually runs for ~1 second by default)
   - Higher number = faster operation (ran more times in same time)

3. **ns/op** (`8.011 ns/op`)
   - **Nanoseconds per operation** (1 nanosecond = 0.000000001 seconds)
   - Average time to execute one operation
   - **Lower is better** - means the operation is faster
   - Example: 8.011 ns = 8.011 billionths of a second (extremely fast!)

4. **B/op** (`0 B/op`)
   - **Bytes allocated per operation**
   - How much memory is allocated each time the operation runs
   - **Lower is better** - means less memory usage
   - `0 B/op` means no new allocations (very efficient!)
   - If you see `320 B/op`, that means 320 bytes allocated per operation

5. **allocs/op** (`0 allocs/op`)
   - **Number of memory allocations per operation**
   - Each allocation has overhead, so fewer is better
   - **Lower is better** - means less memory allocation overhead
   - `0 allocs/op` means no allocations (reuses existing memory)
   - If you see `8 allocs/op`, that means 8 allocations happen per operation

### Example Analysis

Looking at your output:
```
BenchmarkFSM_Get-11         150331503        8.011 ns/op    0 B/op    0 allocs/op
```

This means:
- ✅ **Very fast**: 8 nanoseconds per read (150+ million operations/second!)
- ✅ **No memory allocation**: 0 bytes and 0 allocations (reuses memory)
- ✅ **Highly efficient**: Perfect benchmark result

Compare to:
```
BenchmarkFSM_Snapshot-11       20929    57583 ns/op    198996 B/op    1023 allocs/op
```

This means:
- ⚠️ **Slower**: ~58 microseconds per snapshot (still fast, but much slower than Get)
- ⚠️ **Memory intensive**: ~199KB and 1023 allocations per snapshot
- 📊 **Expected**: Snapshots need to copy all data, so this is normal

## Tips

1. **Compare Results**: Run benchmarks multiple times and compare averages
   ```bash
   go test ./internal/store/... -bench=BenchmarkFSM_Get -benchmem -count=5
   ```

2. **Focus on Specific Benchmark**:
   ```bash
   go test ./internal/store/... -bench=BenchmarkFSM_Get -benchmem
   ```

3. **Longer Runs for Stability**:
   ```bash
   go test ./internal/store/... -bench=. -benchtime=10s -benchmem
   ```

4. **Compare Before/After Changes**:
   ```bash
   # Before changes
   go test ./internal/store/... -bench=BenchmarkFSM_Get -benchmem > before.txt
   
   # After changes
   go test ./internal/store/... -bench=BenchmarkFSM_Get -benchmem > after.txt
   
   # Compare (using benchcmp if installed)
   benchcmp before.txt after.txt
   ```

## Installation of benchcmp (Optional)

```bash
go install golang.org/x/tools/cmd/benchcmp@latest
```

Then compare benchmark results:
```bash
benchcmp before.txt after.txt
```


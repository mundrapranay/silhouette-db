# Research Paper Writing Guide for silhouette-db

This guide outlines the structure and topics for writing a research paper about the `silhouette-db` framework. It provides section-by-section guidance on what to discuss and key points to emphasize.

## Paper Structure Overview

```
1. Abstract
2. Introduction
3. Related Work
4. Background and Preliminaries
5. System Design and Architecture
6. Implementation Details
7. Evaluation and Experimental Results
8. Discussion
9. Limitations and Future Work
10. Conclusion
11. References
```

---

## 1. Abstract (150-250 words)

### Key Points to Cover

- **Problem Statement**: Centralized coordinators are bottlenecks and single points of failure for LEDP algorithms
- **Solution**: Distributed, oblivious coordination layer using Raft consensus
- **Key Contributions**: 
  - Integration of OKVS and PIR for complete obliviousness
  - Round-based synchronous coordination framework
  - Fault-tolerant distributed architecture
- **Results Summary**: (to be filled after evaluation)
  - Performance metrics
  - Scalability results
  - Privacy guarantees

### Template Outline

```
We present silhouette-db, a fault-tolerant, distributed, and oblivious 
coordination layer for testing Local Edge Differentially Private (LEDP) 
algorithms. Unlike existing centralized coordinator models, silhouette-db 
provides a distributed peer-to-peer architecture built upon Raft consensus 
and advanced cryptographic primitives. The system combines Oblivious Key-Value 
Store (OKVS) encoding with Private Information Retrieval (PIR) to achieve 
complete obliviousness—hiding both storage access patterns and query patterns. 
Our evaluation demonstrates [X] performance with [Y] scalability across 
[Z] nodes, maintaining sub-second query latencies while preserving privacy 
guarantees.
```

---

## 2. Introduction

### Subsection 2.1: Motivation and Context

**Topics to Discuss**:

- **LEDP Algorithms**: Explain Local Edge Differential Privacy and its applications
- **Centralized Coordinator Limitations**:
  - Single point of failure
  - Scalability bottlenecks
  - Privacy concerns with centralized data aggregation
  - Lack of fault tolerance
- **Need for Distributed Coordination**: Why distributed systems are better suited for LEDP

### Subsection 2.2: Challenges

**Topics to Discuss**:

- **Privacy-Preserving Storage**: How to hide which keys are stored
- **Private Queries**: How to enable queries without revealing which key is requested
- **Consistency in Distributed Systems**: Ensuring all nodes agree on data
- **Fault Tolerance**: Handling node failures gracefully
- **Round-Based Synchronization**: Coordinating synchronous algorithm rounds across distributed workers

### Subsection 2.3: Contributions

**Topics to Discuss**:

1. **Distributed Oblivious Coordination Framework**:
   - First distributed coordination layer for LEDP algorithms
   - Raft consensus for fault tolerance
   - Round-based synchronous coordination

2. **Cryptographic Integration**:
   - Combined OKVS + PIR for complete obliviousness
   - Practical performance (sub-second queries)
   - Dynamic OKVS encoding based on data size

3. **Graph Algorithm Framework**:
   - Generic framework for round-based algorithms
   - Support for both exact and LEDP algorithms
   - Automatic vertex assignment and graph partitioning

4. **Production-Ready Implementation**:
   - Comprehensive testing (unit, integration, load tests)
   - Multi-node cluster support
   - Practical deployment considerations

### Subsection 2.4: Paper Organization

Brief overview of paper structure.

---

## 3. Related Work

### Subsection 3.1: Privacy-Preserving Database Systems

**Topics to Discuss**:

- **Oblivious RAM (ORAM)**: Alternatives and why OKVS is chosen
- **Searchable Encryption**: Comparison with PIR approaches
- **Oblivious Database Systems**: Related work in oblivious data structures

### Subsection 3.2: Private Information Retrieval

**Topics to Discuss**:

- **Single-Server PIR**: State-of-the-art schemes (FrodoPIR, SealPIR, etc.)
- **Multi-Server PIR**: Trade-offs and why single-server is chosen
- **PIR Performance**: Recent advances in practical PIR

### Subsection 3.3: Distributed Consensus

**Topics to Discuss**:

- **Raft Consensus**: Why Raft over Paxos, PBFT
- **Byzantine Fault Tolerance**: When needed vs. crash fault tolerance
- **Consensus in Privacy-Preserving Systems**: Related work

### Subsection 3.4: LEDP Algorithm Coordination

**Topics to Discuss**:

- **Existing LEDP Frameworks**: Centralized approaches
- **Distributed Graph Algorithms**: Related work in distributed graph processing
- **Gap in Literature**: Why distributed oblivious coordination is novel

### Positioning Statement

```
Unlike existing work that relies on centralized coordinators or focuses on 
single primitives, silhouette-db is the first system to combine distributed 
consensus, oblivious storage, and private queries into a unified framework 
specifically designed for LEDP algorithm execution.
```

---

## 4. Background and Preliminaries

### Subsection 4.1: Local Edge Differential Privacy (LEDP)

**Topics to Discuss**:

- Definition and privacy guarantees
- Applications in graph algorithms
- Round-based synchronous model
- Privacy vs. utility trade-offs

### Subsection 4.2: Oblivious Key-Value Store (OKVS)

**Topics to Discuss**:

- **Definition**: What OKVS provides
- **RB-OKVS Algorithm**: Random Band Matrix approach
- **Properties**:
  - Obliviousness (hides which keys are stored)
  - Compactness (~10-20% overhead)
  - Decodability (any key can be decoded)
- **Limitations**: Minimum pair requirements

### Subsection 4.3: Private Information Retrieval (PIR)

**Topics to Discuss**:

- **Definition**: Query privacy guarantees
- **FrodoPIR**: LWE-based single-server PIR
- **Security Model**: What the server learns vs. doesn't learn
- **Performance Characteristics**: Query latency, database size limits

### Subsection 4.4: Raft Consensus

**Topics to Discuss**:

- **Consensus Problem**: Distributed agreement
- **Raft Algorithm**: Leader election, log replication, safety guarantees
- **Crash Fault Tolerance**: Assumptions and guarantees
- **Why Raft**: Simplicity, understandability, production readiness

---

## 5. System Design and Architecture

### Subsection 5.1: High-Level Architecture

**Topics to Discuss**:

- **Three-Layer Design**:
  1. Client Layer (LEDP Workers)
  2. Coordination Layer (silhouette-db servers)
  3. Storage Layer (Raft cluster)

- **Component Interaction**:
  - Round-based synchronous coordination
  - Publish phase (workers → server)
  - Query phase (workers ← server)

### Subsection 5.2: Round-Based Coordination Model

**Topics to Discuss**:

- **Round Lifecycle**:
  1. Round initialization (`StartRound`)
  2. Worker publishing (`PublishValues`)
  3. Aggregation and encoding
  4. Query phase (`GetValue`)

- **Synchronization Guarantees**: How round completion is ensured
- **Empty Rounds**: Handling synchronization-only rounds

### Subsection 5.3: Oblivious Storage Design

**Topics to Discuss**:

- **OKVS Encoding Decision**:
  - When to use OKVS (≥100 pairs)
  - Direct PIR fallback (<100 pairs)
  - Trade-offs and rationale

- **Key-to-Index Mapping**: 
  - Why needed (PIR uses indices, not keys)
  - Security considerations
  - Distribution mechanism

### Subsection 5.4: Private Query Design

**Topics to Discuss**:

- **PIR Query Flow**:
  1. Client initialization (BaseParams, key mapping)
  2. Query generation (key → index → PIR query)
  3. Server processing (oblivious response)
  4. Client decoding

- **Per-Round PIR Clients**: Why separate clients per round
- **Error Handling**: OverflownAdd retry logic

### Subsection 5.5: Distributed Consensus Integration

**Topics to Discuss**:

- **Raft FSM**: State machine for storing OKVS blobs
- **Replication**: How data is replicated across nodes
- **Leader Election**: Fault tolerance mechanisms
- **Consistency Guarantees**: Strong consistency via Raft

### Subsection 5.6: Graph Algorithm Framework

**Topics to Discuss**:

- **Algorithm Interface**: Generic `GraphAlgorithm` interface
- **Vertex Assignment**: Deterministic assignment for distributed graphs
- **Local Testing vs. Deployment**: Graph loading strategies
- **Round Execution**: How algorithms coordinate through silhouette-db

---

## 6. Implementation Details

### Subsection 6.1: Technology Stack

**Topics to Discuss**:

- **Language Choices**: Why Go (concurrency, gRPC, ecosystem)
- **Cryptographic Libraries**: 
  - FrodoPIR (Rust FFI via cgo)
  - RB-OKVS (Rust FFI via cgo)
- **Consensus**: HashiCorp Raft library
- **RPC Framework**: gRPC for type-safe APIs

### Subsection 6.2: FFI Integration

**Topics to Discuss**:

- **Challenge**: Rust libraries need Go bindings
- **Solution**: C-compatible FFI wrappers
- **Implementation**: 
  - `frodo-pir-ffi`: Rust FFI wrapper for FrodoPIR
  - `rb-okvs-ffi`: Rust FFI wrapper for RB-OKVS
  - cgo bindings in Go

### Subsection 6.3: Key Implementation Details

**Topics to Discuss**:

- **Memory Management**: C memory allocation/deallocation
- **Thread Safety**: Per-round PIR clients, concurrent round handling
- **Error Handling**: Retry logic, graceful degradation
- **Performance Optimizations**: 
  - Key-to-index caching
  - BaseParams caching
  - Deterministic sorting for consistency

### Subsection 6.4: Testing Infrastructure

**Topics to Discuss**:

- **Test Coverage**: Unit, integration, end-to-end tests
- **Load Testing**: Performance under sustained load
- **Multi-Node Testing**: Cluster formation and fault tolerance
- **Algorithm Testing**: Degree-collector example

---

## 7. Evaluation and Experimental Results

### Subsection 7.1: Experimental Setup

**Topics to Discuss**:

- **Hardware Configuration**: CPU, memory, network
- **Deployment Setup**: Number of nodes, cluster configuration
- **Workloads**: Graph sizes, number of workers, rounds
- **Baselines**: Comparison with centralized coordinator (if applicable)

### Subsection 7.2: Performance Evaluation

**Metrics to Report**:

1. **Round Completion Time**:
   - Time to aggregate worker contributions
   - OKVS encoding time
   - PIR server creation time
   - Total round completion latency

2. **Query Performance**:
   - PIR query latency (p50, p95, p99)
   - Throughput (queries per second)
   - Impact of database size
   - Impact of number of concurrent queries

3. **Scalability**:
   - Performance with varying number of workers
   - Performance with varying number of nodes
   - Performance with varying database sizes

4. **Encoding Overhead**:
   - OKVS blob size vs. raw data size
   - Encoding/decoding time
   - Memory usage

### Subsection 7.3: Privacy Evaluation

**Topics to Discuss**:

- **Obliviousness Guarantees**: 
  - What the server learns (nothing about keys)
  - What the server learns from queries (nothing about which key)
  
- **Security Analysis**: 
  - Formal privacy guarantees
  - Attack surface analysis
  - Comparison with theoretical bounds

### Subsection 7.4: Fault Tolerance Evaluation

**Topics to Discuss**:

- **Node Failure Scenarios**: 
  - Leader failure recovery time
  - Follower failure handling
  - Network partition scenarios

- **Consistency**: 
  - Data consistency across nodes
  - Round completion guarantees under failures

### Subsection 7.5: Algorithm Case Study

**Topics to Discuss**:

- **Degree-Collector Algorithm**: 
  - Implementation using silhouette-db
  - Performance results
  - Privacy-preserving aspects

- **Comparison with Centralized Approach**:
  - Performance overhead
  - Privacy improvements
  - Fault tolerance benefits

### Subsection 7.6: Overhead Analysis

**Topics to Discuss**:

- **Coordination Overhead**: Cost of distributed consensus
- **Cryptographic Overhead**: OKVS and PIR costs
- **Network Overhead**: Replication, query traffic
- **Trade-offs**: Privacy vs. performance

---

## 8. Discussion

### Subsection 8.1: Design Choices and Trade-offs

**Topics to Discuss**:

- **OKVS Threshold (100 pairs)**: Why chosen, alternatives
- **Raft vs. PBFT**: Crash fault tolerance vs. Byzantine
- **Single-Server PIR**: Why not multi-server
- **Round-Based Model**: Synchronous vs. asynchronous

### Subsection 8.2: Practical Considerations

**Topics to Discuss**:

- **Deployment Complexity**: Setup and maintenance
- **Resource Requirements**: CPU, memory, network
- **Compatibility**: Integration with existing systems
- **Usability**: Developer experience, API design

### Subsection 8.3: Comparison with Alternatives

**Topics to Discuss**:

- **vs. Centralized Coordinators**: Privacy, fault tolerance
- **vs. Plain Distributed Systems**: Privacy-preserving aspects
- **vs. Theoretical Systems**: Practical performance

---

## 9. Limitations and Future Work

### Subsection 9.1: Current Limitations

**Topics to Discuss**:

- **OKVS Minimum Pairs**: 100-pair requirement
- **Database Size Limits**: PIR performance with very large databases
- **Crash Fault Tolerance Only**: Not Byzantine fault tolerant
- **Fixed Value Size**: OKVS requires fixed-size values (8 bytes)
- **Round Synchronization**: Must wait for all workers

### Subsection 9.2: Future Directions

**Topics to Discuss**:

- **Byzantine Fault Tolerance**: Integration with PBFT
- **Larger Databases**: Multi-server PIR for scalability
- **Variable-Size Values**: Support for arbitrary value sizes
- **Asynchronous Rounds**: Relaxing synchronization requirements
- **Additional Algorithms**: More LEDP algorithm implementations
- **Performance Optimization**: Further optimizations for production

---

## 10. Conclusion

### Topics to Discuss

- **Summary**: Recap of problem, solution, contributions
- **Key Results**: Highlight main findings from evaluation
- **Impact**: Significance for LEDP research and practice
- **Future Vision**: Long-term goals and research directions

### Template Outline

```
We presented silhouette-db, the first distributed, fault-tolerant, and 
oblivious coordination layer for LEDP algorithms. By combining Raft consensus, 
OKVS encoding, and PIR queries, silhouette-db achieves complete obliviousness 
while maintaining practical performance. Our evaluation demonstrates [key 
results]. This work opens new directions for privacy-preserving distributed 
systems and enables scalable LEDP algorithm execution with strong privacy 
guarantees.
```

---

## 11. References

### Key Papers to Include

#### Privacy-Preserving Systems
- LEDP algorithm papers
- OKVS foundational papers
- PIR foundational papers
- Oblivious database systems

#### Distributed Systems
- Raft consensus paper
- Distributed graph algorithms
- Fault-tolerant systems

#### Cryptography
- FrodoPIR paper
- RB-OKVS paper
- LWE-based cryptography

---

## Additional Considerations

### Figures to Include

1. **System Architecture Diagram**: High-level component interaction
2. **Round Workflow Diagram**: Step-by-step round execution
3. **OKVS + PIR Integration Flow**: Detailed data flow
4. **Performance Graphs**: 
   - Query latency vs. database size
   - Throughput vs. number of workers
   - Encoding overhead
5. **Comparison Tables**: 
   - vs. centralized coordinators
   - vs. related systems

### Tables to Include

1. **Experimental Setup**: Hardware, software, configuration
2. **Performance Results**: Summary of key metrics
3. **Comparison**: Feature comparison with alternatives
4. **API Summary**: Key gRPC endpoints

### Evaluation Checklist

- [ ] Performance benchmarks completed
- [ ] Scalability tests (varying workers, nodes, database sizes)
- [ ] Fault tolerance tests (node failures, network partitions)
- [ ] Privacy analysis (formal guarantees, attack surface)
- [ ] Comparison with baselines (if applicable)
- [ ] Algorithm case studies (at least one full algorithm)
- [ ] Resource usage profiling (CPU, memory, network)

---

## Writing Tips

### Clarity and Precision

- **Define Terms**: Always define technical terms on first use
- **Use Examples**: Concrete examples help understanding
- **Be Precise**: Distinguish between "privacy", "obliviousness", "differential privacy"
- **Motivate Choices**: Explain why design decisions were made

### Experimental Sections

- **Reproducibility**: Provide enough detail for others to reproduce
- **Honesty**: Report limitations and negative results
- **Statistical Significance**: Use appropriate statistical methods
- **Visualizations**: Use clear graphs and tables

### Related Work

- **Comprehensive**: Cover relevant related work
- **Critical**: Don't just list, compare and contrast
- **Positioning**: Clearly state how your work differs

---

## Timeline Suggestion

1. **Week 1-2**: Write Introduction, Related Work, Background
2. **Week 3-4**: Write System Design and Architecture sections
3. **Week 5-6**: Write Implementation section, complete evaluation
4. **Week 7**: Write Results, Discussion, Limitations
5. **Week 8**: Write Conclusion, polish all sections
6. **Week 9-10**: Review, revise, submission preparation

---

This guide provides a comprehensive structure for writing a research paper about silhouette-db. Adapt sections based on paper requirements (conference/journal), page limits, and specific focus areas.


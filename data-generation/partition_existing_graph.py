#!/usr/bin/env python3
"""
Partition an existing graph file for workers.

This script reads an edge list file and partitions it based on vertex assignment
for the silhouette-db framework. Edges are partitioned so that edge (u,v) goes to
the worker that owns vertex u.

For undirected graphs, edges should be stored as both (u,v) and (v,u) in the
partitioned files.

Optionally updates a config file with vertex assignments.
"""

import argparse
import os
import yaml
from collections import defaultdict
from math import ceil


def calculate_workloads(n, num_workers):
    """
    Calculate vertex assignments for workers (round-robin).
    
    Args:
        n: Total number of vertices
        num_workers: Number of workers
    
    Returns:
        List of lists, where each inner list contains vertex IDs assigned to a worker
    """
    chunk = n // num_workers
    extra = n % num_workers
    offset = 0
    workloads = []
    
    for p in range(1, num_workers + 1):
        workload = chunk + extra if p == num_workers else chunk
        node_ids = list(range(offset, offset + workload))
        workloads.append(node_ids)
        offset += workload
    
    return workloads


def load_graph_from_file(filepath):
    """
    Load graph from edge list file.
    
    Args:
        filepath: Path to edge list file (format: "u v" per line)
    
    Returns:
        Tuple of (adjacency_dict, max_vertex_id, total_edges)
        adjacency_dict: dict from vertex_id -> set of neighbors
    """
    adjacency_dict = defaultdict(set)
    max_vertex = -1
    total_edges = 0
    
    with open(filepath, 'r') as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith('#'):
                continue
            
            parts = line.split()
            if len(parts) < 2:
                continue
            
            try:
                u = int(parts[0])
                v = int(parts[1])
                
                # Track maximum vertex ID
                max_vertex = max(max_vertex, u, v)
                
                # Add edge to adjacency list (both directions for undirected)
                adjacency_dict[u].add(v)
                # Note: We don't add reverse here - we'll handle it during partitioning
                
                total_edges += 1
            except ValueError:
                continue
    
    return adjacency_dict, max_vertex + 1, total_edges


def save_config(config_file, config):
    """Save algorithm configuration to YAML file."""
    with open(config_file, 'w') as f:
        yaml.dump(config, f, default_flow_style=False, sort_keys=False)


def partition_graph(input_file, output_dir, num_workers, graph_size=None, config_file=None):
    """
    Partition graph for workers.
    
    Args:
        input_file: Path to input edge list file
        output_dir: Directory to write partitioned files (will create if needed)
        num_workers: Number of workers
        graph_size: Optional graph size (number of vertices). If None, inferred from graph.
        config_file: Optional path to config file to update with vertex assignments.
    """
    print(f"Loading graph from {input_file}...")
    adjacency_dict, inferred_size, total_edges = load_graph_from_file(input_file)
    
    # Use provided graph_size or inferred size
    if graph_size is None:
        graph_size = inferred_size
        print(f"Inferred graph size: {graph_size} vertices")
    else:
        print(f"Using provided graph size: {graph_size} vertices")
    
    print(f"Total edges in file: {total_edges}")
    print(f"Partitioning for {num_workers} workers...")
    
    # Calculate vertex assignments (round-robin)
    workloads = calculate_workloads(graph_size, num_workers)
    
    # Create output directory if it doesn't exist
    os.makedirs(output_dir, exist_ok=True)
    
    # Partition edges: edge (u,v) goes to worker that owns vertex u
    partitioned_edges = defaultdict(list)
    
    for vertex_id, neighbors in adjacency_dict.items():
        # Find which worker owns this vertex
        worker_idx = vertex_id % num_workers
        
        # Add edges from this vertex to its neighbors
        for neighbor in neighbors:
            # For undirected graphs, we write both (u,v) and (v,u)
            # Edge (u,v) goes to worker owning u
            partitioned_edges[worker_idx].append((vertex_id, neighbor))
            
            # Edge (v,u) goes to worker owning v
            neighbor_worker = neighbor % num_workers
            partitioned_edges[neighbor_worker].append((neighbor, vertex_id))
    
    # Write partitioned files
    total_written = 0
    for worker_idx in range(num_workers):
        # File names: 1.txt for worker-0, 2.txt for worker-1, etc.
        output_file = os.path.join(output_dir, f"{worker_idx + 1}.txt")
        
        edges = partitioned_edges[worker_idx]
        with open(output_file, 'w') as f:
            for u, v in edges:
                f.write(f"{u} {v}\n")
        
        # Count unique edges (to avoid counting duplicates from undirected storage)
        unique_edges = len(set(edges))
        print(f"Partition {worker_idx + 1} (worker-{worker_idx}): {len(edges)} edges written ({unique_edges} unique)")
        total_written += len(edges)
    
    print(f"\nTotal edges written: {total_written}")
    print(f"Partitioned files written to: {output_dir}/")
    
    # Generate vertex assignment (round-robin)
    vertex_assignment = {}
    for vertex_id in range(graph_size):
        worker_idx = vertex_id % num_workers
        vertex_assignment[str(vertex_id)] = f"worker-{worker_idx}"
    
    print(f"\n✓ Generated vertex assignment for {len(vertex_assignment)} vertices")
    
    # Update config file if provided
    if config_file and os.path.exists(config_file):
        print(f"Updating config file: {config_file}")
        with open(config_file, 'r') as f:
            config = yaml.safe_load(f)
        
        # Update worker_config with vertex_assignment
        if 'worker_config' not in config:
            config['worker_config'] = {}
        config['worker_config']['vertex_assignment'] = vertex_assignment
        config['worker_config']['num_workers'] = num_workers
        
        # Save updated config
        save_config(config_file, config)
        print(f"✓ Updated config file with vertex assignment: {config_file}")
    elif config_file:
        print(f"Warning: Config file not found: {config_file} (skipping config update)")


def main():
    parser = argparse.ArgumentParser(
        description="Partition an existing graph file for silhouette-db workers"
    )
    parser.add_argument(
        'input_file',
        type=str,
        help='Path to input edge list file (format: "u v" per line)'
    )
    parser.add_argument(
        'num_workers',
        type=int,
        help='Number of workers'
    )
    parser.add_argument(
        '--output-dir',
        type=str,
        default='data',
        help='Output directory for partitioned files (default: data)'
    )
    parser.add_argument(
        '--graph-size',
        type=int,
        default=None,
        help='Graph size (number of vertices). If not provided, inferred from graph.'
    )
    parser.add_argument(
        '--config',
        type=str,
        default=None,
        help='Path to config file to update with vertex assignments (optional)'
    )
    
    args = parser.parse_args()
    
    if not os.path.exists(args.input_file):
        print(f"Error: Input file {args.input_file} does not exist")
        return 1
    
    partition_graph(
        args.input_file,
        args.output_dir,
        args.num_workers,
        args.graph_size,
        args.config
    )
    
    return 0


if __name__ == '__main__':
    exit(main())


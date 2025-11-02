#!/usr/bin/env python3
"""
Generate random undirected graphs and partition them for workers.

This script generates a random graph, partitions it based on algorithm configuration,
and saves partitioned edge lists to files (1.txt, 2.txt, ...) in the data/ directory.
"""

import argparse
import random
import yaml
import os
from pathlib import Path
from typing import Dict, List, Tuple


def generate_random_graph(num_vertices: int, num_edges: int, seed: int = None) -> List[Tuple[int, int]]:
    """
    Generate a random undirected graph.
    
    Args:
        num_vertices: Number of vertices in the graph
        num_edges: Number of edges (will be stored twice: (u,v) and (v,u))
        seed: Random seed for reproducibility
    
    Returns:
        List of edges as (u, v) tuples. Each edge appears twice (undirected).
    """
    if seed is not None:
        random.seed(seed)
    
    # Generate unique edges
    edges_set = set()
    max_possible_edges = num_vertices * (num_vertices - 1) // 2
    
    if num_edges > max_possible_edges:
        raise ValueError(f"Cannot generate {num_edges} edges for {num_vertices} vertices (max: {max_possible_edges})")
    
    while len(edges_set) < num_edges:
        u = random.randint(0, num_vertices - 1)
        v = random.randint(0, num_vertices - 1)
        if u != v:
            # Store edges in canonical form (smaller vertex first)
            edge = (min(u, v), max(u, v))
            edges_set.add(edge)
    
    # Convert to list and add reverse edges (undirected graph)
    edges = []
    for u, v in edges_set:
        edges.append((u, v))
        edges.append((v, u))  # Reverse edge for undirected graph
    
    return edges


def assign_vertices(num_vertices: int, num_workers: int, 
                   custom_assignment: Dict[int, str] = None) -> Dict[int, int]:
    """
    Assign vertices to workers.
    
    Args:
        num_vertices: Total number of vertices
        num_workers: Number of workers
        custom_assignment: Optional custom vertex -> worker mapping (vertex_id -> worker_index)
    
    Returns:
        Dictionary mapping vertex_id -> worker_index (0-based)
    """
    assignment = {}
    
    # Apply custom assignment if provided
    if custom_assignment:
        for vertex_id, worker_str in custom_assignment.items():
            # Extract worker index from "worker-X" format
            try:
                worker_idx = int(worker_str.split('-')[-1])
                if 0 <= worker_idx < num_workers:
                    assignment[vertex_id] = worker_idx
            except (ValueError, IndexError):
                pass
    
    # Fill remaining vertices with round-robin assignment
    for vertex_id in range(num_vertices):
        if vertex_id not in assignment:
            assignment[vertex_id] = vertex_id % num_workers
    
    return assignment


def partition_edges(edges: List[Tuple[int, int]], vertex_assignment: Dict[int, int], 
                    num_workers: int) -> Dict[int, List[Tuple[int, int]]]:
    """
    Partition edges based on vertex assignment.
    
    An edge (u, v) is assigned to the worker that owns vertex u.
    
    Args:
        edges: List of edges (u, v) tuples
        vertex_assignment: Dictionary mapping vertex_id -> worker_index
        num_workers: Number of workers
    
    Returns:
        Dictionary mapping worker_index -> list of edges for that worker
    """
    partitioned = {i: [] for i in range(num_workers)}
    
    for u, v in edges:
        # Assign edge to the worker that owns vertex u
        worker_idx = vertex_assignment[u]
        partitioned[worker_idx].append((u, v))
    
    return partitioned


def write_edge_file(filepath: Path, edges: List[Tuple[int, int]]):
    """Write edges to a file in edge list format (u v per line)."""
    with open(filepath, 'w') as f:
        for u, v in edges:
            f.write(f"{u} {v}\n")


def load_config(config_file: str) -> dict:
    """Load algorithm configuration from YAML file."""
    with open(config_file, 'r') as f:
        return yaml.safe_load(f)


def main():
    parser = argparse.ArgumentParser(
        description='Generate random graphs and partition them for workers'
    )
    parser.add_argument(
        '--config',
        type=str,
        required=True,
        help='Path to algorithm configuration YAML file'
    )
    parser.add_argument(
        '--num-vertices',
        type=int,
        help='Number of vertices (overrides config if specified)'
    )
    parser.add_argument(
        '--num-edges',
        type=int,
        help='Number of edges (overrides config if specified)'
    )
    parser.add_argument(
        '--seed',
        type=int,
        default=None,
        help='Random seed for reproducibility'
    )
    parser.add_argument(
        '--output-dir',
        type=str,
        default='data',
        help='Output directory for partitioned graph files (default: data/)'
    )
    parser.add_argument(
        '--global-graph',
        type=str,
        default=None,
        help='Path to save the complete graph file (optional)'
    )
    
    args = parser.parse_args()
    
    # Load configuration
    config = load_config(args.config)
    
    # Get parameters
    worker_config = config.get('worker_config', {})
    num_workers = worker_config.get('num_workers', 1)
    custom_vertex_assignment = worker_config.get('vertex_assignment', {})
    
    # Determine number of vertices and edges
    if args.num_vertices:
        num_vertices = args.num_vertices
    else:
        graph_config = config.get('graph_config', {})
        num_vertices = graph_config.get('num_vertices')
        if num_vertices is None:
            # Try to infer from edges if provided
            edges_config = graph_config.get('edges', [])
            if edges_config:
                all_vertices = set()
                for edge in edges_config:
                    all_vertices.add(edge.get('u'))
                    all_vertices.add(edge.get('v'))
                num_vertices = len(all_vertices)
            else:
                num_vertices = 100  # Default
    
    if args.num_edges:
        num_edges = args.num_edges
    else:
        # Calculate default based on vertices (sparse graph: ~2*vertices edges)
        num_edges = num_vertices * 2
    
    print(f"Generating graph with {num_vertices} vertices and {num_edges} edges...")
    print(f"Number of workers: {num_workers}")
    
    # Generate graph
    edges = generate_random_graph(num_vertices, num_edges, args.seed)
    print(f"Generated {len(edges)} edges (including reverse edges for undirected graph)")
    
    # Assign vertices to workers
    vertex_assignment = assign_vertices(num_vertices, num_workers, custom_vertex_assignment)
    
    # Print vertex assignment statistics
    worker_vertex_counts = {}
    for vertex_id, worker_idx in vertex_assignment.items():
        worker_vertex_counts[worker_idx] = worker_vertex_counts.get(worker_idx, 0) + 1
    
    print("\nVertex assignment:")
    for worker_idx in sorted(worker_vertex_counts.keys()):
        print(f"  Worker {worker_idx}: {worker_vertex_counts[worker_idx]} vertices")
    
    # Partition edges
    partitioned = partition_edges(edges, vertex_assignment, num_workers)
    
    # Create output directory
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    
    # Write partitioned files
    print(f"\nWriting partitioned graph files to {output_dir}/...")
    for worker_idx in range(num_workers):
        worker_edges = partitioned[worker_idx]
        filepath = output_dir / f"{worker_idx + 1}.txt"  # Files: 1.txt, 2.txt, ...
        write_edge_file(filepath, worker_edges)
        print(f"  Worker {worker_idx} ({filepath.name}): {len(worker_edges)} edges")
    
    # Write global graph file if requested
    if args.global_graph:
        global_path = Path(args.global_graph)
        write_edge_file(global_path, edges)
        print(f"\nGlobal graph written to {global_path}")
    
    print("\n✓ Graph generation complete!")


if __name__ == '__main__':
    main()


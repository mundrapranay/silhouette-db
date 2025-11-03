package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mundrapranay/silhouette-db/algorithms"
	"github.com/mundrapranay/silhouette-db/algorithms/common"
	"github.com/mundrapranay/silhouette-db/pkg/client"

	// Import algorithm registries to register algorithms
	_ "github.com/mundrapranay/silhouette-db/algorithms/exact"
	_ "github.com/mundrapranay/silhouette-db/algorithms/ledp"
)

var (
	configFile = flag.String("config", "", "Path to algorithm configuration file (required)")
	verbose    = flag.Bool("verbose", false, "Enable verbose logging")
)

func main() {
	flag.Parse()

	if *configFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <config.yaml>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExample config file:\n")
		printExampleConfig()
		os.Exit(1)
	}

	// Load configuration
	config, err := common.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *verbose {
		log.Printf("Loaded configuration:")
		log.Printf("  Algorithm: %s (%s)", config.AlgorithmName, config.AlgorithmType)
		log.Printf("  Server: %s", config.ServerAddress)
		log.Printf("  Workers: %d", config.WorkerConfig.NumWorkers)
	}

	// Get algorithm instance
	algorithm, err := algorithms.GetAlgorithm(config.AlgorithmType, config.AlgorithmName)
	if err != nil {
		log.Fatalf("Failed to get algorithm: %v", err)
	}

	// Load graph data (pass workerID for local testing mode)
	graphData, err := common.LoadGraphData(&config.GraphConfig, config.WorkerConfig.WorkerID)
	if err != nil {
		log.Fatalf("Failed to load graph data: %v", err)
	}

	if *verbose {
		log.Printf("Loaded graph:")
		log.Printf("  Vertices: %d", graphData.NumVertices)
		log.Printf("  Edges: %d", graphData.NumEdges)
	}

	// Initialize algorithm
	ctx := context.Background()

	// Merge worker config and parameters for algorithm initialization
	initConfig := make(map[string]interface{})
	for k, v := range config.Parameters {
		initConfig[k] = v
	}
	initConfig["worker_id"] = config.WorkerConfig.WorkerID
	initConfig["num_workers"] = config.WorkerConfig.NumWorkers
	if len(config.WorkerConfig.VertexAssignment) > 0 {
		// Convert map[string]string to map[string]interface{} for algorithm initialization
		vertexAssign := make(map[string]interface{})
		for k, v := range config.WorkerConfig.VertexAssignment {
			vertexAssign[k] = v
		}
		initConfig["vertex_assignment"] = vertexAssign
	}

	if err := algorithm.Initialize(ctx, graphData, initConfig); err != nil {
		log.Fatalf("Failed to initialize algorithm: %v", err)
	}

	// Connect to silhouette-db server
	if *verbose {
		log.Printf("Connecting to silhouette-db server at %s...", config.ServerAddress)
	}

	dbClient, err := client.NewClient(config.ServerAddress, nil)
	if err != nil {
		log.Fatalf("Failed to connect to silhouette-db server: %v", err)
	}
	defer dbClient.Close()

	if *verbose {
		log.Printf("Connected successfully!")
	}

	// Determine number of rounds from config or default
	numRounds := 10 // Default
	if rounds, ok := config.Parameters["num_rounds"].(int); ok {
		numRounds = rounds
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Execute algorithm
	done := make(chan error, 1)
	go func() {
		log.Printf("Executing algorithm '%s' for %d rounds...", config.AlgorithmName, numRounds)
		result, err := algorithm.Execute(ctx, dbClient, numRounds)
		if err != nil {
			done <- err
			return
		}

		// Print results
		printResults(result)
		done <- nil
	}()

	// Wait for completion or signal
	select {
	case err := <-done:
		if err != nil {
			log.Fatalf("Algorithm execution failed: %v", err)
		}
		log.Printf("Algorithm execution completed successfully!")
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down...", sig)
		// TODO: Implement graceful shutdown (wait for current round to complete)
		os.Exit(1)
	}
}

func printResults(result *common.AlgorithmResult) {
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Algorithm Results: %s\n", result.AlgorithmName)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Rounds executed:    %d\n", result.NumRounds)
	fmt.Printf("  Converged:          %v\n", result.Converged)
	if result.Converged {
		fmt.Printf("  Convergence round:  %d\n", result.ConvergenceRound)
	}

	if len(result.Results) > 0 {
		fmt.Println()
		fmt.Println("  Results:")
		for key, value := range result.Results {
			fmt.Printf("    %s: %v\n", key, value)
		}
	}

	if len(result.Metadata) > 0 {
		fmt.Println()
		fmt.Println("  Metadata:")
		for key, value := range result.Metadata {
			fmt.Printf("    %s: %v\n", key, value)
		}
	}
	fmt.Println()
}

func printExampleConfig() {
	example := `algorithm_name: shortest_path
algorithm_type: exact  # or 'ledp'
server_address: "127.0.0.1:9090"

worker_config:
  num_workers: 5
  worker_id: "worker-0"
  # vertex_assignment:  # Optional, custom vertex-to-worker mapping

graph_config:
  format: "edgelist"  # or "adjacency_list"
  file_path: "/path/to/graph.txt"  # Path to graph file
  # OR specify edges directly:
  # edges:
  #   - u: 0
  #     v: 1
  #     w: 1.5  # Optional weight
  #   - u: 1
  #     v: 2
  directed: false  # true for directed graphs

parameters:
  num_rounds: 10
  # Algorithm-specific parameters go here
`
	fmt.Print(example)
}

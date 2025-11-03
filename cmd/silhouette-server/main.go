package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"github.com/mundrapranay/silhouette-db/internal/crypto"
	"github.com/mundrapranay/silhouette-db/internal/server"
	"github.com/mundrapranay/silhouette-db/internal/store"
)

var (
	nodeID         = flag.String("node-id", "", "Unique ID for this node")
	listenAddr     = flag.String("listen-addr", "127.0.0.1:8080", "Address to listen for Raft communication")
	grpcAddr       = flag.String("grpc-addr", "127.0.0.1:9090", "Address to listen for gRPC API")
	dataDir        = flag.String("data-dir", "./data", "Directory to store Raft logs and snapshots")
	bootstrap      = flag.Bool("bootstrap", false, "Bootstrap a new cluster (first node)")
	joinAddr       = flag.String("join", "", "Address of an existing cluster member to join")
	storageBackend = flag.String("storage-backend", "okvs", "Storage backend: 'okvs' or 'kvs' (default: okvs)")
)

func main() {
	flag.Parse()

	if *nodeID == "" {
		log.Fatal("node-id is required")
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	// Initialize Raft store
	storeConfig := store.Config{
		NodeID:           *nodeID,
		ListenAddr:       *listenAddr,
		DataDir:          *dataDir,
		Bootstrap:        *bootstrap,
		HeartbeatTimeout: 1000 * time.Millisecond,
		ElectionTimeout:  1000 * time.Millisecond,
		CommitTimeout:    50 * time.Millisecond,
	}

	s, err := store.NewStore(storeConfig)
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}
	defer s.Shutdown()

	// If join address is provided, join the cluster
	if *joinAddr != "" && !*bootstrap {
		// Note: In a production system, you'd want a proper join RPC endpoint
		// For now, this is a placeholder
		log.Printf("Join address provided: %s (joining not yet implemented)", *joinAddr)
	}

	// Initialize cryptographic components based on storage backend
	var okvsEncoder crypto.OKVSEncoder
	switch *storageBackend {
	case "kvs":
		okvsEncoder = crypto.NewKVSEncoder()
		log.Printf("Using KVS (simple key-value store) backend")
	case "okvs":
		okvsEncoder = crypto.NewRBOKVSEncoder()
		log.Printf("Using OKVS (oblivious key-value store) backend")
	default:
		log.Fatalf("Invalid storage backend: %s (must be 'okvs' or 'kvs')", *storageBackend)
	}
	// Note: PIR server is now created per round in server.PublishValues

	// Create gRPC server
	grpcServer := server.NewServer(s, okvsEncoder, *storageBackend)

	// Start gRPC server
	lis, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcSrv := grpc.NewServer()
	apiv1.RegisterCoordinationServiceServer(grpcSrv, grpcServer)

	log.Printf("Starting gRPC server on %s", *grpcAddr)
	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			log.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	// Wait for leadership if bootstrapping
	if *bootstrap {
		log.Println("Bootstrapping cluster...")
		for {
			if s.IsLeader() {
				log.Println("Became leader!")
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	log.Printf("Node %s is ready. Raft: %s, gRPC: %s", *nodeID, *listenAddr, *grpcAddr)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	grpcSrv.GracefulStop()
}

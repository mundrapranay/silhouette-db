package server

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"github.com/mundrapranay/silhouette-db/internal/crypto"
	"github.com/mundrapranay/silhouette-db/internal/store"
)

// Server implements the CoordinationService gRPC server.
type Server struct {
	apiv1.UnimplementedCoordinationServiceServer

	store       *store.Store
	okvsEncoder crypto.OKVSEncoder
	pirServer   crypto.PIRServer

	// Round management
	roundsMu        sync.RWMutex
	roundData       map[uint64]*roundState
	expectedWorkers map[uint64]int32
}

// roundState tracks the state of a round during the publish phase.
type roundState struct {
	mu         sync.Mutex
	workerData map[string][]apiv1.KeyValuePair // worker_id -> pairs
	complete   bool
}

// NewServer creates a new gRPC server instance.
func NewServer(s *store.Store, okvsEncoder crypto.OKVSEncoder, pirServer crypto.PIRServer) *Server {
	return &Server{
		store:           s,
		okvsEncoder:     okvsEncoder,
		pirServer:       pirServer,
		roundData:       make(map[uint64]*roundState),
		expectedWorkers: make(map[uint64]int32),
	}
}

// StartRound initializes a new synchronous round for data submission.
func (s *Server) StartRound(ctx context.Context, req *apiv1.StartRoundRequest) (*apiv1.StartRoundResponse, error) {
	// Only leader can start a round
	if !s.store.IsLeader() {
		return nil, status.Errorf(codes.FailedPrecondition, "not the leader")
	}

	s.roundsMu.Lock()
	defer s.roundsMu.Unlock()

	// Initialize round state
	s.roundData[req.RoundId] = &roundState{
		workerData: make(map[string][]apiv1.KeyValuePair),
		complete:   false,
	}
	s.expectedWorkers[req.RoundId] = req.ExpectedWorkers

	return &apiv1.StartRoundResponse{Success: true}, nil
}

// PublishValues allows a worker to submit its key-value pairs for a given round.
func (s *Server) PublishValues(ctx context.Context, req *apiv1.PublishValuesRequest) (*apiv1.PublishValuesResponse, error) {
	// Only leader can accept publishes
	if !s.store.IsLeader() {
		return nil, status.Errorf(codes.FailedPrecondition, "not the leader")
	}

	s.roundsMu.Lock()
	roundState, exists := s.roundData[req.RoundId]
	expected := s.expectedWorkers[req.RoundId]
	s.roundsMu.Unlock()

	if !exists {
		return nil, status.Errorf(codes.NotFound, "round %d not found", req.RoundId)
	}

	if roundState.complete {
		return nil, status.Errorf(codes.AlreadyExists, "round %d already completed", req.RoundId)
	}

	// Record worker's contribution
	roundState.mu.Lock()
	// Convert []*KeyValuePair to []KeyValuePair
	pairs := make([]apiv1.KeyValuePair, len(req.Pairs))
	for i, p := range req.Pairs {
		pairs[i] = *p
	}
	roundState.workerData[req.WorkerId] = pairs
	numWorkers := len(roundState.workerData)
	roundState.mu.Unlock()

	// Check if all workers have submitted
	if int32(numWorkers) >= expected {
		// Aggregate all pairs
		roundState.mu.Lock()
		allPairs := make(map[string][]byte)
		for _, pairs := range roundState.workerData {
			for i := range pairs {
				pair := &pairs[i]
				allPairs[pair.Key] = pair.Value
			}
		}
		roundState.complete = true
		roundState.mu.Unlock()

		// Encode using OKVS
		okvsBlob, err := s.okvsEncoder.Encode(allPairs)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to encode OKVS: %v", err)
		}

		// Store in Raft cluster
		roundKey := fmt.Sprintf("round_%d_results", req.RoundId)
		if err := s.store.Set(roundKey, okvsBlob); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to store round data: %v", err)
		}
	}

	return &apiv1.PublishValuesResponse{Success: true}, nil
}

// GetValue allows a worker to privately retrieve a value for a specific key
// from a completed round using a PIR query.
func (s *Server) GetValue(ctx context.Context, req *apiv1.GetValueRequest) (*apiv1.GetValueResponse, error) {
	// Forward to leader if not the leader
	if !s.store.IsLeader() {
		return nil, status.Errorf(codes.FailedPrecondition, "not the leader")
	}

	// Retrieve the OKVS blob for this round
	roundKey := fmt.Sprintf("round_%d_results", req.RoundId)
	okvsBlob, exists := s.store.Get(roundKey)
	if !exists {
		return nil, status.Errorf(codes.NotFound, "round %d results not found", req.RoundId)
	}

	// Process PIR query
	pirResponse, err := s.pirServer.ProcessQuery(okvsBlob, req.PirQuery)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to process PIR query: %v", err)
	}

	return &apiv1.GetValueResponse{PirResponse: pirResponse}, nil
}

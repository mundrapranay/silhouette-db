package server

import (
	"context"
	"fmt"
	"sort"
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

	store          *store.Store
	okvsEncoder    crypto.OKVSEncoder
	storageBackend string // "okvs" or "kvs"

	// Round management
	roundsMu        sync.RWMutex
	roundData       map[uint64]*roundState
	expectedWorkers map[uint64]int32

	// FrodoPIR servers per round
	pirServers      map[uint64]*crypto.FrodoPIRServer
	roundBaseParams map[uint64][]byte         // BaseParams for each round (for client distribution)
	roundKeyMapping map[uint64]map[string]int // key-to-index mapping per round

	// Storage per round (OKVS or KVS)
	storageBlobs    map[uint64][]byte             // Encoded blobs per round (OKVS or KVS)
	storageDecoders map[uint64]crypto.OKVSDecoder // Decoders per round (interface type)
}

// roundState tracks the state of a round during the publish phase.
type roundState struct {
	mu         sync.Mutex
	workerData map[string][]apiv1.KeyValuePair // worker_id -> pairs
	complete   bool
}

// NewServer creates a new gRPC server instance.
// storageBackend must be "okvs" or "kvs"
func NewServer(s *store.Store, okvsEncoder crypto.OKVSEncoder, storageBackend string) *Server {
	if storageBackend == "" {
		storageBackend = "okvs" // Default to OKVS
	}
	return &Server{
		store:           s,
		okvsEncoder:     okvsEncoder,
		storageBackend:  storageBackend,
		roundData:       make(map[uint64]*roundState),
		expectedWorkers: make(map[uint64]int32),
		pirServers:      make(map[uint64]*crypto.FrodoPIRServer),
		roundBaseParams: make(map[uint64][]byte),
		roundKeyMapping: make(map[uint64]map[string]int),
		storageBlobs:    make(map[uint64][]byte),
		storageDecoders: make(map[uint64]crypto.OKVSDecoder),
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

		// Create key-to-index mapping (ordered by key for consistency)
		keys := make([]string, 0, len(allPairs))
		for k := range allPairs {
			keys = append(keys, k)
		}
		// Sort keys for deterministic ordering
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		keyToIndex := make(map[string]int)
		for i, k := range keys {
			keyToIndex[k] = i
		}

		// Handle empty rounds (when all workers publish empty pairs)
		// For empty rounds, we skip PIR server creation but still complete the round
		// This allows synchronization-only rounds where no data is published
		if len(allPairs) == 0 {
			// Empty round: just store empty data and skip PIR server creation
			roundKey := fmt.Sprintf("round_%d_results", req.RoundId)
			var storageData []byte
			if err := s.store.Set(roundKey, storageData); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to store round data: %v", err)
			}
			// Mark round as complete (but no PIR server available)
			s.roundsMu.Lock()
			// Store empty key mapping to indicate round exists but has no data
			s.roundKeyMapping[req.RoundId] = make(map[string]int)
			s.roundsMu.Unlock()
			return &apiv1.PublishValuesResponse{Success: true}, nil
		}

		// Use storage backend based on configuration
		var storageBlob []byte
		var storageDecoder crypto.OKVSDecoder
		var pirPairs map[string][]byte
		var err error

		if s.storageBackend == "okvs" {
			// Encode using OKVS
			// Note: RB-OKVS requires:
			// - Minimum 100 key-value pairs
			// - Values must be exactly 8 bytes (float64, little-endian)
			if len(allPairs) < 100 {
				return nil, status.Errorf(codes.InvalidArgument, "OKVS requires at least 100 pairs, got %d", len(allPairs))
			}
			storageBlob, err = s.okvsEncoder.Encode(allPairs)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to encode OKVS: %v", err)
			}

			// Create OKVS decoder for this round
			storageDecoder = crypto.NewRBOKVSDecoder(storageBlob)

			// For PIR, we need to decode all values from OKVS to create the PIR database
			// This maintains the oblivious property: the PIR database contains OKVS-decoded values
			pirPairs = make(map[string][]byte, len(allPairs))
			for _, key := range keys {
				decodedValue, err := storageDecoder.Decode(storageBlob, key)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "failed to decode OKVS value for key %s: %v", key, err)
				}
				pirPairs[key] = decodedValue
			}
		} else if s.storageBackend == "kvs" {
			// Encode using simple KVS (KVS encoder should be used for KVS backend)
			// Note: okvsEncoder is actually a KVSEncoder when backend is "kvs"
			storageBlob, err = s.okvsEncoder.Encode(allPairs)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to encode KVS: %v", err)
			}

			// Create KVS decoder for this round
			storageDecoder, err = crypto.NewKVSDecoder(storageBlob)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "failed to create KVS decoder: %v", err)
			}

			// For PIR, decode all values from KVS to create the PIR database
			pirPairs = make(map[string][]byte, len(allPairs))
			for _, key := range keys {
				decodedValue, err := storageDecoder.Decode(storageBlob, key)
				if err != nil {
					return nil, status.Errorf(codes.Internal, "failed to decode KVS value for key %s: %v", key, err)
				}
				pirPairs[key] = decodedValue
			}
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "invalid storage backend: %s (must be 'okvs' or 'kvs')", s.storageBackend)
		}

		// Create FrodoPIR server for this round
		// Calculate elemSize based on actual data (max value size in BYTES)
		// Note: FrodoPIR expects elemSize in BITS, and decodes base64 to bytes
		// Base64 encoding increases size by ~33%, so we need to account for that
		maxValueBytes := 0
		for _, v := range pirPairs {
			if len(v) > maxValueBytes {
				maxValueBytes = len(v)
			}
		}

		// Ensure minimum size (64 bytes = 512 bits)
		// Round up to next power of 2 for efficiency
		if maxValueBytes < 64 {
			maxValueBytes = 64
		}
		// Round up to next power of 2
		elemSizeBytes := 64
		for elemSizeBytes < maxValueBytes {
			elemSizeBytes *= 2
		}

		// Parameters: lweDim=512 for small databases, plaintextBits=10
		// elemSize is in BITS (library expects bits)
		lweDim := 512
		elemSizeBits := elemSizeBytes * 8 // Convert bytes to bits
		plaintextBits := 10

		pirServer, baseParams, err := crypto.NewFrodoPIRServer(pirPairs, lweDim, elemSizeBits, plaintextBits)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to create FrodoPIR server: %v", err)
		}

		// Store FrodoPIR server, storage blob/decoder, and metadata
		s.roundsMu.Lock()
		s.pirServers[req.RoundId] = pirServer
		s.roundBaseParams[req.RoundId] = baseParams
		s.roundKeyMapping[req.RoundId] = keyToIndex
		s.storageBlobs[req.RoundId] = storageBlob
		s.storageDecoders[req.RoundId] = storageDecoder
		s.roundsMu.Unlock()

		// Store in Raft cluster
		roundKey := fmt.Sprintf("round_%d_results", req.RoundId)
		storageData := storageBlob
		if err := s.store.Set(roundKey, storageData); err != nil {
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

	s.roundsMu.RLock()
	pirServer, pirServerExists := s.pirServers[req.RoundId]
	keyMappingExists := s.roundKeyMapping[req.RoundId] != nil
	s.roundsMu.RUnlock()

	// Check if round exists (even if empty)
	if !keyMappingExists {
		return nil, status.Errorf(codes.NotFound, "round %d not found", req.RoundId)
	}

	// Check if round is empty (no PIR server means empty round)
	if !pirServerExists {
		return nil, status.Errorf(codes.FailedPrecondition, "round %d is empty (no data available)", req.RoundId)
	}

	// Process PIR query using FrodoPIR server
	// Note: db parameter is not used, shard already contains the database
	pirResponse, err := pirServer.ProcessQuery(nil, req.PirQuery)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to process PIR query: %v", err)
	}

	return &apiv1.GetValueResponse{PirResponse: pirResponse}, nil
}

// GetBaseParams returns the serialized BaseParams for a round (for client initialization).
// This allows clients to create FrodoPIR clients for querying.
func (s *Server) GetBaseParams(ctx context.Context, req *apiv1.GetBaseParamsRequest) (*apiv1.GetBaseParamsResponse, error) {
	// Forward to leader if not the leader
	if !s.store.IsLeader() {
		return nil, status.Errorf(codes.FailedPrecondition, "not the leader")
	}

	s.roundsMu.RLock()
	baseParams, exists := s.roundBaseParams[req.RoundId]
	s.roundsMu.RUnlock()

	if !exists {
		return nil, status.Errorf(codes.NotFound, "round %d base params not found", req.RoundId)
	}

	return &apiv1.GetBaseParamsResponse{BaseParams: baseParams}, nil
}

// GetKeyMapping returns the key-to-index mapping for a round (for client queries).
func (s *Server) GetKeyMapping(ctx context.Context, req *apiv1.GetKeyMappingRequest) (*apiv1.GetKeyMappingResponse, error) {
	// Forward to leader if not the leader
	if !s.store.IsLeader() {
		return nil, status.Errorf(codes.FailedPrecondition, "not the leader")
	}

	s.roundsMu.RLock()
	keyMapping, exists := s.roundKeyMapping[req.RoundId]
	s.roundsMu.RUnlock()

	if !exists {
		return nil, status.Errorf(codes.NotFound, "round %d key mapping not found", req.RoundId)
	}

	// Convert map to protobuf format
	entries := make([]*apiv1.KeyMappingEntry, 0, len(keyMapping))
	for key, index := range keyMapping {
		entries = append(entries, &apiv1.KeyMappingEntry{
			Key:   key,
			Index: int32(index),
		})
	}

	return &apiv1.GetKeyMappingResponse{Entries: entries}, nil
}

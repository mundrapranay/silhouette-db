package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	apiv1 "github.com/mundrapranay/silhouette-db/api/v1"
	"github.com/mundrapranay/silhouette-db/internal/crypto"
)

// Client provides a Go client library for LEDP workers to interact with
// the silhouette-db coordination layer.
type Client struct {
	conn      *grpc.ClientConn
	service   apiv1.CoordinationServiceClient
	pirClient interface { // PIRClient interface
		GenerateQuery(key string) ([]byte, error)
		DecodeResponse(response []byte) ([]byte, error)
		Close() error
	}
	keyMappings map[uint64]map[string]int // Cache key mappings per round
}

// NewClient creates a new client connection to a silhouette-db server.
// If pirClient is nil, the client will fetch BaseParams and create a FrodoPIR client.
func NewClient(serverAddr string, pirClient interface {
	GenerateQuery(key string) ([]byte, error)
	DecodeResponse(response []byte) ([]byte, error)
	Close() error
}) (*Client, error) {
	conn, err := grpc.Dial(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		conn:        conn,
		service:     apiv1.NewCoordinationServiceClient(conn),
		pirClient:   pirClient,
		keyMappings: make(map[uint64]map[string]int),
	}, nil
}

// InitializePIRClient initializes a FrodoPIR client for a specific round.
// This fetches BaseParams and key mapping from the server.
func (c *Client) InitializePIRClient(ctx context.Context, roundID uint64) error {
	// Fetch BaseParams from server
	baseParamsReq := &apiv1.GetBaseParamsRequest{RoundId: roundID}
	baseParamsResp, err := c.service.GetBaseParams(ctx, baseParamsReq)
	if err != nil {
		return fmt.Errorf("failed to get base params: %w", err)
	}

	// Fetch key mapping from server
	keyMappingReq := &apiv1.GetKeyMappingRequest{RoundId: roundID}
	keyMappingResp, err := c.service.GetKeyMapping(ctx, keyMappingReq)
	if err != nil {
		return fmt.Errorf("failed to get key mapping: %w", err)
	}

	// Convert protobuf entries to map
	keyToIndex := make(map[string]int)
	for _, entry := range keyMappingResp.Entries {
		keyToIndex[entry.Key] = int(entry.Index)
	}
	c.keyMappings[roundID] = keyToIndex

	// Create FrodoPIR client
	pirClient, err := crypto.NewFrodoPIRClient(baseParamsResp.BaseParams, keyToIndex)
	if err != nil {
		return fmt.Errorf("failed to create FrodoPIR client: %w", err)
	}

	c.pirClient = pirClient
	return nil
}

// Close closes the client connection and frees PIR client resources.
func (c *Client) Close() error {
	if c.pirClient != nil {
		if err := c.pirClient.Close(); err != nil {
			return fmt.Errorf("failed to close PIR client: %w", err)
		}
	}
	return c.conn.Close()
}

// StartRound initializes a new round on the server.
func (c *Client) StartRound(ctx context.Context, roundID uint64, expectedWorkers int32) error {
	req := &apiv1.StartRoundRequest{
		RoundId:         roundID,
		ExpectedWorkers: expectedWorkers,
	}

	resp, err := c.service.StartRound(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start round: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("server returned failure for start round")
	}

	return nil
}

// PublishValues publishes key-value pairs for a given round.
func (c *Client) PublishValues(ctx context.Context, roundID uint64, workerID string, pairs map[string][]byte) error {
	kvPairs := make([]*apiv1.KeyValuePair, 0, len(pairs))
	for k, v := range pairs {
		kvPairs = append(kvPairs, &apiv1.KeyValuePair{
			Key:   k,
			Value: v,
		})
	}

	req := &apiv1.PublishValuesRequest{
		RoundId:  roundID,
		WorkerId: workerID,
		Pairs:    kvPairs,
	}

	resp, err := c.service.PublishValues(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to publish values: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("server returned failure for publish values")
	}

	return nil
}

// GetValue retrieves a value for a specific key from a round using PIR.
// If the PIR client hasn't been initialized for this round, it will be initialized automatically.
func (c *Client) GetValue(ctx context.Context, roundID uint64, key string) ([]byte, error) {
	// Initialize PIR client if not already done for this round
	if c.pirClient == nil || c.keyMappings[roundID] == nil {
		if err := c.InitializePIRClient(ctx, roundID); err != nil {
			return nil, fmt.Errorf("failed to initialize PIR client: %w", err)
		}
	}

	// Generate PIR query for the key
	query, err := c.pirClient.GenerateQuery(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PIR query: %w", err)
	}

	// Send GetValue request
	req := &apiv1.GetValueRequest{
		RoundId:  roundID,
		PirQuery: query,
	}

	resp, err := c.service.GetValue(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get value: %w", err)
	}

	// Decode PIR response
	value, err := c.pirClient.DecodeResponse(resp.PirResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PIR response: %w", err)
	}

	return value, nil
}

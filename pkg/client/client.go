package client

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/mundrapranay/silhouette-db/internal/crypto"
)

// Client provides a Go client library for LEDP workers to interact with
// the silhouette-db coordination layer.
type Client struct {
	conn      *grpc.ClientConn
	service   apiv1.CoordinationServiceClient
	pirClient crypto.PIRClient
}

// NewClient creates a new client connection to a silhouette-db server.
func NewClient(serverAddr string, pirClient crypto.PIRClient) (*Client, error) {
	conn, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Client{
		conn:      conn,
		service:   apiv1.NewCoordinationServiceClient(conn),
		pirClient: pirClient,
	}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
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
func (c *Client) GetValue(ctx context.Context, roundID uint64, key string) ([]byte, error) {
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

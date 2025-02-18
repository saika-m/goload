package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/saika-m/goload/internal/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// GRPCResponse represents the result of a gRPC request
type GRPCResponse struct {
	BytesReceived int64
	BytesSent     int64
	Error         error
	Response      interface{}
}

// GRPCClient handles gRPC protocol requests
type GRPCClient struct {
	connections map[string]*grpc.ClientConn
	kaParams    keepalive.ClientParameters
}

// NewGRPCClient creates a new gRPC client
func NewGRPCClient() *GRPCClient {
	return &GRPCClient{
		connections: make(map[string]*grpc.ClientConn),
		kaParams: keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		},
	}
}

// Execute performs a gRPC request based on the given step configuration
func (c *GRPCClient) Execute(ctx context.Context, step common.RequestStep) (*GRPCResponse, error) {
	// Parse target from path (expected format: host:port/service/method)
	target, service, method, err := parseGRPCPath(step.Path)
	if err != nil {
		return nil, err
	}

	// Get or create connection
	conn, err := c.getConnection(target)
	if err != nil {
		return nil, err
	}

	// Create request message
	request, err := createGRPCRequest(step.Body)
	if err != nil {
		return nil, err
	}

	// Prepare call options
	opts := []grpc.CallOption{
		grpc.WaitForReady(false),
	}

	// Add headers as metadata
	if len(step.Headers) > 0 {
		md := metadata.New(step.Headers)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	// Invoke method
	var response anypb.Any
	startTime := time.Now()
	err = conn.Invoke(ctx, fmt.Sprintf("/%s/%s", service, method), request, &response, opts...)
	duration := time.Since(startTime)

	// Calculate sizes
	requestSize := int64(proto.Size(request))
	responseSize := int64(proto.Size(&response))

	return &GRPCResponse{
		BytesReceived: responseSize,
		BytesSent:     requestSize,
		Error:         err,
		Response:      &response,
	}, nil
}

// Close closes all gRPC connections
func (c *GRPCClient) Close() error {
	var lastErr error
	for target, conn := range c.connections {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
		delete(c.connections, target)
	}
	return lastErr
}

// getConnection gets or creates a gRPC connection
func (c *GRPCClient) getConnection(target string) (*grpc.ClientConn, error) {
	if conn, exists := c.connections[target]; exists && conn.GetState() != connectivity.Shutdown {
		return conn, nil
	}

	// Create new connection
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(c.kaParams),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(20 * 1024 * 1024)), // 20MB
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(20 * 1024 * 1024)), // 20MB
	}

	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}

	c.connections[target] = conn
	return conn, nil
}

// parseGRPCPath parses a gRPC path into target, service, and method
func parseGRPCPath(path string) (string, string, string, error) {
	// Expected format: host:port/service/method
	var target, service, method string
	_, err := fmt.Sscanf(path, "%s/%s/%s", &target, &service, &method)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid gRPC path format (expected host:port/service/method): %w", err)
	}
	return target, service, method, nil
}

// createGRPCRequest creates a gRPC request message from JSON
func createGRPCRequest(body []byte) (*anypb.Any, error) {
	if len(body) == 0 {
		return &anypb.Any{}, nil
	}

	// Parse JSON into map
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse request body as JSON: %w", err)
	}

	// Convert to protobuf struct
	pbStruct, err := structpb.NewStruct(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to protobuf struct: %w", err)
	}

	// Pack into Any
	any, err := anypb.New(pbStruct)
	if err != nil {
		return nil, fmt.Errorf("failed to pack into Any: %w", err)
	}

	return any, nil
}

// SetKeepAliveParams sets the keepalive parameters for new connections
func (c *GRPCClient) SetKeepAliveParams(params keepalive.ClientParameters) {
	c.kaParams = params
}

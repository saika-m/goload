package protocol

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/saika-m/goload/internal/common"
)

// WSResponse represents the result of a WebSocket request
type WSResponse struct {
	BytesReceived int64
	BytesSent     int64
	Messages      [][]byte
	Error         error
}

// WSClient handles WebSocket protocol connections
type WSClient struct {
	connections map[string]*websocket.Conn
	mu          sync.RWMutex
	dialer      *websocket.Dialer
}

// NewWSClient creates a new WebSocket client
func NewWSClient() *WSClient {
	dialer := &websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  10 * time.Second,
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true}, // For testing purposes
		EnableCompression: true,
		Subprotocols:      []string{},
		ReadBufferSize:    4096,
		WriteBufferSize:   4096,
	}

	return &WSClient{
		connections: make(map[string]*websocket.Conn),
		dialer:      dialer,
	}
}

// Execute performs a WebSocket connection and message exchange
func (c *WSClient) Execute(ctx context.Context, step common.RequestStep) (*WSResponse, error) {
	// Get or create connection
	conn, err := c.getConnection(step.Path, step.Headers)
	if err != nil {
		return nil, fmt.Errorf("failed to establish WebSocket connection: %w", err)
	}

	// Set read deadline if context has deadline
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetReadDeadline(deadline)
	}

	// Send message if body is present
	var bytesSent int64
	if len(step.Body) > 0 {
		err = conn.WriteMessage(websocket.TextMessage, step.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to send WebSocket message: %w", err)
		}
		bytesSent = int64(len(step.Body))
	}

	// Read response(s)
	var messages [][]byte
	var bytesReceived int64

	// Create a channel for the read goroutine
	msgCh := make(chan []byte)
	errCh := make(chan error)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				messageType, message, err := conn.ReadMessage()
				if err != nil {
					errCh <- err
					return
				}
				if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
					select {
					case msgCh <- message:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	// Wait for message, timeout, or context cancellation
	select {
	case <-ctx.Done():
		return &WSResponse{
			BytesSent:     bytesSent,
			BytesReceived: bytesReceived,
			Messages:      messages,
			Error:         ctx.Err(),
		}, nil

	case err := <-errCh:
		return &WSResponse{
			BytesSent:     bytesSent,
			BytesReceived: bytesReceived,
			Messages:      messages,
			Error:         err,
		}, nil

	case msg := <-msgCh:
		messages = append(messages, msg)
		bytesReceived += int64(len(msg))

	case <-time.After(step.ThinkTime):
		// If think time is specified, wait for that duration
	}

	return &WSResponse{
		BytesReceived: bytesReceived,
		BytesSent:     bytesSent,
		Messages:      messages,
	}, nil
}

// getConnection gets or creates a WebSocket connection
func (c *WSClient) getConnection(urlStr string, headers map[string]string) (*websocket.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if we already have a valid connection
	if conn, exists := c.connections[urlStr]; exists {
		// Try to send a ping to verify connection is still alive
		err := conn.WriteMessage(websocket.PingMessage, nil)
		if err == nil {
			return conn, nil
		}
		// Connection is dead, remove it
		delete(c.connections, urlStr)
		conn.Close()
	}

	// Create HTTP header from the provided headers
	header := http.Header{}
	for key, value := range headers {
		header.Set(key, value)
	}

	// Establish new connection
	conn, _, err := c.dialer.Dial(urlStr, header)
	if err != nil {
		return nil, err
	}

	// Configure the connection
	conn.SetPingHandler(func(appData string) error {
		return conn.WriteMessage(websocket.PongMessage, []byte(appData))
	})

	conn.SetPongHandler(func(appData string) error {
		return nil // Simply acknowledge pongs
	})

	// Store the new connection
	c.connections[urlStr] = conn
	return conn, nil
}

// Close closes all WebSocket connections
func (c *WSClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var lastErr error
	for url, conn := range c.connections {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
		delete(c.connections, url)
	}
	return lastErr
}

// SetProxy sets the proxy for the WebSocket dialer
func (c *WSClient) SetProxy(proxyURL string) error {
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	c.dialer.Proxy = http.ProxyURL(parsedURL)
	return nil
}

// SetTLSConfig sets the TLS configuration for the WebSocket dialer
func (c *WSClient) SetTLSConfig(config *tls.Config) {
	c.dialer.TLSClientConfig = config
}

// SetHandshakeTimeout sets the handshake timeout for new connections
func (c *WSClient) SetHandshakeTimeout(timeout time.Duration) {
	c.dialer.HandshakeTimeout = timeout
}

// CloseConnection closes a specific WebSocket connection
func (c *WSClient) CloseConnection(urlStr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if conn, exists := c.connections[urlStr]; exists {
		err := conn.Close()
		delete(c.connections, urlStr)
		return err
	}
	return nil
}

// SetReadBufferSize sets the read buffer size for new connections
func (c *WSClient) SetReadBufferSize(size int) {
	c.dialer.ReadBufferSize = size
}

// SetWriteBufferSize sets the write buffer size for new connections
func (c *WSClient) SetWriteBufferSize(size int) {
	c.dialer.WriteBufferSize = size
}

// SetSubprotocols sets the subprotocols for new connections
func (c *WSClient) SetSubprotocols(protocols []string) {
	c.dialer.Subprotocols = protocols
}

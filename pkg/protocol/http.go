package protocol

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/saika-m/goload/internal/common"
)

// HTTPResponse represents the result of an HTTP request
type HTTPResponse struct {
	StatusCode    int
	BytesReceived int64
	BytesSent     int64
	Headers       map[string][]string
	Body          []byte
	Duration      time.Duration
}

// HTTPClient handles HTTP protocol requests
type HTTPClient struct {
	client *http.Client
}

// NewHTTPClient creates a new HTTP client with the specified configuration
func NewHTTPClient(cfg common.ConnectionPoolConfig) (*HTTPClient, error) {
	// Create transport with connection pooling
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.DialTimeout,
			KeepAlive: cfg.KeepAlive,
		}).DialContext,
		MaxIdleConns:        cfg.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.MaxConnsPerHost,
		IdleConnTimeout:     cfg.IdleConnTimeout,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // For testing purposes
		DisableCompression:  true,                                  // Handle compression manually
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second, // Default timeout
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	return &HTTPClient{
		client: client,
	}, nil
}

// Execute performs an HTTP request based on the given step configuration
func (c *HTTPClient) Execute(ctx context.Context, step common.RequestStep) (*HTTPResponse, error) {
	// Create request body if present
	var bodyReader io.Reader
	if len(step.Body) > 0 {
		bodyReader = bytes.NewReader(step.Body)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, step.Method, step.Path, bodyReader)
	if err != nil {
		return nil, err
	}

	// Add headers
	for key, value := range step.Headers {
		req.Header.Set(key, value)
	}

	// Set default headers if not present
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "goload/1.0")
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "*/*")
	}
	if len(step.Body) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Track request size
	var requestSize int64
	if bodyReader != nil {
		if buffered, ok := bodyReader.(*bytes.Reader); ok {
			requestSize = int64(buffered.Len())
		}
	}

	// Add headers size
	for key, values := range req.Header {
		for _, value := range values {
			requestSize += int64(len(key) + len(value) + 4) // 4 for ": " and "\r\n"
		}
	}

	// Execute request
	start := time.Now()
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Calculate response size
	responseSize := int64(len(body))
	for key, values := range resp.Header {
		for _, value := range values {
			responseSize += int64(len(key) + len(value) + 4)
		}
	}

	return &HTTPResponse{
		StatusCode:    resp.StatusCode,
		BytesReceived: responseSize,
		BytesSent:     requestSize,
		Headers:       resp.Header,
		Body:          body,
		Duration:      time.Since(start),
	}, nil
}

// Close closes any idle connections in the client's connection pool
func (c *HTTPClient) Close() {
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// SetTimeout sets the client timeout
func (c *HTTPClient) SetTimeout(timeout time.Duration) {
	c.client.Timeout = timeout
}

// SetTLSConfig sets the TLS configuration for the client
func (c *HTTPClient) SetTLSConfig(config *tls.Config) {
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.TLSClientConfig = config
	}
}

// SetProxy sets the proxy configuration for the client
func (c *HTTPClient) SetProxy(proxyURL string) error {
	proxyFunc := func(_ *http.Request) (*url.URL, error) {
		return url.Parse(proxyURL)
	}

	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.Proxy = proxyFunc
	}

	return nil
}

// SetKeepAlive sets the keep-alive duration for the client
func (c *HTTPClient) SetKeepAlive(keepAlive time.Duration) {
	if transport, ok := c.client.Transport.(*http.Transport); ok {
		transport.DialContext = (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: keepAlive,
		}).DialContext
	}
}

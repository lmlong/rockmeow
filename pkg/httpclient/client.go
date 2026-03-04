// Package httpclient provides shared HTTP clients with different timeout configurations.
// This package implements a singleton pattern to reuse HTTP clients across the application,
// improving resource efficiency and connection pooling.
package httpclient

import (
	"net/http"
	"sync"
	"time"
)

// Default timeout values (used when no config is provided)
const (
	defaultHTTPDefault   = 30 * time.Second
	defaultHTTPLong      = 60 * time.Second
	defaultHTTPExtraLong = 120 * time.Second
	// MaxIdleConns is the maximum number of idle connections across all hosts
	MaxIdleConns = 100
	// MaxIdleConnsPerHost is the maximum number of idle connections per host
	MaxIdleConnsPerHost = 10
	// IdleConnTimeout is how long an idle connection remains open
	IdleConnTimeout = 90 * time.Second
)

// Config holds the timeout configuration for HTTP clients.
type Config struct {
	HTTPDefault   time.Duration // Default HTTP timeout
	HTTPLong      time.Duration // Long HTTP timeout
	HTTPExtraLong time.Duration // Extra long HTTP timeout
}

// ClientPool holds shared HTTP clients with different timeout configurations.
type ClientPool struct {
	defaultClient     *http.Client
	longTimeoutClient *http.Client
	extraLongClient   *http.Client
	transport         *http.Transport
	config            *Config
}

var (
	pool     *ClientPool
	poolOnce sync.Once
	poolMu   sync.RWMutex
	cfg      *Config
)

// initTransport creates a shared transport with connection pooling settings.
func initTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        MaxIdleConns,
		MaxIdleConnsPerHost: MaxIdleConnsPerHost,
		IdleConnTimeout:     IdleConnTimeout,
		// Enable HTTP/2
		ForceAttemptHTTP2: true,
		// Reasonable timeouts for connection establishment
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// initClient creates an HTTP client with the given timeout and shared transport.
func initClient(transport *http.Transport, timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

// Init initializes the HTTP client pool with custom configuration.
// This should be called before any other functions in this package.
// If not called, default values will be used.
func Init(c *Config) {
	if c == nil {
		return
	}
	poolMu.Lock()
	cfg = c
	poolMu.Unlock()
}

// GetPool returns the singleton ClientPool instance.
// It initializes the pool on first call.
func GetPool() *ClientPool {
	poolOnce.Do(func() {
		poolMu.RLock()
		config := cfg
		poolMu.RUnlock()

		if config == nil {
			config = &Config{
				HTTPDefault:   defaultHTTPDefault,
				HTTPLong:      defaultHTTPLong,
				HTTPExtraLong: defaultHTTPExtraLong,
			}
		}

		transport := initTransport()
		pool = &ClientPool{
			transport:         transport,
			defaultClient:     initClient(transport, config.HTTPDefault),
			longTimeoutClient: initClient(transport, config.HTTPLong),
			extraLongClient:   initClient(transport, config.HTTPExtraLong),
			config:            config,
		}
	})
	return pool
}

// Default returns the default HTTP client (30s timeout by default).
// Use this for most API calls and standard operations.
func Default() *http.Client {
	return GetPool().defaultClient
}

// LongTimeout returns an HTTP client with a longer timeout (60s by default).
// Use this for operations that may take longer, such as LLM streaming or file uploads.
func LongTimeout() *http.Client {
	return GetPool().longTimeoutClient
}

// ExtraLongTimeout returns an HTTP client with an extra long timeout (120s by default).
// Use this for heavy operations like AIGC (image/video generation).
func ExtraLongTimeout() *http.Client {
	return GetPool().extraLongClient
}

// WithTimeout returns a new HTTP client with a custom timeout.
// This client shares the same transport for connection pooling.
// Note: This creates a new client instance each time, so use sparingly.
func WithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: GetPool().transport,
		Timeout:   timeout,
	}
}

// WithCustomTimeout returns a new HTTP client with a custom timeout and no ResponseHeaderTimeout.
// Use this for long-running operations like OpenCode where the server may take a long time to respond.
func WithCustomTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:          MaxIdleConns,
			MaxIdleConnsPerHost:   MaxIdleConnsPerHost,
			IdleConnTimeout:       IdleConnTimeout,
			ForceAttemptHTTP2:     true,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 0, // No header timeout for long-running requests
			ExpectContinueTimeout: 1 * time.Second,
		},
		Timeout: timeout,
	}
}

// GetTransport returns the shared transport for custom client configurations.
func GetTransport() *http.Transport {
	return GetPool().transport
}

// CustomClient creates a new HTTP client with custom configuration.
// It uses the shared transport for connection pooling.
func CustomClient(timeout time.Duration, customize func(*http.Client)) *http.Client {
	client := &http.Client{
		Transport: GetPool().transport,
		Timeout:   timeout,
	}
	if customize != nil {
		customize(client)
	}
	return client
}

// Package client demonstrates an idiomatic functional-options constructor.
package client

import "time"

// Client holds configuration for one outbound service client.
type Client struct {
	endpoint string
	timeout  time.Duration
}

// Option configures a Client during construction.
type Option func(*Client)

// WithEndpoint configures the client endpoint.
func WithEndpoint(endpoint string) Option {
	return func(client *Client) {
		client.endpoint = endpoint
	}
}

// WithTimeout configures the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(client *Client) {
		client.timeout = timeout
	}
}

// New creates a Client with conservative defaults.
func New(options ...Option) *Client {
	client := &Client{
		endpoint: "https://api.example.test",
		timeout:  3 * time.Second,
	}
	for _, option := range options {
		option(client)
	}
	return client
}

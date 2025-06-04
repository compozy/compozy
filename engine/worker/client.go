package worker

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// -----------------------------------------------------------------------------
// Client
// -----------------------------------------------------------------------------

type TemporalConfig struct {
	HostPort  string
	Namespace string
	TaskQueue string
}

type Client struct {
	client.Client
	config *TemporalConfig
}

func NewClient(cfg *TemporalConfig) (*Client, error) {
	options := client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
		Logger:    logger.GetDefault(),
	}
	temporalClient, err := client.Dial(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporal client: %w", err)
	}
	return &Client{
		Client: temporalClient,
		config: cfg,
	}, nil
}

func (c *Client) Config() *TemporalConfig {
	return c.config
}

func (c *Client) NewWorker(taskQueue string) worker.Worker {
	return worker.New(c.Client, taskQueue, worker.Options{})
}

func (c *Client) Close() {
	c.Client.Close()
}

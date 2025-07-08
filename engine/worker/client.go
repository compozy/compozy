package worker

import (
	"context"
	"fmt"
	"time"

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

func NewClient(ctx context.Context, cfg *TemporalConfig) (*Client, error) {
	log := logger.FromContext(ctx)
	options := client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
		Logger:    log,
	}
	dialStart := time.Now()
	temporalClient, err := client.Dial(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporal client: %w", err)
	}
	log.Debug("Temporal client connected", "duration", time.Since(dialStart))
	return &Client{
		Client: temporalClient,
		config: cfg,
	}, nil
}

func (c *Client) Config() *TemporalConfig {
	return c.config
}

func (c *Client) NewWorker(taskQueue string, options *worker.Options) worker.Worker {
	if options == nil {
		return worker.New(c.Client, taskQueue, worker.Options{})
	}
	return worker.New(c.Client, taskQueue, *options)
}

func (c *Client) Close() {
	c.Client.Close()
}

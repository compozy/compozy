package temporal

import (
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// -----------------------------------------------------------------------------
// Client
// -----------------------------------------------------------------------------

type Config struct {
	HostPort  string
	Namespace string
	TaskQueue string
}

type Client struct {
	client.Client
	config *Config
}

func New(cfg *Config) (*Client, error) {
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

func (c *Client) Config() *Config {
	return c.config
}

func (c *Client) NewWorker(taskQueue string) worker.Worker {
	return worker.New(c.Client, taskQueue, worker.Options{})
}

func (c *Client) Close() {
	c.Client.Close()
}

func (c *Client) RegisterWorker(w worker.Worker, activities *Activities) {
	w.RegisterWorkflow(CompozyWorkflow)
	w.RegisterActivity(activities.TriggerWorkflow)
	w.RegisterActivity(activities.UpdateWorkflowState)
}

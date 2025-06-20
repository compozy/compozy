package worker

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/sethvargo/go-retry"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"google.golang.org/protobuf/types/known/durationpb"
)

// randSource provides a properly seeded random source for jitter
// #nosec G404 - jitter doesn't need crypto-grade randomness
var randSource = rand.New(rand.NewSource(time.Now().UnixNano()))

// -----------------------------------------------------------------------------
// Client
// -----------------------------------------------------------------------------

type TemporalConfig struct {
	HostPort               string        `json:"host_port"`
	Namespace              string        `json:"namespace"`
	ConnectTimeout         time.Duration `json:"connect_timeout"`
	RequestTimeout         time.Duration `json:"request_timeout"`
	RetryAttempts          int           `json:"retry_attempts"`
	RetryDelayStart        time.Duration `json:"retry_delay_start"`
	RetryDelayMax          time.Duration `json:"retry_delay_max"`
	NamespaceReadyTimeout  time.Duration `json:"namespace_ready_timeout"`
	NamespaceCheckInterval time.Duration `json:"namespace_check_interval"`
}

// Validate checks if the configuration is valid
func (c *TemporalConfig) Validate() error {
	if c.HostPort == "" {
		return errors.New("host_port cannot be empty")
	}
	if c.ConnectTimeout <= 0 {
		return errors.New("connect_timeout must be positive")
	}
	if c.RequestTimeout <= 0 {
		return errors.New("request_timeout must be positive")
	}
	if c.RetryAttempts < 1 {
		return errors.New("retry_attempts must be at least 1")
	}
	if c.RetryDelayStart <= 0 {
		return errors.New("retry_delay_start must be positive")
	}
	if c.RetryDelayMax <= 0 {
		return errors.New("retry_delay_max must be positive")
	}
	if c.RetryDelayMax < c.RetryDelayStart {
		return errors.New("retry_delay_max must be >= retry_delay_start")
	}
	if c.NamespaceReadyTimeout <= 0 {
		return errors.New("namespace_ready_timeout must be positive")
	}
	if c.NamespaceCheckInterval <= 0 {
		return errors.New("namespace_check_interval must be positive")
	}
	return nil
}

type Client struct {
	client.Client
	nsClient client.NamespaceClient
	config   *TemporalConfig
}

func NewClient(ctx context.Context, cfg *TemporalConfig) (*Client, error) {
	log := logger.FromContext(ctx)
	if cfg == nil {
		cfg = DefaultTemporalConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid temporal config: %w", err)
	}
	// Create a context with timeout for the connection
	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	options := client.Options{
		HostPort:  cfg.HostPort,
		Namespace: cfg.Namespace,
		Logger:    log,
	}
	dialStart := time.Now()
	// Create client with timeout
	var temporalClient client.Client
	var nsClient client.NamespaceClient
	var dialErr error
	dialDone := make(chan struct{})
	go func() {
		defer close(dialDone)
		temporalClient, dialErr = client.Dial(options)
		if dialErr != nil {
			return
		}
		nsClient, dialErr = client.NewNamespaceClient(client.Options{
			HostPort: cfg.HostPort,
		})
		if dialErr != nil && temporalClient != nil {
			temporalClient.Close()
		}
	}()
	select {
	case <-connectCtx.Done():
		return nil, fmt.Errorf("connection timeout after %v", cfg.ConnectTimeout)
	case <-dialDone:
		if dialErr != nil {
			return nil, fmt.Errorf("failed to create temporal client: %w", dialErr)
		}
	}
	log.Debug("Temporal client connected", "duration", time.Since(dialStart))
	return &Client{
		Client:   temporalClient,
		nsClient: nsClient,
		config:   cfg,
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
	if c.nsClient != nil {
		c.nsClient.Close()
	}
	c.Client.Close()
}

// DefaultTemporalConfig returns the default Temporal configuration
func DefaultTemporalConfig() *TemporalConfig {
	return &TemporalConfig{
		HostPort:               "localhost:7233",
		Namespace:              "default",
		ConnectTimeout:         10 * time.Second,
		RequestTimeout:         30 * time.Second,
		RetryAttempts:          3,
		RetryDelayStart:        500 * time.Millisecond,
		RetryDelayMax:          5 * time.Second,
		NamespaceReadyTimeout:  30 * time.Second,
		NamespaceCheckInterval: 1 * time.Second,
	}
}

// ProvisionNamespace creates a new Temporal namespace
func (c *Client) ProvisionNamespace(ctx context.Context, namespace string) error {
	log := logger.FromContext(ctx)
	log.With("namespace", namespace).Info("Creating Temporal namespace")
	// Use retry with exponential backoff, cap, and jitter
	backoff := retry.NewExponential(c.config.RetryDelayStart)
	backoff = retry.WithCappedDuration(c.config.RetryDelayMax, backoff)     // Cap max delay
	backoff = retry.WithJitter(100*time.Millisecond, backoff)               // Add jitter
	backoff = retry.WithMaxRetries(uint64(c.config.RetryAttempts), backoff) // #nosec G115
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		reqCtx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
		defer cancel()
		log.With("namespace", namespace).Debug("Attempting to register Temporal namespace")
		err := c.nsClient.Register(reqCtx, &workflowservice.RegisterNamespaceRequest{
			Namespace:                        namespace,
			Description:                      fmt.Sprintf("Organization namespace: %s", namespace),
			WorkflowExecutionRetentionPeriod: durationpb.New(7 * 24 * time.Hour),
			Data: map[string]string{
				"created_by": "compozy_organization_service",
				"created_at": time.Now().UTC().Format(time.RFC3339),
			},
		})
		if err != nil {
			// Handle AlreadyExists as success (idempotent operation)
			var alreadyExists *serviceerror.NamespaceAlreadyExists
			if errors.As(err, &alreadyExists) {
				log.With("namespace", namespace).Info("Namespace already exists, treating as success")
				return nil
			}
			// Check if error is retryable
			var unavailable *serviceerror.Unavailable
			if errors.As(err, &unavailable) {
				log.With("namespace", namespace, "error", err).Warn("Service unavailable, will retry")
				return retry.RetryableError(err)
			}
			// Non-retryable error
			return err
		}
		return nil
	},
	)
	if err != nil {
		return fmt.Errorf("failed to register namespace '%s': %w", namespace, err)
	}
	log.With("namespace", namespace).Info("Temporal namespace created successfully")
	return c.waitForNamespaceReady(ctx, namespace)
}

// NamespaceExists checks if a namespace exists
func (c *Client) NamespaceExists(ctx context.Context, namespace string) (bool, error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.config.RequestTimeout)
	defer cancel()
	_, err := c.nsClient.Describe(reqCtx, namespace)
	if err != nil {
		var notFound *serviceerror.NotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to describe namespace '%s': %w", namespace, err)
	}
	return true, nil
}

// waitForNamespaceReady waits for the namespace to be ready for use
func (c *Client) waitForNamespaceReady(ctx context.Context, namespace string) error {
	log := logger.FromContext(ctx)
	timeoutCtx, cancel := context.WithTimeout(ctx, c.config.NamespaceReadyTimeout)
	defer cancel()
	var lastErr error
	for {
		select {
		case <-timeoutCtx.Done():
			if lastErr != nil {
				return fmt.Errorf("timeout waiting for namespace '%s' to be ready: %w", namespace, lastErr)
			}
			return fmt.Errorf("timeout waiting for namespace '%s' to be ready", namespace)
		default:
			exists, err := c.NamespaceExists(timeoutCtx, namespace)
			if err != nil {
				lastErr = err
				log.With("namespace", namespace, "error", err).Debug("Error checking namespace readiness")
			} else if exists {
				log.With("namespace", namespace).Info("Namespace is ready")
				return nil
			}
			// Sleep with jitter to prevent thundering herd
			// #nosec G404 - jitter doesn't need crypto-grade randomness
			jitter := time.Duration(randSource.Intn(200)) * time.Millisecond
			interval := c.config.NamespaceCheckInterval + jitter
			select {
			case <-timeoutCtx.Done():
				// Context canceled during sleep
				continue
			case <-time.After(interval):
				// Continue to next iteration
			}
		}
	}
}

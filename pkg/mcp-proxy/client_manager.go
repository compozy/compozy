package mcpproxy

import (
	"context"
	"fmt"
	"maps"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/compozy/compozy/pkg/logger"
)

// MCPClientManager manages multiple MCP client connections
type MCPClientManager struct {
	storage Storage
	clients map[string]*MCPClient
	mu      sync.RWMutex
	// Background context for lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	// Configuration
	config *ClientManagerConfig
	// Track ongoing reconnections to prevent concurrent attempts
	reconnecting map[string]bool
	reconnectMu  sync.Mutex
}

// ClientManagerConfig holds configuration for the client manager
type ClientManagerConfig struct {
	// Health check interval for all clients
	HealthCheckInterval time.Duration

	// Default reconnection settings
	DefaultReconnectDelay time.Duration
	DefaultMaxReconnects  int
	DefaultConnectTimeout time.Duration
	DefaultRequestTimeout time.Duration

	// Maximum number of concurrent connections
	MaxConcurrentConnections int

	// Maximum number of concurrent health checks
	HealthCheckParallelism int
}

// DefaultClientManagerConfig returns default configuration
func DefaultClientManagerConfig() *ClientManagerConfig {
	return &ClientManagerConfig{
		HealthCheckInterval:      30 * time.Second,
		DefaultReconnectDelay:    5 * time.Second,
		DefaultMaxReconnects:     5,
		DefaultConnectTimeout:    10 * time.Second,
		DefaultRequestTimeout:    30 * time.Second,
		MaxConcurrentConnections: 100,
		HealthCheckParallelism:   8,
	}
}

// NewMCPClientManager creates a new MCP client manager
func NewMCPClientManager(storage Storage, config *ClientManagerConfig) *MCPClientManager {
	if config == nil {
		config = DefaultClientManagerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &MCPClientManager{
		storage:      storage,
		clients:      make(map[string]*MCPClient),
		ctx:          ctx,
		cancel:       cancel,
		config:       config,
		reconnecting: make(map[string]bool),
	}
}

// Start starts the client manager and begins monitoring existing definitions
func (m *MCPClientManager) Start(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("Starting MCP client manager")
	// Replace internal context with a cancelable derivative that preserves values
	base := context.WithoutCancel(ctx)
	// Cancel any previous background context to avoid leaks
	if m.cancel != nil {
		m.cancel()
	}
	mctx, cancel := context.WithCancel(base)
	m.ctx = logger.ContextWithLogger(mctx, log)
	m.cancel = cancel

	// Load existing definitions and start clients
	definitions, err := m.storage.ListMCPs(ctx)
	if err != nil {
		return fmt.Errorf("failed to load MCP definitions: %w", err)
	}

	// Use errgroup to start clients concurrently for faster startup
	// This improves startup time when multiple MCP servers need to be connected
	g, groupCtx := errgroup.WithContext(ctx)
	for _, def := range definitions {
		// capture loop variable for closure
		g.Go(func() error {
			if err := m.AddClient(groupCtx, def); err != nil {
				log.Error("Failed to add client during startup", "name", def.Name, "error", err)
				return fmt.Errorf("failed to add client '%s': %w", def.Name, err)
			}
			return nil
		})
	}

	// Wait for all clients to start or fail
	if err := g.Wait(); err != nil {
		return fmt.Errorf("failed to start some MCP clients: %w", err)
	}

	// Start background health monitoring
	m.wg.Add(1)
	go m.healthMonitor()

	log.Info("MCP client manager started", "clients", len(m.clients))
	return nil
}

// Stop stops the client manager and all active clients
func (m *MCPClientManager) Stop(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("Stopping MCP client manager")

	// Cancel background operations
	m.cancel()

	// Disconnect all clients concurrently using errgroup
	m.mu.Lock()
	clients := make(map[string]*MCPClient)
	maps.Copy(clients, m.clients)
	m.mu.Unlock()

	// Use errgroup for concurrent disconnection
	g, groupCtx := errgroup.WithContext(ctx)
	for name, client := range clients {
		name, client := name, client // capture loop variables
		g.Go(func() error {
			if err := client.Disconnect(groupCtx); err != nil {
				log.Error("Failed to disconnect client", "name", name, "error", err)
				return fmt.Errorf("failed to disconnect client '%s': %w", name, err)
			}
			return nil
		})
	}

	// Wait for all disconnections to complete
	if err := g.Wait(); err != nil {
		log.Warn("Some clients failed to disconnect cleanly", "error", err)
	}

	// Wait for background goroutines to finish
	m.wg.Wait()

	log.Info("MCP client manager stopped")
	return nil
}

// AddClient adds a new MCP client based on the definition
func (m *MCPClientManager) AddClient(ctx context.Context, def *MCPDefinition) error {
	log := logger.FromContext(ctx)
	if def == nil {
		return fmt.Errorf("definition cannot be nil")
	}

	if err := def.Validate(); err != nil {
		return fmt.Errorf("invalid definition: %w", err)
	}

	// Create client outside the critical section to avoid blocking
	client, err := m.createClient(def)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Only hold lock for map operations
	m.mu.Lock()
	// Check if client already exists
	if _, exists := m.clients[def.Name]; exists {
		m.mu.Unlock()
		// Clean up the created client since we're not using it
		if disconnectErr := client.Disconnect(ctx); disconnectErr != nil {
			log.Debug("Failed to clean up unused client", "name", def.Name, "error", disconnectErr)
		}
		return fmt.Errorf("client '%s' already exists", def.Name)
	}

	// Check connection limit
	if len(m.clients) >= m.config.MaxConcurrentConnections {
		m.mu.Unlock()
		// Clean up the created client since we can't add it
		if disconnectErr := client.Disconnect(ctx); disconnectErr != nil {
			log.Debug("Failed to clean up client due to connection limit", "name", def.Name, "error", disconnectErr)
		}
		return fmt.Errorf("maximum concurrent connections (%d) reached", m.config.MaxConcurrentConnections)
	}

	// Add to map
	m.clients[def.Name] = client
	m.mu.Unlock()

	// Start connection in background
	m.wg.Go(func() {
		m.connectClient(m.ctx, client)
	})

	log.Debug("Added MCP client", "name", def.Name, "transport", def.Transport)
	return nil
}

// RemoveClient removes and disconnects an MCP client
func (m *MCPClientManager) RemoveClient(ctx context.Context, name string) error {
	log := logger.FromContext(ctx)
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[name]
	if !exists {
		return fmt.Errorf("client '%s' not found", name)
	}

	// Disconnect the client
	if err := client.Disconnect(ctx); err != nil {
		log.Error("Failed to disconnect client", "name", name, "error", err)
	}

	// Remove from map
	delete(m.clients, name)

	log.Debug("Removed MCP client", "name", name)
	return nil
}

// GetClient returns an MCP client by name
func (m *MCPClientManager) GetClient(name string) (MCPClientInterface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, exists := m.clients[name]
	if !exists {
		return nil, fmt.Errorf("client '%s' not found", name)
	}

	return client, nil
}

// ListClientStatuses returns status copies for all clients using concurrent retrieval
func (m *MCPClientManager) ListClientStatuses(ctx context.Context) map[string]*MCPStatus {
	log := logger.FromContext(ctx)
	m.mu.RLock()
	clients := make(map[string]*MCPClient)
	maps.Copy(clients, m.clients)
	m.mu.RUnlock()

	if len(clients) == 0 {
		return make(map[string]*MCPStatus)
	}

	// Use errgroup for concurrent status retrieval
	g := &errgroup.Group{}
	statuses := make(map[string]*MCPStatus)
	statusesMu := sync.Mutex{}

	for name, client := range clients {
		name, client := name, client // capture loop variables
		g.Go(func() error {
			status := client.GetStatus()
			statusesMu.Lock()
			statuses[name] = status
			statusesMu.Unlock()
			return nil
		})
	}

	// Wait for all status retrievals to complete
	if err := g.Wait(); err != nil {
		log.Warn("Error during concurrent status retrieval", "error", err)
	}

	return statuses
}

// GetClientStatus returns the status of a specific client
func (m *MCPClientManager) GetClientStatus(name string) (*MCPStatus, error) {
	client, err := m.GetClient(name)
	if err != nil {
		return nil, err
	}

	return client.GetStatus(), nil
}

// ReloadClient reloads a client with updated definition
func (m *MCPClientManager) ReloadClient(ctx context.Context, def *MCPDefinition) error {
	if def == nil {
		return fmt.Errorf("definition cannot be nil")
	}

	// Remove existing client if it exists
	if _, err := m.GetClient(def.Name); err == nil {
		if err := m.RemoveClient(ctx, def.Name); err != nil {
			return fmt.Errorf("failed to remove existing client: %w", err)
		}
	}

	// Add the new client
	return m.AddClient(ctx, def)
}

// createClient creates a new client instance based on transport type
func (m *MCPClientManager) createClient(def *MCPDefinition) (*MCPClient, error) {
	return NewMCPClient(m.ctx, def, m.storage, m.config)
}

// connectClient attempts to connect a client with retry logic
func (m *MCPClientManager) connectClient(ctx context.Context, client *MCPClient) {
	log := logger.FromContext(ctx)
	def := client.GetDefinition()
	status := client.GetStatus()

	maxRetries := def.MaxReconnects
	if maxRetries == 0 {
		maxRetries = m.config.DefaultMaxReconnects
	}

	reconnectDelay := def.ReconnectDelay
	if reconnectDelay == 0 {
		reconnectDelay = m.config.DefaultReconnectDelay
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return
		case <-m.ctx.Done():
			return
		default:
		}

		// Update status to connecting
		status.UpdateStatus(StatusConnecting, "")
		m.saveStatus(ctx, status)

		// Attempt connection with timeout; prefer per-definition Timeout when provided
		timeout := m.config.DefaultConnectTimeout
		if def.Timeout > 0 {
			timeout = def.Timeout
		}
		// Derive from the provided context to honor upstream cancellation
		connectCtx, cancel := context.WithTimeout(ctx, timeout)
		err := client.Connect(connectCtx)
		cancel()

		if err == nil {
			// Success
			status.UpdateStatus(StatusConnected, "")
			m.saveStatus(ctx, status)
			log.Debug("MCP client connected", "name", def.Name, "attempt", attempt+1)
			return
		}

		// Failed - update status and potentially retry
		status.UpdateStatus(StatusError, err.Error())
		m.saveStatus(ctx, status)

		log.Warn("MCP client connection failed",
			"name", def.Name,
			"attempt", attempt+1,
			"maxRetries", maxRetries+1,
			"error", err)

		// Don't sleep after last attempt
		if attempt < maxRetries {
			// Exponential backoff with jitter to prevent thundering herd
			backoffDelay := min(time.Duration(float64(reconnectDelay)*(1.5*float64(attempt)+1)),
				// Cap at 60 seconds
				MaxBackoffDelay)

			select {
			case <-time.After(backoffDelay):
			case <-ctx.Done():
				return
			case <-m.ctx.Done():
				return
			}
		}
	}

	// All attempts failed
	status.UpdateStatus(StatusError, "maximum connection attempts exceeded")
	m.saveStatus(ctx, status)
	log.Error("MCP client connection failed permanently", "name", def.Name)
}

// healthMonitor runs periodic health checks on all clients
func (m *MCPClientManager) healthMonitor() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performHealthChecks()
		case <-m.ctx.Done():
			return
		}
	}
}

// performHealthChecks checks the health of all connected clients concurrently
func (m *MCPClientManager) performHealthChecks() {
	log := logger.FromContext(m.ctx)
	// Get client list outside of individual health checks to avoid long lock
	m.mu.RLock()
	clientsCopy := make(map[string]*MCPClient)
	maps.Copy(clientsCopy, m.clients)
	m.mu.RUnlock()

	// Filter clients that need health checks
	clientsToCheck := make(map[string]*MCPClient)
	for name, client := range clientsCopy {
		if !client.IsConnected() {
			continue
		}

		def := client.GetDefinition()
		if !def.HealthCheckEnabled {
			continue
		}

		clientsToCheck[name] = client
	}

	if len(clientsToCheck) == 0 {
		return
	}

	// Use errgroup for concurrent health checks with bounded parallelism
	g, ctx := errgroup.WithContext(m.ctx)

	// Create semaphore for bounded concurrency (ensure at least 1 to avoid deadlock)
	parallelism := m.config.HealthCheckParallelism
	if parallelism < 1 {
		parallelism = 1
	}
	semaphore := make(chan struct{}, parallelism)

	for name, client := range clientsToCheck {
		// Capture loop variables
		name, client := name, client

		g.Go(func() error {
			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return ctx.Err()
			}

			// Perform health check with timeout
			healthCtx, cancel := context.WithTimeout(ctx, AdminHealthCheckTimeout)
			defer cancel()

			err := client.Health(healthCtx)
			status := client.GetStatus()

			if err != nil {
				log.Warn("MCP client health check failed", "name", name, "error", err)
				status.UpdateStatus(StatusError, fmt.Sprintf("health check failed: %v", err))

				// Trigger reconnection if auto-reconnect is enabled
				def := client.GetDefinition()
				if def.AutoReconnect {
					m.triggerReconnection(client)
				}
			} else if status.Status != StatusConnected {
				// Health check passed - ensure status is connected
				status.UpdateStatus(StatusConnected, "")
			}

			m.saveStatus(ctx, status)
			return nil // Don't propagate health check errors as fatal
		})
	}

	// Wait for all health checks to complete
	if err := g.Wait(); err != nil {
		log.Error("Health check process interrupted", "error", err)
	}
}

// triggerReconnection safely triggers reconnection for a client, preventing concurrent attempts
func (m *MCPClientManager) triggerReconnection(client *MCPClient) {
	log := logger.FromContext(m.ctx)
	name := client.GetDefinition().Name

	m.reconnectMu.Lock()
	if m.reconnecting[name] {
		m.reconnectMu.Unlock()
		log.Debug("Reconnection already in progress", "name", name)
		return // Already reconnecting
	}
	m.reconnecting[name] = true
	m.reconnectMu.Unlock()

	m.wg.Go(func() {
		defer func() {
			m.reconnectMu.Lock()
			delete(m.reconnecting, name)
			m.reconnectMu.Unlock()
		}()

		log.Info("Starting automatic reconnection", "name", name)
		m.connectClient(m.ctx, client)
	})
}

// saveStatus saves a client status to storage
func (m *MCPClientManager) saveStatus(ctx context.Context, status *MCPStatus) {
	log := logger.FromContext(ctx)
	if err := m.storage.SaveStatus(ctx, status); err != nil {
		log.Error("Failed to save client status", "name", status.Name, "error", err)
	}
}

// GetMetrics returns overall metrics for the client manager
func (m *MCPClientManager) GetMetrics() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var connected, connecting, errored int
	var totalRequests, totalErrors int64

	for _, client := range m.clients {
		status := client.GetStatus()
		switch status.Status {
		case StatusConnected:
			connected++
		case StatusConnecting:
			connecting++
		case StatusError:
			errored++
		}
		totalRequests += status.TotalRequests
		totalErrors += status.TotalErrors
	}

	return map[string]any{
		"total_clients":   len(m.clients),
		"connected":       connected,
		"connecting":      connecting,
		"errored":         errored,
		"disconnected":    len(m.clients) - connected - connecting - errored,
		"total_requests":  totalRequests,
		"total_errors":    totalErrors,
		"max_connections": m.config.MaxConcurrentConnections,
	}
}

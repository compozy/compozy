package mcp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"golang.org/x/sync/errgroup"
)

// RegisterService manages MCP registration lifecycle with the proxy
type RegisterService struct {
	proxy *Client
	mux   sync.RWMutex
	regs  map[string]bool // id → registered
}

// New creates a new MCP registration service
func NewRegisterService(proxyClient *Client) *RegisterService {
	return &RegisterService{
		proxy: proxyClient,
		regs:  make(map[string]bool),
	}
}

// NewWithTimeout creates a service with a configured proxy client using default timeout
func NewWithTimeout(proxyURL, adminToken string, timeout time.Duration) *RegisterService {
	proxyClient := NewProxyClient(proxyURL, adminToken, timeout)
	return NewRegisterService(proxyClient)
}

// Ensure registers an MCP with the proxy if not already registered (idempotent)
func (s *RegisterService) Ensure(ctx context.Context, config *Config) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// Check if already registered
	if s.regs[config.ID] {
		logger.Debug("MCP already registered, skipping", "mcp_id", config.ID)
		return nil
	}

	// Convert MCP config to proxy definition
	def, err := s.convertToDefinition(config)
	if err != nil {
		return fmt.Errorf("failed to convert MCP config to definition: %w", err)
	}

	// Register with proxy
	if err := s.proxy.Register(ctx, &def); err != nil {
		return fmt.Errorf("failed to register MCP with proxy: %w", err)
	}

	// Mark as registered
	s.regs[config.ID] = true
	return nil
}

// Deregister removes an MCP from the proxy
func (s *RegisterService) Deregister(ctx context.Context, mcpID string) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// Check if registered
	if !s.regs[mcpID] {
		logger.Debug("MCP not registered, skipping deregistration", "mcp_id", mcpID)
		return nil
	}

	// Deregister from proxy
	if err := s.proxy.Deregister(ctx, mcpID); err != nil {
		return fmt.Errorf("failed to deregister MCP from proxy: %w", err)
	}

	// Remove from registry
	delete(s.regs, mcpID)

	logger.Info("Successfully deregistered MCP from proxy", "mcp_id", mcpID)
	return nil
}

// IsRegistered checks if an MCP is currently registered
func (s *RegisterService) IsRegistered(mcpID string) bool {
	s.mux.RLock()
	defer s.mux.RUnlock()
	return s.regs[mcpID]
}

// ListRegistered returns a list of all registered MCP IDs
func (s *RegisterService) ListRegistered() []string {
	s.mux.RLock()
	defer s.mux.RUnlock()

	var registered []string
	for mcpID := range s.regs {
		registered = append(registered, mcpID)
	}
	return registered
}

// Shutdown deregisters all MCPs and cleans up resources
func (s *RegisterService) Shutdown(ctx context.Context) error {
	// Collect MCP IDs under lock
	s.mux.Lock()
	mcpIDs := make([]string, 0, len(s.regs))
	for mcpID := range s.regs {
		mcpIDs = append(mcpIDs, mcpID)
	}
	s.mux.Unlock()

	logger.Info("Shutting down MCP register, deregistering all MCPs",
		"count", len(mcpIDs))

	if len(mcpIDs) == 0 {
		logger.Info("No MCPs to deregister during shutdown")
		return nil
	}

	// Use errgroup for concurrent deregistration
	g, gCtx := errgroup.WithContext(ctx)

	for _, mcpID := range mcpIDs {
		mcpID := mcpID // capture loop variable
		g.Go(func() error {
			if err := s.proxy.Deregister(gCtx, mcpID); err != nil {
				logger.Error("Failed to deregister MCP during shutdown",
					"mcp_id", mcpID, "error", err)
				return fmt.Errorf("failed to deregister %s: %w", mcpID, err)
			}
			logger.Debug("Deregistered MCP during shutdown", "mcp_id", mcpID)
			return nil
		})
	}

	// Wait for all deregistrations to complete
	err := g.Wait()

	// Clear the registry
	s.mux.Lock()
	s.regs = make(map[string]bool)
	s.mux.Unlock()

	if err != nil {
		return fmt.Errorf("shutdown failed: %w", err)
	}

	logger.Info("MCP register shutdown completed successfully")
	return nil
}

// HealthCheck verifies the proxy connection and registry state
func (s *RegisterService) HealthCheck(ctx context.Context) error {
	// Check proxy health
	if err := s.proxy.Health(ctx); err != nil {
		return fmt.Errorf("proxy health check failed: %w", err)
	}

	s.mux.RLock()
	registeredCount := len(s.regs)
	s.mux.RUnlock()

	logger.Debug("MCP register health check passed",
		"registered_mcps", registeredCount)

	return nil
}

// SyncWithProxy synchronizes the local registry with the proxy state
func (s *RegisterService) SyncWithProxy(ctx context.Context) error {
	// Get MCPs from proxy
	proxyMCPs, err := s.proxy.ListMCPs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list MCPs from proxy: %w", err)
	}

	s.mux.Lock()
	defer s.mux.Unlock()

	// Create map of proxy MCPs for comparison
	proxyMap := make(map[string]bool)
	for _, mcpDef := range proxyMCPs {
		proxyMap[mcpDef.Name] = true
	}

	// Find discrepancies
	var localOnly, proxyOnly []string

	// Check for MCPs registered locally but not in proxy
	for mcpID := range s.regs {
		if !proxyMap[mcpID] {
			localOnly = append(localOnly, mcpID)
		}
	}

	// Check for MCPs in proxy but not registered locally
	for mcpID := range proxyMap {
		if !s.regs[mcpID] {
			proxyOnly = append(proxyOnly, mcpID)
		}
	}

	// Log discrepancies
	if len(localOnly) > 0 {
		logger.Warn("MCPs registered locally but missing from proxy",
			"mcps", localOnly)
		// Remove from local registry
		for _, mcpID := range localOnly {
			delete(s.regs, mcpID)
		}
	}

	if len(proxyOnly) > 0 {
		logger.Info("MCPs found in proxy but not in local registry",
			"mcps", proxyOnly)
		// Add to local registry (assuming they're valid)
		for _, mcpID := range proxyOnly {
			s.regs[mcpID] = true
		}
	}

	logger.Debug("Registry synchronized with proxy",
		"total_registered", len(s.regs),
		"corrected_local_only", len(localOnly),
		"added_from_proxy", len(proxyOnly))

	return nil
}

// convertToDefinition converts an MCP config to a proxy definition
func (s *RegisterService) convertToDefinition(config *Config) (Definition, error) {
	def := Definition{
		Name:      config.ID,
		Transport: config.Transport,
		Env:       config.Env,
	}

	// Handle different MCP types based on available fields
	switch {
	case config.URL != "":
		// Remote MCP (SSE or streamable-http)
		def.URL = config.URL
	case config.Command != "":
		// Parse command with basic validation
		commandParts, err := parseCommand(config.Command)
		if err != nil {
			return def, fmt.Errorf("invalid command format: %w", err)
		}
		def.Command = commandParts[0]
		if len(commandParts) > 1 {
			def.Args = commandParts[1:]
		} else {
			def.Args = []string{}
		}
	default:
		return def, fmt.Errorf("MCP configuration must specify either URL (for remote) or Command (for stdio)")
	}

	// Validate the definition
	if def.Name == "" {
		return def, fmt.Errorf("MCP name is required")
	}
	if def.Transport == "" {
		return def, fmt.Errorf("MCP transport is required")
	}
	if def.URL == "" && def.Command == "" {
		return def, fmt.Errorf("MCP must have either URL or command specified")
	}

	return def, nil
}

// parseCommand safely parses a command string into command and arguments
// This is a basic implementation that handles simple cases and validates input
func parseCommand(command string) ([]string, error) {
	if command == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}

	// Trim whitespace
	command = strings.TrimSpace(command)

	// Basic validation - reject obviously malicious patterns
	if strings.Contains(command, "\n") || strings.Contains(command, "\r") {
		return nil, fmt.Errorf("command cannot contain newlines")
	}

	// Use simple splitting for now - this handles basic cases
	// Note: This doesn't handle quoted arguments with spaces properly
	// For production use, consider using shlex or similar library
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return nil, fmt.Errorf("command cannot be empty after parsing")
	}

	// Validate command name (basic check)
	if strings.HasPrefix(parts[0], "-") {
		return nil, fmt.Errorf("command name cannot start with dash")
	}

	return parts, nil
}

// EnsureMultiple registers multiple MCPs in parallel with error handling
func (s *RegisterService) EnsureMultiple(ctx context.Context, configs []Config) error {
	if len(configs) == 0 {
		return nil
	}

	logger.Info("Registering multiple MCPs with proxy", "count", len(configs))

	// Use a goroutine pool for concurrent registration
	type result struct {
		mcpID string
		err   error
	}

	results := make(chan result, len(configs))

	// Limit concurrent registrations to avoid overwhelming the proxy
	const maxConcurrent = 5
	semaphore := make(chan struct{}, maxConcurrent)

	// Start registration goroutines
	for _, config := range configs {
		go func(cfg Config) {
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			err := s.Ensure(ctx, &cfg)
			results <- result{mcpID: cfg.ID, err: err}
		}(config)
	}

	// Collect results
	var errs []error
	successCount := 0

	for i := 0; i < len(configs); i++ {
		select {
		case res := <-results:
			if res.err != nil {
				logger.Error("Failed to register MCP", "mcp_id", res.mcpID, "error", res.err)
				errs = append(errs, fmt.Errorf("MCP %s: %w", res.mcpID, res.err))
			} else {
				successCount++
			}
		case <-ctx.Done():
			return fmt.Errorf("registration canceled: %w", ctx.Err())
		}
	}

	logger.Info("MCP registration completed",
		"total", len(configs),
		"successful", successCount,
		"failed", len(errs))

	// Return combined error if any registrations failed
	if len(errs) > 0 {
		return fmt.Errorf("failed to register %d MCPs: %v", len(errs), errs)
	}

	return nil
}

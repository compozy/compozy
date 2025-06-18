package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/google/shlex"
	"golang.org/x/sync/errgroup"
)

// WorkflowConfig represents the minimal interface needed from workflow configs
type WorkflowConfig interface {
	GetMCPs() []Config
}

// RegisterService manages MCP registration lifecycle with the proxy
type RegisterService struct {
	proxy *Client
}

// New creates a new MCP registration service
func NewRegisterService(proxyClient *Client) *RegisterService {
	return &RegisterService{
		proxy: proxyClient,
	}
}

// NewWithTimeout creates a service with a configured proxy client using default timeout
func NewWithTimeout(proxyURL, adminToken string, timeout time.Duration) *RegisterService {
	proxyClient := NewProxyClient(proxyURL, adminToken, timeout)
	return NewRegisterService(proxyClient)
}

// Ensure registers an MCP with the proxy if not already registered (idempotent)
func (s *RegisterService) Ensure(ctx context.Context, config *Config) error {
	// Convert MCP config to proxy definition
	def, err := s.convertToDefinition(config)
	if err != nil {
		return fmt.Errorf("failed to convert MCP config to definition: %w", err)
	}

	// Register with proxy
	if err := s.proxy.Register(ctx, &def); err != nil {
		return fmt.Errorf("failed to register MCP with proxy: %w", err)
	}

	return nil
}

// Deregister removes an MCP from the proxy
func (s *RegisterService) Deregister(ctx context.Context, mcpID string) error {
	log := logger.FromContext(ctx)
	// Deregister from proxy
	if err := s.proxy.Deregister(ctx, mcpID); err != nil {
		return fmt.Errorf("failed to deregister MCP from proxy: %w", err)
	}

	log.Info("Successfully deregistered MCP from proxy", "mcp_id", mcpID)
	return nil
}

// IsRegistered checks if an MCP is currently registered with the proxy
func (s *RegisterService) IsRegistered(ctx context.Context, mcpID string) (bool, error) {
	mcps, err := s.proxy.ListMCPs(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check registration status: %w", err)
	}
	for _, mcp := range mcps {
		if mcp.Name == mcpID {
			return true, nil
		}
	}
	return false, nil
}

// ListRegistered returns a list of all registered MCP IDs from the proxy
func (s *RegisterService) ListRegistered(ctx context.Context) ([]string, error) {
	mcps, err := s.proxy.ListMCPs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list registered MCPs: %w", err)
	}
	var mcpIDs []string
	for _, mcp := range mcps {
		mcpIDs = append(mcpIDs, mcp.Name)
	}
	return mcpIDs, nil
}

// Shutdown deregisters all MCPs and cleans up resources
func (s *RegisterService) Shutdown(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Info("Shutting down MCP register, deregistering all MCPs")
	mcpIDs, err := s.ListRegistered(ctx)
	if err != nil {
		log.Error("Failed to get registered MCPs for shutdown", "error", err)
		return fmt.Errorf("failed to get registered MCPs: %w", err)
	}
	if len(mcpIDs) == 0 {
		log.Debug("No MCPs to deregister during shutdown")
		return nil
	}
	log.Debug("Found MCPs to deregister", "count", len(mcpIDs))
	// Use errgroup for concurrent deregistration
	g, gCtx := errgroup.WithContext(ctx)
	for _, mcpID := range mcpIDs {
		mcpID := mcpID // capture loop variable
		g.Go(func() error {
			if err := s.proxy.Deregister(gCtx, mcpID); err != nil {
				log.Error("Failed to deregister MCP during shutdown",
					"mcp_id", mcpID, "error", err)
				return fmt.Errorf("failed to deregister %s: %w", mcpID, err)
			}
			log.Debug("Deregistered MCP during shutdown", "mcp_id", mcpID)
			return nil
		})
	}
	// Wait for all deregistrations to complete
	err = g.Wait()
	if err != nil {
		return fmt.Errorf("shutdown failed: %w", err)
	}
	log.Info("MCP register shutdown completed successfully")
	return nil
}

// HealthCheck verifies the proxy connection
func (s *RegisterService) HealthCheck(ctx context.Context) error {
	log := logger.FromContext(ctx)
	// Check proxy health
	if err := s.proxy.Health(ctx); err != nil {
		return fmt.Errorf("proxy health check failed: %w", err)
	}
	log.Debug("MCP register health check passed")
	return nil
}

// SyncWithProxy is no longer needed since we always use proxy as source of truth
func (s *RegisterService) SyncWithProxy(ctx context.Context) error {
	log := logger.FromContext(ctx)
	log.Debug("Registry synchronized with proxy")
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
// Uses shell lexer to properly handle quoted arguments with spaces
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

	// Use shell lexer to properly parse quoted arguments
	parts, err := shlex.Split(command)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %w", err)
	}

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
	log := logger.FromContext(ctx)
	if len(configs) == 0 {
		return nil
	}
	log.Info("Registering multiple MCPs with proxy", "count", len(configs))
	// Use a worker pool for concurrent registration
	type result struct {
		mcpID string
		err   error
	}
	results := make(chan result, len(configs))
	// Limit concurrent registrations to avoid overwhelming the proxy
	const maxConcurrent = 5
	work := make(chan Config, len(configs))
	// Start fixed number of workers
	for i := 0; i < maxConcurrent; i++ {
		go func() {
			for cfg := range work {
				err := s.Ensure(ctx, &cfg)
				results <- result{mcpID: cfg.ID, err: err}
			}
		}()
	}
	// Send work to workers
	for _, config := range configs {
		work <- config
	}
	close(work)
	// Collect results
	var errs []error
	successCount := 0
	for range configs {
		select {
		case res := <-results:
			if res.err != nil {
				log.Error("Failed to register MCP", "mcp_id", res.mcpID, "error", res.err)
				errs = append(errs, fmt.Errorf("MCP %s: %w", res.mcpID, res.err))
			} else {
				successCount++
			}
		case <-ctx.Done():
			return fmt.Errorf("registration canceled: %w", ctx.Err())
		}
	}
	log.Info("MCP registration completed",
		"total", len(configs),
		"successful", successCount,
		"failed", len(errs))
	// Return combined error if any registrations failed
	if len(errs) > 0 {
		return fmt.Errorf("failed to register %d MCPs: %v", len(errs), errs)
	}
	return nil
}

// -----------------------------------------------------------------------------
// Setup and Initialization Functions
// -----------------------------------------------------------------------------

// SetupForWorkflows creates and initializes an MCP RegisterService for the given workflows
// Returns nil if MCP_PROXY_URL is not configured
func SetupForWorkflows(ctx context.Context, workflows []WorkflowConfig) (*RegisterService, error) {
	log := logger.FromContext(ctx)
	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		return nil, nil // No proxy configured
	}

	adminToken := os.Getenv("MCP_PROXY_ADMIN_TOKEN")
	timeout := 30 * time.Second
	service := NewWithTimeout(proxyURL, adminToken, timeout)
	log.Info("Initialized MCP register with proxy", "proxy_url", proxyURL)

	// Collect all MCPs from all workflows
	allMCPs := CollectWorkflowMCPs(workflows)
	if len(allMCPs) > 0 {
		log.Info("Starting async MCP registration", "mcp_count", len(allMCPs))
		// Register MCPs asynchronously to avoid blocking server startup
		go func() {
			// Use a fresh context with timeout for registration
			regCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			regLog := logger.FromContext(regCtx)
			if err := service.EnsureMultiple(regCtx, allMCPs); err != nil {
				regLog.Error("Failed to register some MCPs with proxy", "error", err)
			} else {
				regLog.Info("Successfully registered all MCPs", "count", len(allMCPs))
			}
		}()
	}

	return service, nil
}

// CollectWorkflowMCPs collects all unique MCP configurations from all workflows
func CollectWorkflowMCPs(workflows []WorkflowConfig) []Config {
	seen := make(map[string]bool)
	var allMCPs []Config

	for _, workflow := range workflows {
		for _, mcpConfig := range workflow.GetMCPs() {
			mcpConfig.SetDefaults() // Ensure defaults are applied
			// Use ID to deduplicate MCPs across workflows
			if !seen[mcpConfig.ID] {
				seen[mcpConfig.ID] = true
				allMCPs = append(allMCPs, mcpConfig)
			}
		}
	}

	return allMCPs
}

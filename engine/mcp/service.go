package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/pkg/tplengine"
	"github.com/google/shlex"
	"golang.org/x/sync/errgroup"
)

// WorkflowConfig represents the minimal interface needed from workflow configs
type WorkflowConfig interface {
	GetMCPs() []Config
	GetEnv() core.EnvMap
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
func NewWithTimeout(proxyURL string, timeout time.Duration) *RegisterService {
	proxyClient := NewProxyClient(proxyURL, timeout)
	return NewRegisterService(proxyClient)
}

// Ensure registers an MCP with the proxy if not already registered (idempotent)
func (s *RegisterService) Ensure(ctx context.Context, config *Config) error {
	log := logger.FromContext(ctx)
	// Convert MCP config to proxy definition
	def, err := s.convertToDefinition(config)
	if err != nil {
		return fmt.Errorf("failed to convert MCP config to definition: %w", err)
	}
	// Log registration with redacted headers for security
	if len(def.Headers) > 0 {
		log.Debug("Registering MCP with headers", "mcp_id", config.ID, "headers", redactHeaders(def.Headers))
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
		id := mcpID
		g.Go(func() error {
			if err := s.proxy.Deregister(gCtx, id); err != nil {
				log.Error("Failed to deregister MCP during shutdown",
					"mcp_id", id, "error", err)
				return fmt.Errorf("failed to deregister %s: %w", id, err)
			}
			log.Debug("Deregistered MCP during shutdown", "mcp_id", id)
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
		Headers:   normalizeHeaders(config.Headers),
		Timeout:   config.StartTimeout,
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

// normalizeHeaders returns a defensive copy of headers with case normalization.
// It ensures the Authorization header uses proper casing but does NOT infer schemes.
// Users must provide complete Authorization headers with the correct scheme.
func normalizeHeaders(h map[string]string) map[string]string {
	if len(h) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(h))
	var authValue string
	var haveAuth bool
	for k, v := range h {
		if strings.EqualFold(k, "authorization") {
			authValue = v
			haveAuth = true
			continue
		}
		out[k] = v
	}
	if haveAuth {
		out["Authorization"] = authValue
	}
	return out
}

// redactToken redacts sensitive parts of tokens for safe logging.
// Keeps first 6 and last 4 characters visible for debugging.
func redactToken(token string) string {
	if len(token) <= 10 {
		return "***REDACTED***"
	}
	return token[:6] + "..." + token[len(token)-4:]
}

// redactHeaders creates a copy of headers with redacted Authorization values for logging.
func redactHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return headers
	}
	redacted := make(map[string]string, len(headers))
	for k, v := range headers {
		if strings.EqualFold(k, "authorization") {
			// Redact the token part while keeping the scheme visible
			parts := strings.SplitN(v, " ", 2)
			if len(parts) == 2 {
				redacted[k] = parts[0] + " " + redactToken(parts[1])
			} else {
				redacted[k] = redactToken(v)
			}
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// resolveHeadersWithEnv evaluates header templates using the provided env (hierarchical env already merged by loader)
// It validates templates to prevent injection attacks.
func resolveHeadersWithEnv(headers map[string]string, env core.EnvMap) map[string]string {
	if len(headers) == 0 {
		return headers
	}
	out := make(map[string]string, len(headers))
	engine := tplengine.NewEngine(tplengine.FormatText)
	// Use a restricted context with only environment variables
	// The template engine already provides XSS protection via htmlEscape functions
	ctx := map[string]any{"env": env.AsMap()}
	for k, v := range headers {
		if tplengine.HasTemplate(v) {
			// Validate template doesn't contain dangerous patterns
			if err := validateTemplate(v); err != nil {
				// Skip suspicious templates and use original value
				out[k] = v
				continue
			}
			if s, err := engine.ProcessString(v, ctx); err == nil {
				out[k] = s
			} else {
				// Use original value if template processing fails
				out[k] = v
			}
		} else {
			out[k] = v
		}
	}
	return normalizeHeaders(out)
}

// validateTemplate checks for potentially dangerous template patterns
func validateTemplate(tmpl string) error {
	if strings.Count(tmpl, "{{") > 5 {
		return fmt.Errorf("template too complex: too many expressions")
	}
	return nil
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
	for i := range configs {
		work <- configs[i]
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
		return nil, nil
	}
	timeout := 30 * time.Second
	service := NewWithTimeout(proxyURL, timeout)
	log.Info("Initialized MCP register with proxy", "proxy_url", proxyURL)

	// Collect unique MCPs declared by all workflows
	allMCPs := CollectWorkflowMCPs(workflows)
	if len(allMCPs) == 0 {
		return service, nil
	}

	log.Info("Registering MCPs and waiting for connections", "mcp_count", len(allMCPs))

	// Fresh instance per server lifecycle: deregister first to avoid reusing existing clients
	// Log errors but proceed to re-register.
	for i := range allMCPs {
		if err := service.proxy.Deregister(ctx, allMCPs[i].ID); err != nil {
			log.Debug("Failed to deregister MCP (may not exist)", "mcp", allMCPs[i].ID, "error", err)
		}
	}

	// Register all
	if err := service.EnsureMultiple(ctx, allMCPs); err != nil {
		return service, err
	}

	// Wait until all registered MCPs report connected
	clientNames := make([]string, 0, len(allMCPs))
	for i := range allMCPs {
		clientNames = append(clientNames, allMCPs[i].ID)
	}
	// Bound the total wait; surface detailed errors on timeout
	waitCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := service.proxy.WaitForConnections(waitCtx, clientNames, 200*time.Millisecond); err != nil {
		return service, fmt.Errorf("MCP connection readiness failed: %w", err)
	}

	log.Info("All MCP clients connected", "count", len(clientNames))
	return service, nil
}

// CollectWorkflowMCPs collects all unique MCP configurations from all workflows
func CollectWorkflowMCPs(workflows []WorkflowConfig) []Config {
	seen := make(map[string]bool)
	var allMCPs []Config

	for _, workflow := range workflows {
		mcps := workflow.GetMCPs()
		for i := range mcps {
			mcps[i].SetDefaults() // Ensure defaults are applied
			// Resolve headers using hierarchical env from workflow
			if len(mcps[i].Headers) > 0 {
				mcps[i].Headers = resolveHeadersWithEnv(mcps[i].Headers, workflow.GetEnv())
			}
			// Use ID to deduplicate MCPs across workflows
			if !seen[mcps[i].ID] {
				seen[mcps[i].ID] = true
				allMCPs = append(allMCPs, mcps[i])
			}
		}
	}

	return allMCPs
}

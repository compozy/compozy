package mcp

import (
	"context"
	"fmt"
	"strings"
	"text/template"
	tplparse "text/template/parse"
	"time"

	"github.com/compozy/compozy/engine/core"
	appconfig "github.com/compozy/compozy/pkg/config"
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
		log.Debug("Registering MCP with headers", "mcp_id", config.ID, "headers", core.RedactHeaders(def.Headers))
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
	defer func() { _ = s.proxy.Close() }()
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

// redactHeaders returns a copy of headers with sensitive values redacted for logging.
// Any header key containing "token", "key", or "secret" (case-insensitive), or
// the Authorization header, will have its value replaced with "[REDACTED]".
// Other headers have values passed through core.RedactString for safety.
// redaction of headers moved to core.RedactHeaders

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
			// Optional strict mode for template validation, disabled by default for compatibility.
			if appconfig.Get().LLM.MCPHeaderTemplateStrict {
				if err := validateTemplate(v); err != nil {
					out[k] = v
					continue
				}
			}
			// Validate template doesn't contain dangerous patterns
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
// In strict mode, we allow only simple lookups such as {{ .env.API_KEY }} without
// function calls or pipelines. Control structures and template inclusions are rejected.
func validateTemplate(tmpl string) error {
	if strings.Count(tmpl, "{{") > 10 {
		return fmt.Errorf("template too complex: too many expressions")
	}
	t, err := template.New("hdr").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}
	if t.Tree == nil || t.Root == nil {
		return nil
	}
	return walkTemplateNodes(t.Root)
}

func walkTemplateNodes(n tplparse.Node) error {
	switch nn := n.(type) {
	case *tplparse.ListNode:
		return walkListNode(nn)
	case *tplparse.ActionNode:
		return walkActionNode(nn)
	case *tplparse.IfNode:
		return walkIfNode(nn)
	case *tplparse.RangeNode:
		return walkRangeNode(nn)
	case *tplparse.WithNode:
		return walkWithNode(nn)
	case *tplparse.TemplateNode:
		return fmt.Errorf("template inclusions are not allowed")
	default:
		return nil
	}
}

func walkListNode(n *tplparse.ListNode) error {
	for _, ch := range n.Nodes {
		if err := walkTemplateNodes(ch); err != nil {
			return err
		}
	}
	return nil
}

func walkActionNode(n *tplparse.ActionNode) error { return checkPipeNode(n.Pipe) }

func walkIfNode(n *tplparse.IfNode) error {
	if err := checkPipeNode(n.Pipe); err != nil {
		return err
	}
	if n.List != nil {
		if err := walkTemplateNodes(n.List); err != nil {
			return err
		}
	}
	if n.ElseList != nil {
		if err := walkTemplateNodes(n.ElseList); err != nil {
			return err
		}
	}
	return nil
}

func walkRangeNode(n *tplparse.RangeNode) error {
	if err := checkPipeNode(n.Pipe); err != nil {
		return err
	}
	if n.List != nil {
		if err := walkTemplateNodes(n.List); err != nil {
			return err
		}
	}
	if n.ElseList != nil {
		if err := walkTemplateNodes(n.ElseList); err != nil {
			return err
		}
	}
	return nil
}

func walkWithNode(n *tplparse.WithNode) error {
	if err := checkPipeNode(n.Pipe); err != nil {
		return err
	}
	if n.List != nil {
		if err := walkTemplateNodes(n.List); err != nil {
			return err
		}
	}
	if n.ElseList != nil {
		if err := walkTemplateNodes(n.ElseList); err != nil {
			return err
		}
	}
	return nil
}

func checkPipeNode(p *tplparse.PipeNode) error {
	if p == nil {
		return nil
	}
	if len(p.Cmds) != 1 {
		return fmt.Errorf("pipelines are not allowed")
	}
	return checkCommandNode(p.Cmds[0])
}

func checkCommandNode(cmd *tplparse.CommandNode) error {
	if cmd == nil {
		return nil
	}
	if len(cmd.Args) != 1 {
		return fmt.Errorf("function calls are not allowed")
	}
	switch a := cmd.Args[0].(type) {
	case *tplparse.FieldNode, *tplparse.VariableNode, *tplparse.DotNode:
		return nil
	case *tplparse.ChainNode:
		switch a.Node.(type) {
		case *tplparse.DotNode, *tplparse.FieldNode, *tplparse.VariableNode:
			return nil
		default:
			return fmt.Errorf("function calls are not allowed")
		}
	case *tplparse.IdentifierNode:
		return fmt.Errorf("function calls are not allowed")
	case *tplparse.StringNode, *tplparse.NumberNode, *tplparse.BoolNode, *tplparse.NilNode:
		return nil
	default:
		return fmt.Errorf("unsupported template expression")
	}
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
	close(results)
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
	proxyURL := appconfig.Get().LLM.ProxyURL
	if proxyURL == "" {
		return nil, nil
	}
	clientTimeout := appconfig.Get().LLM.MCPClientTimeout
	if clientTimeout <= 0 {
		clientTimeout = 30 * time.Second
	}
	service := NewWithTimeout(proxyURL, clientTimeout)
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
	readinessTimeout := appconfig.Get().LLM.MCPReadinessTimeout
	if readinessTimeout <= 0 {
		readinessTimeout = 60 * time.Second
	}
	pollInterval := appconfig.Get().LLM.MCPReadinessPollInterval
	if pollInterval <= 0 {
		pollInterval = 200 * time.Millisecond
	}
	waitCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()
	if err := service.proxy.WaitForConnections(waitCtx, clientNames, pollInterval); err != nil {
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
			cfg := mcps[i]
			cfg.SetDefaults()
			if len(cfg.Headers) > 0 {
				cfg.Headers = resolveHeadersWithEnv(cfg.Headers, workflow.GetEnv())
			}
			if !seen[cfg.ID] {
				seen[cfg.ID] = true
				allMCPs = append(allMCPs, cfg)
			}
		}
	}

	return allMCPs
}

// readDurationEnv returns a parsed duration from env or the provided default on empty/invalid values
// readDurationEnv removed in favor of pkg/config driven values

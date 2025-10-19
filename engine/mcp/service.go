package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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

const (
	defaultMaxConcurrentMCPRegistrations = 5
	defaultMCPClientTimeout              = 30 * time.Second
	defaultMCPReadinessTimeout           = 60 * time.Second
	defaultMCPReadinessPollInterval      = 200 * time.Millisecond
)

// WorkflowConfig represents the minimal interface needed from workflow configs
type WorkflowConfig interface {
	GetMCPs() []Config
	GetEnv() core.EnvMap
}

// RegisterService manages MCP registration lifecycle with the proxy
type RegisterService struct {
	proxy *Client
	// registered holds MCP IDs successfully registered by this process
	registeredMu sync.RWMutex
	registered   map[string]struct{}
}

// NewRegisterService creates a new RegisterService that uses the provided MCP proxy client.
// The returned service has an initialized internal set for tracking registered MCP IDs.
func NewRegisterService(proxyClient *Client) *RegisterService {
	return &RegisterService{
		proxy:      proxyClient,
		registered: make(map[string]struct{}),
	}
}

// NewWithTimeout creates a service with a configured proxy client using default timeout
func NewWithTimeout(ctx context.Context, proxyURL string, timeout time.Duration) *RegisterService {
	proxyClient := NewProxyClient(ctx, proxyURL, timeout)
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
	// Track successful registrations for best-effort shutdown without admin list
	s.registeredMu.Lock()
	s.registered[config.ID] = struct{}{}
	s.registeredMu.Unlock()
	return nil
}

// Deregister removes an MCP from the proxy
func (s *RegisterService) Deregister(ctx context.Context, mcpID string) error {
	log := logger.FromContext(ctx)
	// Deregister from proxy
	if err := s.proxy.Deregister(ctx, mcpID); err != nil {
		return fmt.Errorf("failed to deregister MCP from proxy: %w", err)
	}

	s.registeredMu.Lock()
	delete(s.registered, mcpID)
	s.registeredMu.Unlock()

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
	defer func() {
		if err := s.proxy.Close(); err != nil {
			log.Warn("Failed to close MCP proxy client", "error", err)
		}
	}()
	log.Info("Shutting down MCP register, deregistering all MCPs")
	ids := s.collectShutdownIDs(ctx)
	if len(ids) == 0 {
		log.Debug("No MCPs to deregister during shutdown")
		return nil
	}
	s.deregisterIDs(ctx, ids)
	log.Info("MCP register shutdown completed successfully")
	return nil
}

// collectShutdownIDs determines which MCP IDs to deregister, preferring locally tracked IDs.
// Helper placed directly after Shutdown per request.
func (s *RegisterService) collectShutdownIDs(ctx context.Context) []string {
	s.registeredMu.Lock()
	if len(s.registered) > 0 {
		ids := make([]string, 0, len(s.registered))
		for id := range s.registered {
			ids = append(ids, id)
		}
		s.registered = make(map[string]struct{})
		s.registeredMu.Unlock()
		return ids
	}
	s.registeredMu.Unlock()
	mcps, err := s.ListRegistered(ctx)
	if err != nil {
		logger.FromContext(ctx).Warn("Skipping MCP deregistration due to list failure", "reason", err)
		return nil
	}
	return mcps
}

// deregisterIDs performs best-effort concurrent deregistration; helper kept close to Shutdown.
func (s *RegisterService) deregisterIDs(ctx context.Context, ids []string) {
	log := logger.FromContext(ctx)
	g, gCtx := errgroup.WithContext(ctx)
	for _, id := range ids {
		id := id
		g.Go(func() error {
			if err := s.proxy.Deregister(gCtx, id); err != nil {
				log.Warn("Failed to deregister MCP during shutdown", "mcp_id", id, "error", err)
			} else {
				log.Debug("Deregistered MCP during shutdown", "mcp_id", id)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Warn("Errors occurred during MCP deregistration", "error", err)
	}
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
		if len(config.Args) > 0 {
			def.Args = append(def.Args, config.Args...)
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

	// Enforce transport compatibility per MCP spec
	t := strings.ToLower(string(def.Transport))
	if def.URL != "" {
		if t != "sse" && t != "streamable-http" {
			return def, fmt.Errorf("remote MCP must use 'sse' or 'streamable-http' transport: got %q", def.Transport)
		}
	}
	if def.Command != "" {
		if t != "stdio" {
			return def, fmt.Errorf("stdio MCP must use 'stdio' transport: got %q", def.Transport)
		}
	}

	return def, nil
}

// normalizeHeaders returns a defensive copy of headers with case normalization.
// It ensures the Authorization header uses proper casing but does NOT infer schemes.
// normalizeHeaders returns a defensive copy of the provided headers with canonical
// casing for the Authorization header.
//
// It preserves all header keys/values from the input map but normalizes any header
// whose key equals "authorization" (case-insensitively) to the exact key
// "Authorization". If the input is nil or empty, an empty map is returned.
// The function does not modify the input map.
//
// Users must provide a complete Authorization value (including scheme) when one is used.
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

// resolveHeadersWithEnv evaluates header templates using the provided env (hierarchical env already merged by loader)
// the engine provides its own escaping protections.
func resolveHeadersWithEnv(ctx context.Context, headers map[string]string, env core.EnvMap) map[string]string {
	if len(headers) == 0 {
		return headers
	}
	out := make(map[string]string, len(headers))
	engine := tplengine.NewEngine(tplengine.FormatText)
	tplCtx := map[string]any{"env": map[string]string(env)}
	for k, v := range headers {
		if tplengine.HasTemplate(v) {
			// Optional strict mode for template validation, disabled by default for compatibility.
			if cfg := appconfig.FromContext(ctx); cfg != nil && cfg.LLM.MCPHeaderTemplateStrict {
				if err := validateTemplate(v); err != nil {
					out[k] = v
					continue
				}
			}
			// Validate template doesn't contain dangerous patterns
			if s, err := engine.ProcessString(v, tplCtx); err == nil {
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

// walkListNode walks each child node of the provided template ListNode and
// validates them via walkTemplateNodes, returning the first error encountered.
func walkListNode(n *tplparse.ListNode) error {
	for _, ch := range n.Nodes {
		if err := walkTemplateNodes(ch); err != nil {
			return err
		}
	}
	return nil
}

// walkActionNode validates the given template ActionNode by checking its pipeline for disallowed
// constructs and returns any validation error.
func walkActionNode(n *tplparse.ActionNode) error {
	return checkPipeNode(n.Pipe)
}

// walkIfNode validates an IfNode's pipe and recursively validates its List and ElseList children.
// It ensures the condition pipe is allowed (via checkPipeNode) and walks any contained template node lists,
// returning the first encountered validation error.
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
// parseCommand parses a shell-like command string into its command and arguments.
// It uses a shell lexer to handle quotes and escaped spaces, and applies validation:
// - input must be non-empty and not contain newlines,
// - the parsed result must contain at least one token,
// - the command name (first token) must not start with a dash.
// Returns the parsed tokens or a descriptive error.
func parseCommand(command string) ([]string, error) {
	if command == "" {
		return nil, fmt.Errorf("command cannot be empty")
	}
	command = strings.TrimSpace(command)
	if strings.Contains(command, "\n") || strings.Contains(command, "\r") {
		return nil, fmt.Errorf("command cannot contain newlines")
	}
	parts, err := shlex.Split(command)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command: %w", err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("command cannot be empty after parsing")
	}
	if strings.HasPrefix(parts[0], "-") {
		return nil, fmt.Errorf("command name cannot start with dash")
	}
	return parts, nil
}

// EnsureMultiple registers multiple MCPs in parallel with error handling
func (s *RegisterService) EnsureMultiple(ctx context.Context, configs []Config) error {
	log := logger.FromContext(ctx)
	if err := s.validateEnsureMultipleInput(ctx, configs); err != nil {
		return err
	}
	maxConcurrent := s.getConcurrencyLimit(ctx)
	log.Info("Registering multiple MCPs with proxy", "count", len(configs), "max_concurrent", maxConcurrent)
	results := s.executeConcurrentRegistrations(ctx, configs, maxConcurrent)
	s.logRegistrationResults(ctx, configs, results)
	return s.aggregateRegistrationErrors(results.errors)
}

// validateEnsureMultipleInput performs input validation for EnsureMultiple
func (s *RegisterService) validateEnsureMultipleInput(ctx context.Context, configs []Config) error {
	if len(configs) == 0 {
		return nil
	}
	if ctx.Err() != nil {
		return fmt.Errorf("registration canceled: %w", ctx.Err())
	}
	return nil
}

// getConcurrencyLimit retrieves the maximum concurrent registrations from config with fallback
func (s *RegisterService) getConcurrencyLimit(ctx context.Context) int {
	maxConcurrent := defaultMaxConcurrentMCPRegistrations
	if cfg := appconfig.FromContext(ctx); cfg != nil && cfg.LLM.MaxConcurrentTools > 0 {
		maxConcurrent = cfg.LLM.MaxConcurrentTools
	}
	return maxConcurrent
}

// registrationResults holds the results of concurrent MCP registration
type registrationResults struct {
	successCount int
	errors       []error
}

// executeConcurrentRegistrations performs the actual concurrent MCP registrations
func (s *RegisterService) executeConcurrentRegistrations(
	ctx context.Context,
	configs []Config,
	maxConcurrent int,
) registrationResults {
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)
	var mu sync.Mutex
	var errs []error
	var successCount int
	for i := range configs {
		config := configs[i]
		g.Go(func() error {
			return s.registerSingleMCP(gCtx, &config, &mu, &errs, &successCount)
		})
	}
	_ = g.Wait() //nolint:errcheck // Error handling is done within individual goroutines
	return registrationResults{
		successCount: successCount,
		errors:       errs,
	}
}

// registerSingleMCP registers a single MCP and handles result collection
func (s *RegisterService) registerSingleMCP(
	ctx context.Context,
	config *Config,
	mu *sync.Mutex,
	errs *[]error,
	successCount *int,
) error {
	log := logger.FromContext(ctx)
	if err := s.Ensure(ctx, config); err != nil {
		mu.Lock()
		log.Error("Failed to register MCP", "mcp_id", config.ID, "error", err)
		*errs = append(*errs, fmt.Errorf("MCP %s: %w", config.ID, err))
		mu.Unlock()
		return err // Continue with other registrations
	}
	mu.Lock()
	*successCount++
	mu.Unlock()
	return nil
}

// logRegistrationResults logs the final results of the MCP registration process
func (s *RegisterService) logRegistrationResults(ctx context.Context, configs []Config, results registrationResults) {
	log := logger.FromContext(ctx)
	if len(results.errors) > 0 {
		log.Info("MCP registration completed with errors",
			"total", len(configs),
			"successful", results.successCount,
			"failed", len(results.errors))
	} else {
		log.Info("MCP registration completed successfully",
			"total", len(configs),
			"successful", results.successCount)
	}
}

// aggregateRegistrationErrors combines multiple registration errors into a single error
func (s *RegisterService) aggregateRegistrationErrors(errs []error) error {
	if len(errs) > 0 {
		return fmt.Errorf("failed to register %d MCPs: %w", len(errs), errors.Join(errs...))
	}
	return nil
}

// -----------------------------------------------------------------------------
// Setup and Initialization Functions
// -----------------------------------------------------------------------------

// SetupForWorkflows creates and initializes an MCP RegisterService for the given workflows
// SetupForWorkflows creates and initializes an MCP RegisterService from application
// configuration and registers all unique MCPs referenced by the provided workflows.
//
// If the proxy is not configured (no MCP proxy URL), this returns (nil, nil).
//
// For a configured proxy, SetupForWorkflows will:
// - build a RegisterService from app config,
// - collect unique MCP configurations across the given workflows,
// - attempt to deregister any existing MCPs with the same IDs (best-effort, errors are logged),
// - register all collected MCPs, and
// - wait (bounded by application-configured timeouts) for each MCP to report a connected state via the proxy.
//
// The function returns the initialized RegisterService even if some registration steps fail;
// an error is returned when registration or readiness waiting fails.
//
// Parameters:
// - ctx: context for cancellation and timeouts used during registration and readiness polling.
// - workflows: a slice of WorkflowConfig used to discover MCP configurations to register.
//
// Side effects:
// - May deregister and register MCPs on the configured proxy.
// - Blocks until MCPs report readiness or the configured readiness timeout elapses.
func SetupForWorkflows(ctx context.Context, workflows []WorkflowConfig) (*RegisterService, error) {
	log := logger.FromContext(ctx)
	service := setupRegisterServiceFromApp(ctx)
	if service == nil {
		return nil, nil
	}

	allMCPs := CollectWorkflowMCPs(ctx, workflows)
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
	cfg := appconfig.FromContext(ctx)
	var readinessTimeout time.Duration
	if cfg != nil && cfg.LLM.MCPReadinessTimeout > 0 {
		readinessTimeout = cfg.LLM.MCPReadinessTimeout
	} else {
		readinessTimeout = defaultMCPReadinessTimeout
	}
	var pollInterval time.Duration
	if cfg != nil && cfg.LLM.MCPReadinessPollInterval > 0 {
		pollInterval = cfg.LLM.MCPReadinessPollInterval
	} else {
		pollInterval = defaultMCPReadinessPollInterval
	}
	waitCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()
	if err := service.proxy.WaitForConnections(waitCtx, clientNames, pollInterval); err != nil {
		return service, fmt.Errorf("MCP connection readiness failed: %w", err)
	}

	log.Info("All MCP clients connected", "count", len(clientNames))
	return service, nil
}

// setupRegisterServiceFromApp builds a RegisterService from application config.
// It returns nil if no proxy URL is configured. The MCP client timeout is taken
// from app config (defaults to 30s when unset or non-positive). The created
// service is initialized with a proxy client using those settings.
func setupRegisterServiceFromApp(ctx context.Context) *RegisterService {
	log := logger.FromContext(ctx)
	cfg := appconfig.FromContext(ctx)
	var proxyURL string
	if cfg != nil {
		proxyURL = cfg.LLM.ProxyURL
	}
	if proxyURL == "" {
		return nil
	}
	var clientTimeout time.Duration
	if cfg != nil && cfg.LLM.MCPClientTimeout > 0 {
		clientTimeout = cfg.LLM.MCPClientTimeout
	} else {
		clientTimeout = defaultMCPClientTimeout
	}
	service := NewWithTimeout(ctx, proxyURL, clientTimeout)
	// Avoid logging full proxy URL which may contain credentials
	log.Info("Initialized MCP register with proxy", "proxy_configured", true)
	return service
}

// CollectWorkflowMCPs returns a deduplicated list of MCP configs referenced by the provided workflows.
//
// For each workflow it:
// - retrieves MCP configs via GetMCPs,
// - applies SetDefaults to each config,
// - resolves header templates against the workflow's environment (GetEnv) when headers are present,
// and then includes the config in the result if its ID has not already been seen.
// The returned slice preserves the first-seen order of unique MCP IDs.
func CollectWorkflowMCPs(ctx context.Context, workflows []WorkflowConfig) []Config {
	seen := make(map[string]bool)
	var allMCPs []Config

	for _, workflow := range workflows {
		mcps := workflow.GetMCPs()
		for i := range mcps {
			cfg := mcps[i]
			cfg.SetDefaults()
			if len(cfg.Headers) > 0 {
				cfg.Headers = resolveHeadersWithEnv(ctx, cfg.Headers, workflow.GetEnv())
			}
			if !seen[cfg.ID] {
				seen[cfg.ID] = true
				allMCPs = append(allMCPs, cfg)
			}
		}
	}

	return allMCPs
}

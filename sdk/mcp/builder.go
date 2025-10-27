package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	enginemcp "github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/pkg/logger"
	mcpproxy "github.com/compozy/compozy/pkg/mcp-proxy"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// Builder constructs engine MCP configurations using a fluent API while accumulating validation errors.
type Builder struct {
	config *enginemcp.Config
	errors []error
}

// New creates an MCP builder initialized with the provided identifier.
func New(id string) *Builder {
	trimmedID := strings.TrimSpace(id)
	return &Builder{
		config: &enginemcp.Config{
			Resource: string(core.ConfigMCP),
			ID:       trimmedID,
		},
		errors: make([]error, 0),
	}
}

// WithCommand configures a command-based MCP server using stdio transport.
func (b *Builder) WithCommand(command string, args ...string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("mcp command cannot be empty"))
		b.config.Command = ""
		b.config.Args = nil
		return b
	}
	b.config.Command = trimmed
	if len(args) > 0 {
		copied := make([]string, len(args))
		for idx, value := range args {
			copied[idx] = strings.TrimSpace(value)
		}
		b.config.Args = copied
	} else {
		b.config.Args = nil
	}
	if b.config.Transport == "" {
		b.config.Transport = mcpproxy.TransportStdio
	}
	return b
}

// WithURL configures a URL-based MCP server using SSE/HTTP transport.
func (b *Builder) WithURL(url string) *Builder {
	if b == nil {
		return nil
	}
	trimmed := strings.TrimSpace(url)
	if trimmed == "" {
		b.errors = append(b.errors, fmt.Errorf("mcp url cannot be empty"))
		b.config.URL = ""
		return b
	}
	b.config.URL = trimmed
	if b.config.Transport == "" {
		b.config.Transport = enginemcp.DefaultTransport
	}
	return b
}

// WithTransport sets the transport type explicitly for the MCP server configuration.
func (b *Builder) WithTransport(transport mcpproxy.TransportType) *Builder {
	if b == nil {
		return nil
	}
	if !transport.IsValid() {
		b.errors = append(b.errors, fmt.Errorf("invalid transport type %q", transport))
		return b
	}
	b.config.Transport = transport
	return b
}

// Build validates the MCP configuration and returns a deep-copied engine config.
func (b *Builder) Build(ctx context.Context) (*enginemcp.Config, error) {
	if b == nil {
		return nil, fmt.Errorf("mcp builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("building MCP configuration", "mcp", b.config.ID)

	collected := make([]error, 0, len(b.errors)+4)
	collected = append(collected, b.errors...)
	if err := b.validateID(ctx); err != nil {
		collected = append(collected, err)
	}
	if err := b.validateSelection(ctx); err != nil {
		collected = append(collected, err)
	}
	if err := b.validateCommand(ctx); err != nil {
		collected = append(collected, err)
	}
	if err := b.validateURL(ctx); err != nil {
		collected = append(collected, err)
	}
	if err := b.validateTransport(ctx); err != nil {
		collected = append(collected, err)
	}

	filtered := make([]error, 0, len(collected))
	for _, err := range collected {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	b.config.SetDefaults()
	clone, err := core.DeepCopy(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone mcp config: %w", err)
	}
	return clone, nil
}

func (b *Builder) validateID(ctx context.Context) error {
	b.config.ID = strings.TrimSpace(b.config.ID)
	if err := validate.ValidateID(ctx, b.config.ID); err != nil {
		return fmt.Errorf("mcp id is invalid: %w", err)
	}
	return nil
}

func (b *Builder) validateSelection(_ context.Context) error {
	hasCommand := strings.TrimSpace(b.config.Command) != ""
	hasURL := strings.TrimSpace(b.config.URL) != ""
	switch {
	case hasCommand && hasURL:
		return fmt.Errorf("configure either command or url, not both")
	case !hasCommand && !hasURL:
		return fmt.Errorf("either command or url must be configured")
	default:
		return nil
	}
}

func (b *Builder) validateCommand(ctx context.Context) error {
	if strings.TrimSpace(b.config.Command) == "" {
		return nil
	}
	b.config.Command = strings.TrimSpace(b.config.Command)
	if err := validate.ValidateNonEmpty(ctx, "mcp command", b.config.Command); err != nil {
		return err
	}
	if len(b.config.Args) > 0 {
		cleaned := make([]string, len(b.config.Args))
		for idx, value := range b.config.Args {
			cleaned[idx] = strings.TrimSpace(value)
		}
		b.config.Args = cleaned
	}
	if b.config.Transport == "" {
		b.config.Transport = mcpproxy.TransportStdio
	}
	return nil
}

func (b *Builder) validateURL(ctx context.Context) error {
	if strings.TrimSpace(b.config.URL) == "" {
		return nil
	}
	b.config.URL = strings.TrimSpace(b.config.URL)
	if err := validate.ValidateURL(ctx, b.config.URL); err != nil {
		return fmt.Errorf("mcp url is invalid: %w", err)
	}
	if b.config.Transport == "" {
		b.config.Transport = enginemcp.DefaultTransport
	}
	return nil
}

func (b *Builder) validateTransport(_ context.Context) error {
	transport := mcpproxy.TransportType(strings.TrimSpace(string(b.config.Transport)))
	if transport == "" {
		return nil
	}
	b.config.Transport = transport
	switch transport {
	case mcpproxy.TransportStdio:
		if strings.TrimSpace(b.config.URL) != "" {
			return fmt.Errorf("stdio transport cannot be used with url configuration")
		}
		if strings.TrimSpace(b.config.Command) == "" {
			return fmt.Errorf("stdio transport requires a command to be configured")
		}
	case mcpproxy.TransportSSE, mcpproxy.TransportStreamableHTTP:
		if strings.TrimSpace(b.config.Command) != "" {
			return fmt.Errorf("%s transport cannot be used with command configuration", transport)
		}
		if strings.TrimSpace(b.config.URL) == "" {
			return fmt.Errorf("%s transport requires a url to be configured", transport)
		}
	default:
		return fmt.Errorf("invalid transport type %q", transport)
	}
	return nil
}

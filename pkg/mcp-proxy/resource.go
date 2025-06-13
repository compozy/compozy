package mcpproxy

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"golang.org/x/sync/errgroup"
)

// ResourceLoader handles loading different types of MCP resources from a client to a proxy server.
// It provides concurrent loading with bounded parallelism using a semaphore to prevent resource exhaustion.
// The loader supports pagination for large resource sets and includes proper error handling and logging.
type ResourceLoader struct {
	client    *MCPClient        // MCP client to load resources from
	mcpServer *server.MCPServer // Proxy server to register resources to
	name      string            // Client name for logging and identification
	sem       chan struct{}     // Reusable semaphore to limit concurrent operations
}

// NewResourceLoader creates a new resource loader for the given MCP client and proxy server.
// The loader is configured with a bounded semaphore (maxConcurrentAdds=5) to limit concurrent
// resource registration operations and prevent overwhelming the system.
func NewResourceLoader(client *MCPClient, mcpServer *server.MCPServer, name string) *ResourceLoader {
	const maxConcurrentAdds = 5
	return &ResourceLoader{
		client:    client,
		mcpServer: mcpServer,
		name:      name,
		sem:       make(chan struct{}, maxConcurrentAdds),
	}
}

// LoadTools loads tools from the MCP client to the proxy server.
// Tools are filtered according to the provided toolFilter configuration (allow/block lists).
// This method loads all tools at once (no pagination) and applies filtering before registration.
func (rl *ResourceLoader) LoadTools(ctx context.Context, toolFilter *ToolFilter) error {
	tools, err := rl.client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools: %w", err)
	}

	logger.Info("Successfully listed tools", "name", rl.name, "count", len(tools))

	// Create tool filter function
	filterFunc := rl.createToolFilter(toolFilter)

	addedCount := 0
	for i := range tools {
		tool := &tools[i]
		if filterFunc(tool.Name) {
			logger.Debug("Adding tool to proxy server", "name", rl.name, "tool", tool.Name)
			rl.mcpServer.AddTool(*tool, rl.client.CallTool)
			addedCount++
		}
	}

	logger.Info("Successfully added filtered tools", "name", rl.name, "total", len(tools), "added", addedCount)
	return nil
}

// LoadPrompts loads prompts from the MCP client to the proxy server with pagination.
// Uses the generic loadResources function to handle pagination, concurrency, and error handling.
// Each prompt is registered with its corresponding GetPrompt handler from the client.
func (rl *ResourceLoader) LoadPrompts(ctx context.Context) error {
	return loadResources(ctx, rl.name, "prompts", rl.sem,
		rl.client.ListPromptsWithCursor,
		func(_ context.Context, prompt mcp.Prompt) error {
			logger.Debug("Adding prompt to proxy server", "name", rl.name, "prompt", prompt.Name)
			rl.mcpServer.AddPrompt(prompt, rl.client.GetPrompt)
			return nil
		},
	)
}

// LoadResources loads resources from the MCP client to the proxy server with pagination.
// Uses the generic loadResources function to handle pagination, concurrency, and error handling.
// Each resource is registered with a ReadResource handler that forwards requests to the client.
func (rl *ResourceLoader) LoadResources(ctx context.Context) error {
	return loadResources(ctx, rl.name, "resources", rl.sem,
		rl.client.ListResourcesWithCursor,
		func(_ context.Context, resource mcp.Resource) error {
			logger.Debug("Adding resource to proxy server", "name", rl.name, "resource", resource.URI)
			rl.mcpServer.AddResource(
				resource,
				func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
					result, err := rl.client.ReadResource(ctx, request)
					if err != nil {
						return nil, err
					}
					return result.Contents, nil
				},
			)
			return nil
		},
	)
}

// LoadResourceTemplates loads resource templates from the MCP client to the proxy server with pagination.
// Uses the generic loadResources function to handle pagination, concurrency, and error handling.
// Each template is registered with a ReadResource handler that forwards requests to the client.
func (rl *ResourceLoader) LoadResourceTemplates(ctx context.Context) error {
	return loadResources(ctx, rl.name, "resource templates", rl.sem,
		rl.client.ListResourceTemplatesWithCursor,
		func(_ context.Context, template mcp.ResourceTemplate) error {
			logger.Debug("Adding resource template to proxy server", "name", rl.name, "template", template.Name)
			rl.mcpServer.AddResourceTemplate(
				template,
				func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
					result, err := rl.client.ReadResource(ctx, request)
					if err != nil {
						return nil, err
					}
					return result.Contents, nil
				},
			)
			return nil
		},
	)
}

// loadResources is a generic function to handle paginated resource loading with bounded concurrency.
// It handles pagination using cursors, processes items in batches with concurrent registration,
// and uses a semaphore to limit concurrent operations to prevent resource exhaustion.
// The function is type-safe and reusable for different MCP resource types.
//
// Type parameter T represents the resource type (e.g., mcp.Prompt, mcp.Resource, mcp.ResourceTemplate).
// The listFn should return items, nextCursor, and error for pagination.
// The addFn should handle registration of individual items to the proxy server.
func loadResources[T any](
	ctx context.Context,
	name string,
	resourceType string,
	sem chan struct{},
	listFn func(context.Context, string) ([]T, string, error),
	addFn func(context.Context, T) error,
) error {
	var cursor string
	totalCount := 0

	for {
		items, nextCursor, err := listFn(ctx, cursor)
		if err != nil {
			return fmt.Errorf("failed to list %s: %w", resourceType, err)
		}

		if len(items) == 0 {
			break
		}

		totalCount += len(items)
		logger.Debug("Listed batch", "name", name, "type", resourceType, "count", len(items), "cursor", cursor)

		// Process items in this batch concurrently
		g, gCtx := errgroup.WithContext(ctx)

		for _, item := range items {
			item := item // capture loop variable

			g.Go(func() error {
				// Acquire semaphore to limit concurrency
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-gCtx.Done():
					return gCtx.Err()
				}

				return addFn(gCtx, item)
			})
		}

		// Wait for all items in this batch to be processed
		if err := g.Wait(); err != nil {
			return fmt.Errorf("failed to add %s: %w", resourceType, err)
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	logger.Info("Successfully added all resources", "name", name, "type", resourceType, "total", totalCount)
	return nil
}

// createToolFilter creates a tool filtering function based on the provided configuration.
// Returns a function that evaluates whether a tool should be included based on allow/block lists.
// If no filter is configured, all tools are allowed. The function logs filtering decisions for debugging.
func (rl *ResourceLoader) createToolFilter(filter *ToolFilter) func(string) bool {
	if filter == nil || len(filter.List) == 0 {
		return func(_ string) bool { return true }
	}

	filterSet := make(map[string]struct{})
	for _, toolName := range filter.List {
		filterSet[toolName] = struct{}{}
	}

	switch filter.Mode {
	case ToolFilterAllow:
		return func(toolName string) bool {
			_, inList := filterSet[toolName]
			if !inList {
				logger.Debug("Tool filtered out by allow list", "client", rl.name, "tool", toolName)
			}
			return inList
		}
	case ToolFilterBlock:
		return func(toolName string) bool {
			_, inList := filterSet[toolName]
			if inList {
				logger.Debug("Tool filtered out by block list", "client", rl.name, "tool", toolName)
			}
			return !inList
		}
	default:
		logger.Warn("Unknown tool filter mode, allowing all tools", "client", rl.name, "mode", filter.Mode)
		return func(_ string) bool { return true }
	}
}

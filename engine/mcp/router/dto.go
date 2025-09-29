package mcprouter

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/mcp"
)

type MCPDTO struct {
	MCPCoreDTO
}

type MCPListItem struct {
	MCPCoreDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

type MCPsListResponse struct {
	MCPs []MCPListItem       `json:"mcps"`
	Page httpdto.PageInfoDTO `json:"page"`
}

type MCPCoreDTO struct {
	Resource string `json:"resource,omitempty"`
	ID       string `json:"id"`
	URL      string `json:"url,omitempty"`
	Command  string `json:"command,omitempty"`
	// Args lists additional command arguments when the MCP server runs via stdio transport.
	Args         []string          `json:"args,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Proto        string            `json:"proto,omitempty"`
	Transport    string            `json:"transport,omitempty"`
	StartTimeout time.Duration     `json:"start_timeout,omitempty"`
	MaxSessions  int               `json:"max_sessions,omitempty"`
}

func toMCPDTO(src map[string]any) (MCPDTO, error) {
	cfg, err := mapToMCPConfig(src)
	if err != nil {
		return MCPDTO{}, err
	}
	coreDTO, err := convertMCPConfigToDTO(cfg)
	if err != nil {
		return MCPDTO{}, err
	}
	return MCPDTO{MCPCoreDTO: coreDTO}, nil
}

func toMCPListItem(src map[string]any) (MCPListItem, error) {
	dto, err := toMCPDTO(src)
	if err != nil {
		return MCPListItem{}, err
	}
	return MCPListItem{MCPCoreDTO: dto.MCPCoreDTO, ETag: httpdto.AsString(src["_etag"])}, nil
}

func mapToMCPConfig(src map[string]any) (*mcp.Config, error) {
	if src == nil {
		return nil, fmt.Errorf("mcp payload is nil")
	}
	cfg, err := core.FromMapDefault[*mcp.Config](src)
	if err != nil {
		return nil, fmt.Errorf("map to mcp config: %w", err)
	}
	return cfg, nil
}

func convertMCPConfigToDTO(cfg *mcp.Config) (MCPCoreDTO, error) {
	if cfg == nil {
		return MCPCoreDTO{}, fmt.Errorf("mcp config is nil")
	}
	clone, err := core.DeepCopy[*mcp.Config](cfg)
	if err != nil {
		return MCPCoreDTO{}, fmt.Errorf("deep copy mcp config: %w", err)
	}
	return MCPCoreDTO{
		Resource:     clone.Resource,
		ID:           clone.ID,
		URL:          clone.URL,
		Command:      clone.Command,
		Args:         append([]string(nil), clone.Args...),
		Headers:      copyStringMap(clone.Headers),
		Env:          copyStringMap(clone.Env),
		Proto:        clone.Proto,
		Transport:    string(clone.Transport),
		StartTimeout: clone.StartTimeout,
		MaxSessions:  clone.MaxSessions,
	}, nil
}

func copyStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

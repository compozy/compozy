package mcprouter

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/compozy/compozy/engine/infra/server/router"
)

type MCPDTO struct {
	MCPCoreDTO
}

type MCPListItem struct {
	MCPCoreDTO
	ETag string `json:"etag,omitempty" example:"abc123"`
}

type MCPsListResponse struct {
	MCPs []MCPListItem      `json:"mcps"`
	Page router.PageInfoDTO `json:"page"`
}

type MCPCoreDTO struct {
	Resource     string            `json:"resource,omitempty"`
	ID           string            `json:"id"`
	URL          string            `json:"url,omitempty"`
	Command      string            `json:"command,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	Proto        string            `json:"proto,omitempty"`
	Transport    string            `json:"transport,omitempty"`
	StartTimeout string            `json:"start_timeout,omitempty"`
	MaxSessions  *int              `json:"max_sessions,omitempty"`
}

func toMCPDTO(src map[string]any) MCPDTO {
	return MCPDTO{MCPCoreDTO: MCPCoreDTO{
		Resource:     router.AsString(src["resource"]),
		ID:           router.AsString(src["id"]),
		URL:          router.AsString(src["url"]),
		Command:      router.AsString(src["command"]),
		Headers:      stringMapFromAny(src["headers"]),
		Env:          stringMapFromAny(src["env"]),
		Proto:        router.AsString(src["proto"]),
		Transport:    router.AsString(src["transport"]),
		StartTimeout: durationStringFromAny(src["start_timeout"]),
		MaxSessions:  intPtrFromAny(src["max_sessions"]),
	}}
}

func toMCPListItem(src map[string]any) MCPListItem {
	return MCPListItem{MCPCoreDTO: MCPCoreDTO{
		Resource:     router.AsString(src["resource"]),
		ID:           router.AsString(src["id"]),
		URL:          router.AsString(src["url"]),
		Command:      router.AsString(src["command"]),
		Headers:      stringMapFromAny(src["headers"]),
		Env:          stringMapFromAny(src["env"]),
		Proto:        router.AsString(src["proto"]),
		Transport:    router.AsString(src["transport"]),
		StartTimeout: durationStringFromAny(src["start_timeout"]),
		MaxSessions:  intPtrFromAny(src["max_sessions"]),
	}, ETag: router.AsString(src["_etag"])}
}

func stringMapFromAny(v any) map[string]string {
	switch t := v.(type) {
	case map[string]string:
		out := make(map[string]string, len(t))
		for k, val := range t {
			out[k] = val
		}
		return out
	case map[string]any:
		out := make(map[string]string, len(t))
		for k, val := range t {
			if s, ok := val.(string); ok {
				out[k] = s
			}
		}
		return out
	default:
		return nil
	}
}

func intPtrFromAny(v any) *int {
	switch t := v.(type) {
	case int:
		if t == 0 {
			return nil
		}
		x := t
		return &x
	case int64:
		if t == 0 {
			return nil
		}
		x := int(t)
		return &x
	case float64:
		if t == 0 {
			return nil
		}
		x := int(t)
		return &x
	case string:
		if t == "" {
			return nil
		}
		if n, err := strconv.Atoi(t); err == nil {
			if n == 0 {
				return nil
			}
			x := n
			return &x
		}
	case json.Number:
		if n, err := t.Int64(); err == nil {
			if n == 0 {
				return nil
			}
			x := int(n)
			return &x
		}
	}
	return nil
}

func durationStringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case time.Duration:
		if t == 0 {
			return ""
		}
		return t.String()
	case int:
		if t == 0 {
			return ""
		}
		return time.Duration(t).String()
	case int64:
		if t == 0 {
			return ""
		}
		return time.Duration(t).String()
	case float64:
		if t == 0 {
			return ""
		}
		return time.Duration(int64(t)).String()
	case json.Number:
		if n, err := t.Int64(); err == nil {
			if n == 0 {
				return ""
			}
			return time.Duration(n).String()
		}
	}
	return ""
}

package mcprouter

import (
	"encoding/json"
	"math"
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
		return map[string]string{}
	}
}

func intPtrFromAny(v any) *int {
	switch t := v.(type) {
	case int:
		return intFromInt64(int64(t))
	case int64:
		return intFromInt64(t)
	case float64:
		return intFromFloat64(t)
	case string:
		return intFromString(t)
	case json.Number:
		return intFromJSONNumber(t)
	}
	return nil
}

func intFromInt64(n int64) *int {
	if n == 0 {
		return nil
	}
	if n > int64(math.MaxInt) || n < int64(math.MinInt) {
		return nil
	}
	x := int(n)
	return &x
}

func intFromFloat64(f float64) *int {
	if f == 0 {
		return nil
	}
	n := int64(f)
	if n == 0 {
		return nil
	}
	return intFromInt64(n)
}

func intFromString(s string) *int {
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return intFromInt64(n)
}

func intFromJSONNumber(num json.Number) *int {
	if n, err := num.Int64(); err == nil {
		return intFromInt64(n)
	}
	f, err := num.Float64()
	if err != nil {
		return nil
	}
	return intFromFloat64(f)
}

func durationStringFromAny(v any) string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return ""
		}
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
		return (time.Duration(t) * time.Second).String()
	case int64:
		if t == 0 {
			return ""
		}
		return (time.Duration(t) * time.Second).String()
	case float64:
		if t == 0 {
			return ""
		}
		return (time.Duration(int64(t)) * time.Second).String()
	case json.Number:
		if n, err := t.Float64(); err == nil {
			if n == 0 {
				return ""
			}
			return (time.Duration(int64(n)) * time.Second).String()
		}
	}
	return ""
}

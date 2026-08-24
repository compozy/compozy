package daemon

import (
	"context"
	"strings"

	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/soul"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
)

func (s *daemonExtensionService) extensionResourceName(
	ctx context.Context,
	record resources.RawRecord,
) string {
	if s == nil {
		return extensionResourceNameFromID(record)
	}
	return extensionResourceNameFromCodec(ctx, s.resourceCodecs, record)
}

func extensionResourceNameFromCodec(
	ctx context.Context,
	codecs *resources.CodecRegistry,
	record resources.RawRecord,
) string {
	switch record.Kind.Normalize() {
	case compozyconfig.AgentResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec compozyconfig.AgentDef) string {
			return spec.Name
		})
	case soul.ResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec soul.ResourceSpec) string {
			return spec.AgentName
		})
	case heartbeat.ResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec heartbeat.ResourceSpec) string {
			return spec.AgentName
		})
	case skillspkg.SkillResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec skillspkg.SkillResourceSpec) string {
			return spec.Name
		})
	case looppkg.ResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec looppkg.ResourceSpec) string {
			return spec.Name
		})
	case hookBindingResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec hookspkg.HookDecl) string {
			return spec.Name
		})
	case automationpkg.JobResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec automationpkg.Job) string {
			return spec.Name
		})
	case automationpkg.TriggerResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec automationpkg.Trigger) string {
			return spec.Name
		})
	case compozyconfig.MCPServerResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec compozyconfig.MCPServer) string {
			return spec.Name
		})
	case toolspkg.ToolResourceKind:
		return decodeExtensionResourceName(ctx, codecs, record, func(spec toolspkg.Tool) string {
			return spec.ID.String()
		})
	case windowmanager.WindowLayoutResourceKind:
		return extensionLayoutResourceNameFromID(record)
	default:
		return extensionResourceNameFromID(record)
	}
}

func extensionLayoutResourceNameFromID(record resources.RawRecord) string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(record.ID), "/"), "/")
	for index := len(parts) - 2; index > 0; index-- {
		switch parts[index] {
		case "profile", "workspace", "workspace_profile":
			return strings.TrimSpace(parts[index-1])
		}
	}
	return extensionResourceNameFromID(record)
}

func decodeExtensionResourceName[T any](
	ctx context.Context,
	codecs *resources.CodecRegistry,
	record resources.RawRecord,
	name func(T) string,
) string {
	if ctx != nil && codecs != nil {
		codec, err := resources.ResolveCodec[T](codecs, record.Kind)
		if err == nil {
			spec, decodeErr := codec.DecodeAndValidate(ctx, record.Scope, record.SpecJSON)
			if decodeErr == nil {
				if resolved := strings.TrimSpace(name(spec)); resolved != "" {
					return resolved
				}
			}
		}
	}
	return extensionResourceNameFromID(record)
}

func extensionResourceNameFromID(record resources.RawRecord) string {
	parts := strings.Split(strings.Trim(strings.TrimSpace(record.ID), "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	if (record.Kind == soul.ResourceKind || record.Kind == heartbeat.ResourceKind) && len(parts) > 1 {
		return strings.TrimSpace(parts[len(parts)-2])
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

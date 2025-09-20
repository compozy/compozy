package uc

import (
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/workflow"
)

func ensureNonNilBody(body map[string]any) error {
	if body == nil {
		return ErrInvalidPayload
	}
	return nil
}

func ensureNoProjectField(body map[string]any) error {
	if v, ok := body["project"]; ok && v != nil {
		return ErrProjectInBody
	}
	return nil
}

func readID(body map[string]any) (string, bool, error) {
	raw, ok := body["id"]
	if !ok {
		return "", false, nil
	}
	s, ok2 := raw.(string)
	if !ok2 {
		return "", true, ErrInvalidPayload
	}
	return strings.TrimSpace(s), true, nil
}

func ensureTypeMatches(typ resources.ResourceType, body map[string]any) error {
	if t, ok := body["type"].(string); ok {
		if strings.TrimSpace(t) != string(typ) {
			return ErrTypeMismatch
		}
	}
	return nil
}

func ensureValidID(id, pathID string, requireID bool) (string, error) {
	if requireID && id == "" {
		return "", ErrMissingID
	}
	if pathID != "" && id != "" && id != pathID {
		return "", ErrIDMismatch
	}
	if id != "" {
		if strings.ContainsAny(id, " \t\n\r") || strings.HasPrefix(id, "/") {
			return "", ErrInvalidID
		}
	} else {
		id = pathID
	}
	return id, nil
}

func validateBody(typ resources.ResourceType, body map[string]any, pathID string, requireID bool) (string, error) {
	if err := ensureNonNilBody(body); err != nil {
		return "", err
	}
	if err := ensureNoProjectField(body); err != nil {
		return "", err
	}
	id, _, err := readID(body)
	if err != nil {
		return "", err
	}
	if err := ensureTypeMatches(typ, body); err != nil {
		return "", err
	}
	return ensureValidID(id, pathID, requireID)
}

type validator func(map[string]any) error

var typeValidators = map[resources.ResourceType]validator{
	resources.ResourceAgent: func(body map[string]any) error {
		var cfg agent.Config
		if err := cfg.FromMap(body); err != nil {
			return ErrInvalidPayload
		}
		return nil
	},
	resources.ResourceTool: func(body map[string]any) error {
		var cfg tool.Config
		if err := cfg.FromMap(body); err != nil {
			return ErrInvalidPayload
		}
		return nil
	},
	resources.ResourceMCP: func(body map[string]any) error {
		if _, err := core.FromMapDefault[*mcp.Config](body); err != nil {
			return ErrInvalidPayload
		}
		return nil
	},
	resources.ResourceWorkflow: func(body map[string]any) error {
		if _, err := core.FromMapDefault[*workflow.Config](body); err != nil {
			return ErrInvalidPayload
		}
		return nil
	},
	resources.ResourceProject: func(body map[string]any) error {
		if _, err := core.FromMapDefault[*project.Config](body); err != nil {
			return ErrInvalidPayload
		}
		return nil
	},
	resources.ResourceMemory: func(body map[string]any) error {
		cfg, err := core.FromMapDefault[*memory.Config](body)
		if err != nil {
			return ErrInvalidPayload
		}
		if err := cfg.Validate(); err != nil {
			return ErrInvalidPayload
		}
		return nil
	},
	resources.ResourceSchema: func(body map[string]any) error {
		sc := schema.Schema(body)
		if _, err := sc.Compile(); err != nil {
			return ErrInvalidPayload
		}
		return nil
	},
	resources.ResourceModel: func(body map[string]any) error {
		cfg, err := core.FromMapDefault[*core.ProviderConfig](body)
		if err != nil {
			return ErrInvalidPayload
		}
		if cfg.Provider == "" || cfg.Model == "" {
			return ErrInvalidPayload
		}
		return nil
	},
}

func validateTypedResource(typ resources.ResourceType, body map[string]any) error {
	if v, ok := typeValidators[typ]; ok {
		return v(body)
	}
	return nil
}

package loop

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
)

func renderNodeParams(node dsl.Node, namespace map[string]any) (map[string]any, error) {
	return renderNodeParamsExcept(node, namespace, nil)
}

func renderHarvestSpec(node dsl.Node, namespace map[string]any) (*dsl.HarvestSpec, error) {
	if node.Harvest == nil {
		return nil, nil
	}
	rendered := *node.Harvest
	responder, err := refs.RenderTemplateString(
		fmt.Sprintf("nodes.%s.harvest.responder", node.ID),
		rendered.Responder,
		namespace,
	)
	if err != nil {
		return nil, err
	}
	contentRule, err := refs.RenderTemplateString(
		fmt.Sprintf("nodes.%s.harvest.content_rule", node.ID),
		rendered.ContentRule,
		namespace,
	)
	if err != nil {
		return nil, err
	}
	rendered.Responder = responder
	rendered.ContentRule = contentRule
	return &rendered, nil
}

func renderNodeParamsExcept(
	node dsl.Node,
	namespace map[string]any,
	rawKeys map[string]struct{},
) (map[string]any, error) {
	params := map[string]any(node.Params)
	if len(params) == 0 {
		return map[string]any{}, nil
	}
	out := make(map[string]any, len(params))
	for key, value := range params {
		if _, raw := rawKeys[key]; raw {
			normalized, err := normalizeJSONValue(value)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
			continue
		}
		rendered, err := renderAny(fmt.Sprintf("nodes.%s.params.%s", node.ID, key), value, namespace)
		if err != nil {
			return nil, err
		}
		out[key] = rendered
	}
	return out, nil
}

func normalizeNodeParams(params dsl.NodeParams) (map[string]any, error) {
	normalized, err := normalizeJSONValue(map[string]any(params))
	if err != nil {
		return nil, err
	}
	out, ok := normalized.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: node params must normalize to an object", ErrValidation)
	}
	return out, nil
}

func renderAny(name string, value any, namespace map[string]any) (any, error) {
	normalized, err := normalizeJSONValue(value)
	if err != nil {
		return nil, err
	}
	switch typed := normalized.(type) {
	case string:
		return refs.RenderTemplateString(name, typed, namespace)
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			rendered, err := renderAny(name+"."+key, child, namespace)
			if err != nil {
				return nil, err
			}
			out[key] = rendered
		}
		return out, nil
	case []any:
		out := make([]any, 0, len(typed))
		for idx, child := range typed {
			rendered, err := renderAny(name+"["+strconv.Itoa(idx)+"]", child, namespace)
			if err != nil {
				return nil, err
			}
			out = append(out, rendered)
		}
		return out, nil
	default:
		return typed, nil
	}
}

func normalizeJSONValue(value any) (any, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case dsl.NodeParams:
		return normalizeJSONValue(map[string]any(typed))
	case dsl.Schema:
		return normalizeJSONValue(map[string]any(typed))
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			normalized, err := normalizeJSONValue(child)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			keyString, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("%w: map key %v is not a string", ErrValidation, key)
			}
			normalized, err := normalizeJSONValue(child)
			if err != nil {
				return nil, err
			}
			out[keyString] = normalized
		}
		return out, nil
	case []any:
		out := make([]any, 0, len(typed))
		for _, child := range typed {
			normalized, err := normalizeJSONValue(child)
			if err != nil {
				return nil, err
			}
			out = append(out, normalized)
		}
		return out, nil
	default:
		return typed, nil
	}
}

func evaluateTransformMapping(key string, mapping dsl.TransformMapping, namespace map[string]any) (any, error) {
	if from := strings.TrimSpace(mapping.From); from != "" {
		value, err := namespacePathValue(namespace, from)
		if err != nil {
			return nil, fmt.Errorf("transform map %q from %q: %w", key, from, err)
		}
		return value, nil
	}
	if templateSource := strings.TrimSpace(mapping.Template); templateSource != "" {
		return refs.RenderTemplateString("transform."+key, mapping.Template, namespace)
	}
	normalized, err := normalizeJSONValue(mapping.Value)
	if err != nil {
		return nil, fmt.Errorf("transform map %q value: %w", key, err)
	}
	return normalized, nil
}

func namespacePathValue(namespace map[string]any, path string) (any, error) {
	segments := strings.Split(strings.TrimPrefix(strings.TrimSpace(path), "."), ".")
	var current any = namespace
	for _, segment := range segments {
		if segment == "" {
			return nil, fmt.Errorf("%w: empty path segment", ErrValidation)
		}
		switch typed := current.(type) {
		case map[string]any:
			value, ok := typed[segment]
			if !ok {
				return nil, fmt.Errorf("%w: missing path segment %q", ErrValidation, segment)
			}
			current = value
		case map[any]any:
			value, ok := typed[segment]
			if !ok {
				return nil, fmt.Errorf("%w: missing path segment %q", ErrValidation, segment)
			}
			current = value
		default:
			return nil, fmt.Errorf("%w: path segment %q cannot read from %T", ErrValidation, segment, current)
		}
	}
	return normalizeJSONValue(current)
}

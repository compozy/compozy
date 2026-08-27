package loop

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

// ItemRuntimeFromNamespace extracts runtime-selection metadata and the authored node/input layer.
func ItemRuntimeFromNamespace(
	namespace map[string]any,
	params dsl.NodeParams,
	materialized RuntimeSpec,
) (ItemRuntime, error) {
	node, input, err := materializedNodeRuntime(params, materialized)
	if err != nil {
		return ItemRuntime{}, fmt.Errorf("%w: params.runtime: %w", ErrValidation, err)
	}
	item := ItemRuntime{Node: node, Input: input}
	raw, ok := namespace["item"]
	if !ok {
		return item, nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return item, nil
	}
	// Imported task items have a stable path/body envelope. Arbitrary fan-out objects
	// must not opt into task rule matching merely because they have an id.
	if _, hasPath := values["path"]; !hasPath {
		return item, nil
	}
	item.TaskID = runtimeItemString(values, "id")
	item.TaskType = runtimeItemString(values, "type")
	item.Complexity = runtimeItemString(values, "complexity")
	if rawRuntime, exists := values["runtime"]; exists && rawRuntime != nil {
		runtime, err := runtimeSpecFromValue(rawRuntime)
		if err != nil {
			return ItemRuntime{}, fmt.Errorf("%w: item.runtime: %w", ErrValidation, err)
		}
		item.Frontmatter = runtime
	}
	return item, nil
}

func runtimeItemString(values map[string]any, key string) string {
	value, ok := values[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func runtimeSpecFromValue(value any) (RuntimeSpec, error) {
	runtime, err := runtimeInputSpec(value)
	if err != nil {
		return RuntimeSpec{}, fmt.Errorf("unknown runtime value: %w", err)
	}
	return runtime, nil
}

func appliedResolvedRuntime(
	intent ResolvedRuntime,
	applied RuntimeSpec,
	speedResolution *speedpkg.Resolution,
) ResolvedRuntime {
	resolved := normalizeResolvedRuntime(ResolvedRuntime{
		Runtime: applied, Source: intent.Source, SpeedResolution: speedResolution,
	})
	if strings.TrimSpace(intent.Runtime.Provider) == "" && resolved.Runtime.Provider != "" {
		resolved.Source.Provider = RuntimeSourceAgent
	}
	if strings.TrimSpace(intent.Runtime.Model) == "" && resolved.Runtime.Model != "" {
		resolved.Source.Model = RuntimeSourceAgent
	}
	if strings.TrimSpace(intent.Runtime.Reasoning) == "" && resolved.Runtime.Reasoning != "" {
		resolved.Source.Reasoning = RuntimeSourceAgent
	}
	if strings.TrimSpace(string(intent.Runtime.Speed)) == "" && resolved.Runtime.Speed != "" {
		resolved.Source.Speed = RuntimeSourceAgent
	}
	if len(intent.Runtime.ACPOptions) == 0 && len(resolved.Runtime.ACPOptions) > 0 {
		resolved.Source.ACPOptions = make(map[string]RuntimeSource, len(resolved.Runtime.ACPOptions))
		for _, option := range resolved.Runtime.ACPOptions {
			resolved.Source.ACPOptions[option.ID] = RuntimeSourceAgent
		}
	}
	return resolved
}

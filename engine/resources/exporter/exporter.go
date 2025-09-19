package exporter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"time"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"gopkg.in/yaml.v3"
)

// DirForType maps a ResourceType to a repository subdirectory name
func DirForType(t resources.ResourceType) (string, bool) {
	switch t {
	case resources.ResourceWorkflow:
		return "workflows", true
	case resources.ResourceAgent:
		return "agents", true
	case resources.ResourceTool:
		return "tools", true
	case resources.ResourceSchema:
		return "schemas", true
	case resources.ResourceMCP:
		return "mcps", true
	case resources.ResourceModel:
		return "models", true
	default:
		return "", false
	}
}

// Result summarizes export operation
type Result struct {
	Written map[resources.ResourceType]int
}

// ExportToDir walks the ResourceStore for the given project and writes
// deterministic YAML files under rootDir per type directory.
func ExportToDir(ctx context.Context, project string, store resources.ResourceStore, rootDir string) (*Result, error) {
	_ = config.FromContext(ctx)
	log := logger.FromContext(ctx)
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	if store == nil {
		return nil, fmt.Errorf("resource store is required")
	}
	if rootDir == "" {
		return nil, fmt.Errorf("root directory is required")
	}
	types := []resources.ResourceType{
		resources.ResourceWorkflow,
		resources.ResourceAgent,
		resources.ResourceTool,
		resources.ResourceSchema,
		resources.ResourceMCP,
		resources.ResourceModel,
	}
	res := &Result{Written: make(map[resources.ResourceType]int)}
	for _, typ := range types {
		dir, ok := DirForType(typ)
		if !ok {
			continue
		}
		keys, err := store.List(ctx, project, typ)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", typ, err)
		}
		if len(keys) == 0 {
			continue
		}
		absDir := filepath.Join(rootDir, dir)
		if err := os.MkdirAll(absDir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", absDir, err)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
		for i := range keys {
			key := keys[i]
			val, _, err := store.Get(ctx, key)
			if err != nil {
				if err == resources.ErrNotFound {
					continue
				}
				return nil, fmt.Errorf("get %s/%s: %w", string(typ), key.ID, err)
			}
			node := buildYAMLNode(val)
			var buf bytes.Buffer
			enc := yaml.NewEncoder(&buf)
			enc.SetIndent(2)
			if err := enc.Encode(node); err != nil {
				return nil, fmt.Errorf("encode yaml for %s/%s: %w", string(typ), key.ID, err)
			}
			_ = enc.Close()
			filename := filepath.Join(absDir, sanitizeID(key.ID)+".yaml")
			if err := os.WriteFile(filename, buf.Bytes(), 0o600); err != nil {
				return nil, fmt.Errorf("write file %s: %w", filename, err)
			}
			res.Written[typ]++
			log.Debug("exported yaml", "type", string(typ), "id", key.ID, "file", filename)
		}
	}
	return res, nil
}

// buildYAMLNode constructs a yaml.Node tree from a generic value with
// lexicographically sorted mapping keys to ensure deterministic output.
func buildYAMLNode(v any) *yaml.Node {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		n := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range keys {
			n.Content = append(n.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k},
				buildYAMLNode(t[k]),
			)
		}
		return n
	case []any:
		n := &yaml.Node{Kind: yaml.SequenceNode}
		for i := range t {
			n.Content = append(n.Content, buildYAMLNode(t[i]))
		}
		return n
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: t}
	case bool:
		if t {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "true"}
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: "false"}
	case int, int32, int64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprintf("%v", t)}
	case float32, float64:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!float", Value: fmt.Sprintf("%v", t)}
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}
	default:
		// Handle common complex types explicitly when possible
		if tm, ok := t.(time.Time); ok {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: tm.Format(time.RFC3339Nano)}
		}
		// Fallback: rely on fmt and let encoder infer
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprintf("%v", t)}
	}
}

func sanitizeID(id string) string {
	out := make([]rune, 0, len(id))
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			out = append(out, r)
		} else {
			out = append(out, '-')
		}
	}
	return string(out)
}

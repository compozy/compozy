package exporter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/compozy/compozy/engine/resources"
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

// buildYAMLNode marshals any Go value to a yaml.Node and canonicalizes mapping key order.
func buildYAMLNode(v any) *yaml.Node {
	var n yaml.Node
	b, err := yaml.Marshal(v)
	if err != nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprintf("marshal-error: %v", err)}
	}
	if err := yaml.Unmarshal(b, &n); err != nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: fmt.Sprintf("unmarshal-error: %v", err)}
	}
	canonicalizeYAML(&n)
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0]
	}
	return &n
}

// canonicalizeYAML sorts mapping node keys lexicographically, recursively.
func canonicalizeYAML(n *yaml.Node) {
	if n == nil || len(n.Content) == 0 {
		return
	}
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			canonicalizeYAML(c)
		}
	case yaml.MappingNode:
		type kv struct{ k, v *yaml.Node }
		pairs := make([]kv, 0, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			pairs = append(pairs, kv{n.Content[i], n.Content[i+1]})
		}
		sort.Slice(pairs, func(i, j int) bool { return pairs[i].k.Value < pairs[j].k.Value })
		n.Content = n.Content[:0]
		for _, p := range pairs {
			canonicalizeYAML(p.v)
			n.Content = append(n.Content, p.k, p.v)
		}
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

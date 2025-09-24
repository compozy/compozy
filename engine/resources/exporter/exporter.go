package exporter

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/pkg/logger"
	"gopkg.in/yaml.v3"
)

// Result summarizes export operation
type Result struct {
	Written map[resources.ResourceType]int
}

const exportedFileMode os.FileMode = 0o644

// ExportToDir walks the ResourceStore for the given project and writes
// deterministic YAML files under rootDir per type directory.
func ExportToDir(ctx context.Context, project string, store resources.ResourceStore, rootDir string) (*Result, error) {
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
		resources.ResourceTask,
		resources.ResourceSchema,
		resources.ResourceMCP,
		resources.ResourceModel,
		resources.ResourceMemory,
		resources.ResourceProject,
	}
	res := &Result{Written: make(map[resources.ResourceType]int)}
	for _, typ := range types {
		out, err := ExportTypeToDir(ctx, project, store, rootDir, typ)
		if err != nil {
			return nil, err
		}
		for k, v := range out.Written {
			res.Written[k] += v
		}
	}
	return res, nil
}

// ExportTypeToDir writes YAML files for a single resource type under its directory.
func ExportTypeToDir(
	ctx context.Context,
	project string,
	store resources.ResourceStore,
	rootDir string,
	typ resources.ResourceType,
) (*Result, error) {
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
	dir, ok := resources.DirForType(typ)
	if !ok {
		return nil, fmt.Errorf("unsupported resource type: %s", typ)
	}
	keys, err := store.List(ctx, project, typ)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", typ, err)
	}
	res := &Result{Written: make(map[resources.ResourceType]int)}
	if len(keys) == 0 {
		return res, nil
	}
	absDir := filepath.Join(rootDir, dir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", absDir, err)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
	used := make(map[string]string, len(keys))
	for i := range keys {
		written, err := exportResourceToFile(ctx, log, store, typ, keys[i], absDir, used)
		if err != nil {
			return nil, err
		}
		if written {
			res.Written[typ]++
		}
	}
	return res, nil
}

func exportResourceToFile(
	ctx context.Context,
	log logger.Logger,
	store resources.ResourceStore,
	typ resources.ResourceType,
	key resources.ResourceKey,
	absDir string,
	used map[string]string,
) (bool, error) {
	val, _, getErr := store.Get(ctx, key)
	if getErr != nil {
		if errors.Is(getErr, resources.ErrNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("get %s/%s: %w", string(typ), key.ID, getErr)
	}
	node := buildYAMLNode(val)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		if cerr := enc.Close(); cerr != nil {
			log.Warn("yaml encoder close after encode error", "type", string(typ), "id", key.ID, "err", cerr)
		}
		return false, fmt.Errorf("encode yaml for %s/%s: %w", string(typ), key.ID, err)
	}
	if cerr := enc.Close(); cerr != nil {
		return false, fmt.Errorf("close yaml encoder for %s/%s: %w", string(typ), key.ID, cerr)
	}
	sid := sanitizeID(key.ID)
	if prev, ok := used[sid]; ok {
		return false, fmt.Errorf("sanitized id collision: %q and %q both map to %q", prev, key.ID, sid)
	}
	used[sid] = key.ID
	filename := filepath.Join(absDir, sid+".yaml")
	if err := os.WriteFile(filename, buf.Bytes(), exportedFileMode); err != nil {
		return false, fmt.Errorf("write file %s: %w", filename, err)
	}
	log.Debug("exported yaml", "type", string(typ), "id", key.ID, "file", filename)
	return true, nil
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
	clean := strings.TrimLeft(string(out), ".")
	if clean == "" {
		return "-"
	}
	return clean
}

package importer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/resources/uc"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"gopkg.in/yaml.v3"
)

// Strategy defines import conflict behavior
type Strategy string

const (
	SeedOnly           Strategy = "seed_only"
	OverwriteConflicts Strategy = "overwrite_conflicts"
)

// Result summarizes import operation
type Result struct {
	Imported    map[resources.ResourceType]int
	Skipped     map[resources.ResourceType]int
	Overwritten map[resources.ResourceType]int
}

func dirToType(dir string) (resources.ResourceType, bool) {
	switch dir {
	case "workflows":
		return resources.ResourceWorkflow, true
	case "agents":
		return resources.ResourceAgent, true
	case "tools":
		return resources.ResourceTool, true
	case "schemas":
		return resources.ResourceSchema, true
	case "mcps":
		return resources.ResourceMCP, true
	case "models":
		return resources.ResourceModel, true
	default:
		return "", false
	}
}

// ImportFromDir reads YAML files under the well-known type directories in rootDir
// and upserts them into the store following the provided strategy.
func ImportFromDir(
	ctx context.Context,
	project string,
	store resources.ResourceStore,
	rootDir string,
	strategy Strategy,
	updatedBy string,
) (*Result, error) {
	_ = config.FromContext(ctx)
	if project == "" {
		return nil, fmt.Errorf("project is required")
	}
	if store == nil {
		return nil, fmt.Errorf("resource store is required")
	}
	if rootDir == "" {
		return nil, fmt.Errorf("root directory is required")
	}
	res := &Result{
		Imported:    map[resources.ResourceType]int{},
		Skipped:     map[resources.ResourceType]int{},
		Overwritten: map[resources.ResourceType]int{},
	}
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("read root directory: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		typ, ok := dirToType(e.Name())
		if !ok {
			continue
		}
		dir := filepath.Join(rootDir, e.Name())
		files, err := listYAMLFiles(dir)
		if err != nil {
			return nil, err
		}
		bodies, ids, err := parseTypeFiles(files)
		if err != nil {
			return nil, err
		}
		imp, skp, owr, err := applyForType(
			ctx,
			store,
			project,
			typ,
			bodies,
			ids,
			strategy,
			strings.TrimSpace(updatedBy),
		)
		if err != nil {
			return nil, err
		}
		res.Imported[typ] += imp
		res.Skipped[typ] += skp
		res.Overwritten[typ] += owr
	}
	return res, nil
}

func listYAMLFiles(dir string) ([]string, error) {
	files := make([]string, 0, 16)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

func parseTypeFiles(files []string) ([]map[string]any, []string, error) {
	parsed := make([]map[string]any, 0, len(files))
	ids := make([]string, 0, len(files))
	for _, f := range files {
		body, id, err := parseYAMLFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", f, err)
		}
		if id == "" {
			return nil, nil, fmt.Errorf("file %s missing id field", f)
		}
		ids = append(ids, id)
		parsed = append(parsed, body)
	}
	return parsed, ids, nil
}

func applyForType(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	typ resources.ResourceType,
	bodies []map[string]any,
	ids []string,
	strategy Strategy,
	updatedBy string,
) (imported int, skipped int, overwritten int, err error) {
	log := logger.FromContext(ctx)
	createUC := uc.NewCreateResource(store)
	upsertUC := uc.NewUpsertResource(store)
	for i := range bodies {
		id := ids[i]
		b := bodies[i]
		key := resources.ResourceKey{Project: project, Type: typ, ID: id}
		switch strategy {
		case SeedOnly:
			if _, _, err := store.Get(ctx, key); err == nil {
				skipped++
				continue
			} else if err != nil && !errors.Is(err, resources.ErrNotFound) {
				return 0, 0, 0, fmt.Errorf("get existing %s/%s: %w", string(typ), id, err)
			}
			if _, err := createUC.Execute(ctx, &uc.CreateInput{Project: project, Type: typ, Body: b}); err != nil {
				return 0, 0, 0, fmt.Errorf("create %s/%s: %w", string(typ), id, err)
			}
			imported++
		case OverwriteConflicts:
			prev, etag, err := store.Get(ctx, key)
			switch {
			case err == nil:
				if deepEqual(prev, b) {
					skipped++
					continue
				}
				if _, err := upsertUC.Execute(
					ctx,
					&uc.UpsertInput{Project: project, Type: typ, ID: id, Body: b, IfMatch: etag},
				); err != nil {
					return 0, 0, 0, fmt.Errorf("upsert %s/%s: %w", string(typ), id, err)
				}
				overwritten++
			case errors.Is(err, resources.ErrNotFound):
				if _, err := createUC.Execute(ctx, &uc.CreateInput{Project: project, Type: typ, Body: b}); err != nil {
					return 0, 0, 0, fmt.Errorf("create %s/%s: %w", string(typ), id, err)
				}
				imported++
			default:
				return 0, 0, 0, fmt.Errorf("get existing %s/%s: %w", string(typ), id, err)
			}
		default:
			return 0, 0, 0, fmt.Errorf("unknown strategy: %s", strategy)
		}
		metaID := project + ":" + string(typ) + ":" + id
		meta := map[string]any{
			"source":     "yaml",
			"updated_at": time.Now().UTC().Format(time.RFC3339),
			"updated_by": updatedBy,
		}
		if _, err := store.Put(
			ctx,
			resources.ResourceKey{Project: project, Type: resources.ResourceMeta, ID: metaID},
			meta,
		); err != nil {
			log.Warn("failed to write meta record", "error", err, "id", metaID)
		}
	}
	return imported, skipped, overwritten, nil
}

func parseYAMLFile(path string) (map[string]any, string, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var m map[string]any
	if err := yaml.Unmarshal(bs, &m); err != nil {
		return nil, "", err
	}
	id := ""
	if v, ok := m["id"].(string); ok {
		id = strings.TrimSpace(v)
	}
	return m, id, nil
}

// deepEqual is a conservative equality check for map[string]any and slices.
func deepEqual(a, b any) bool {
	switch at := a.(type) {
	case map[string]any:
		bt, ok := b.(map[string]any)
		if !ok {
			return false
		}
		if len(at) != len(bt) {
			return false
		}
		keys := make([]string, 0, len(at))
		for k := range at {
			keys = append(keys, k)
		}
		for _, k := range keys {
			if !deepEqual(at[k], bt[k]) {
				return false
			}
		}
		return true
	case []any:
		bs, ok := b.([]any)
		if !ok {
			return false
		}
		if len(at) != len(bs) {
			return false
		}
		for i := range at {
			if !deepEqual(at[i], bs[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

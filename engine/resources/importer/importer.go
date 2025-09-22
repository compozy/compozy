package importer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	agentuc "github.com/compozy/compozy/engine/agent/uc"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/userctx"
	"github.com/compozy/compozy/engine/core"
	mcpuc "github.com/compozy/compozy/engine/mcp/uc"
	memoryuc "github.com/compozy/compozy/engine/memoryconfig/uc"
	modeluc "github.com/compozy/compozy/engine/model/uc"
	projectuc "github.com/compozy/compozy/engine/project/uc"
	"github.com/compozy/compozy/engine/resources"
	schemauc "github.com/compozy/compozy/engine/schema/uc"
	tooluc "github.com/compozy/compozy/engine/tool/uc"
	wfuc "github.com/compozy/compozy/engine/workflow/uc"
	"gopkg.in/yaml.v3"
)

// Strategy defines import conflict behavior
type Strategy string

const (
	SeedOnly           Strategy = "seed_only"
	OverwriteConflicts Strategy = "overwrite_conflicts"
)

// well-known directory names
const (
	dirWorkflows = "workflows"
	dirAgents    = "agents"
	dirTools     = "tools"
	dirSchemas   = "schemas"
	dirMCPs      = "mcps"
	dirModels    = "models"
	dirMemories  = "memories"
	dirProject   = "project"
)

// Result summarizes import operation
type Result struct {
	Imported    map[resources.ResourceType]int
	Skipped     map[resources.ResourceType]int
	Overwritten map[resources.ResourceType]int
}

func dirToType(dir string) (resources.ResourceType, bool) {
	switch dir {
	case dirWorkflows:
		return resources.ResourceWorkflow, true
	case dirAgents:
		return resources.ResourceAgent, true
	case dirTools:
		return resources.ResourceTool, true
	case dirSchemas:
		return resources.ResourceSchema, true
	case dirMCPs:
		return resources.ResourceMCP, true
	case dirModels:
		return resources.ResourceModel, true
	case dirMemories:
		return resources.ResourceMemory, true
	case dirProject:
		return resources.ResourceProject, true
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
	trimmedUpdatedBy := strings.TrimSpace(updatedBy)
	if trimmedUpdatedBy != "" {
		ctx = userctx.WithUser(ctx, &model.User{ID: core.ID(trimmedUpdatedBy)})
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
	seen := make(map[string]string, len(files))
	for _, f := range files {
		body, id, err := parseYAMLFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", f, err)
		}
		if id == "" {
			return nil, nil, fmt.Errorf("file %s missing id field", f)
		}
		if prev, ok := seen[id]; ok {
			return nil, nil, fmt.Errorf("duplicate id '%s' in %s (first seen in %s)", id, f, prev)
		}
		seen[id] = f
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
) (imported int, skipped int, overwritten int, err error) {
	for i := range bodies {
		id := ids[i]
		body := bodies[i]
		key := resources.ResourceKey{Project: project, Type: typ, ID: id}
		switch strategy {
		case SeedOnly:
			if _, _, err := store.Get(ctx, key); err == nil {
				skipped++
				continue
			} else if err != nil && !errors.Is(err, resources.ErrNotFound) {
				return 0, 0, 0, fmt.Errorf("get existing %s/%s: %w", string(typ), id, err)
			}
			if _, _, err := upsertResource(ctx, store, typ, project, id, "", body); err != nil {
				return 0, 0, 0, fmt.Errorf("create %s/%s: %w", string(typ), id, err)
			}
			imported++
		case OverwriteConflicts:
			_, existingETag, err := store.Get(ctx, key)
			if err != nil {
				if errors.Is(err, resources.ErrNotFound) {
					if _, _, err := upsertResource(ctx, store, typ, project, id, "", body); err != nil {
						return 0, 0, 0, fmt.Errorf("create %s/%s: %w", string(typ), id, err)
					}
					imported++
					continue
				}
				return 0, 0, 0, fmt.Errorf("get existing %s/%s: %w", string(typ), id, err)
			}
			newETag, created, err := upsertResource(
				ctx,
				store,
				typ,
				project,
				id,
				string(existingETag),
				body,
			)
			if err != nil {
				return 0, 0, 0, fmt.Errorf("upsert %s/%s: %w", string(typ), id, err)
			}
			if created {
				imported++
				continue
			}
			if newETag == existingETag {
				skipped++
			} else {
				overwritten++
			}
		default:
			return 0, 0, 0, fmt.Errorf("unknown strategy: %s", strategy)
		}
	}
	return imported, skipped, overwritten, nil
}

type upsertHandler func(
	context.Context,
	resources.ResourceStore,
	string,
	string,
	string,
	map[string]any,
) (resources.ETag, bool, error)

var standardUpsertHandlers = map[resources.ResourceType]upsertHandler{
	resources.ResourceWorkflow: workflowUpsert,
	resources.ResourceAgent:    agentUpsert,
	resources.ResourceTool:     toolUpsert,
	resources.ResourceSchema:   schemaUpsert,
	resources.ResourceModel:    modelUpsert,
	resources.ResourceMemory:   memoryUpsert,
	resources.ResourceProject:  projectUpsert,
	resources.ResourceMCP:      mcpUpsert,
}

func upsertResource(
	ctx context.Context,
	store resources.ResourceStore,
	typ resources.ResourceType,
	project string,
	id string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	if handler, ok := standardUpsertHandlers[typ]; ok {
		return handler(ctx, store, project, id, ifMatch, body)
	}
	return "", false, fmt.Errorf("unsupported resource type: %s", typ)
}

func workflowUpsert(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	id string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	input := &wfuc.UpsertInput{
		Project: project,
		ID:      id,
		Body:    body,
		IfMatch: strings.TrimSpace(ifMatch),
	}
	out, err := wfuc.NewUpsert(store).Execute(ctx, input)
	if err != nil {
		return "", false, err
	}
	return out.ETag, out.Created, nil
}

func agentUpsert(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	id string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	input := &agentuc.UpsertInput{
		Project: project,
		ID:      id,
		Body:    body,
		IfMatch: strings.TrimSpace(ifMatch),
	}
	out, err := agentuc.NewUpsert(store).Execute(ctx, input)
	if err != nil {
		return "", false, err
	}
	return out.ETag, out.Created, nil
}

func toolUpsert(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	id string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	input := &tooluc.UpsertInput{
		Project: project,
		ID:      id,
		Body:    body,
		IfMatch: strings.TrimSpace(ifMatch),
	}
	out, err := tooluc.NewUpsert(store).Execute(ctx, input)
	if err != nil {
		return "", false, err
	}
	return out.ETag, out.Created, nil
}

func schemaUpsert(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	id string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	input := &schemauc.UpsertInput{
		Project: project,
		ID:      id,
		Body:    body,
		IfMatch: strings.TrimSpace(ifMatch),
	}
	out, err := schemauc.NewUpsert(store).Execute(ctx, input)
	if err != nil {
		return "", false, err
	}
	return out.ETag, out.Created, nil
}

func modelUpsert(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	id string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	input := &modeluc.UpsertInput{
		Project: project,
		ID:      id,
		Body:    body,
		IfMatch: strings.TrimSpace(ifMatch),
	}
	out, err := modeluc.NewUpsert(store).Execute(ctx, input)
	if err != nil {
		return "", false, err
	}
	return out.ETag, out.Created, nil
}

func memoryUpsert(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	id string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	input := &memoryuc.UpsertInput{
		Project: project,
		ID:      id,
		Body:    body,
		IfMatch: strings.TrimSpace(ifMatch),
	}
	out, err := memoryuc.NewUpsert(store).Execute(ctx, input)
	if err != nil {
		return "", false, err
	}
	return out.ETag, out.Created, nil
}

func projectUpsert(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	_ string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	input := &projectuc.UpsertInput{
		Project: project,
		Body:    body,
		IfMatch: strings.TrimSpace(ifMatch),
	}
	out, err := projectuc.NewUpsert(store).Execute(ctx, input)
	if err != nil {
		return "", false, err
	}
	return out.ETag, out.Created, nil
}

func mcpUpsert(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	id string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	input := &mcpuc.UpsertInput{
		Project: project,
		ID:      id,
		Body:    body,
		IfMatch: strings.TrimSpace(ifMatch),
	}
	out, err := mcpuc.NewUpsert(store).Execute(ctx, input)
	if err != nil {
		return "", false, err
	}
	return out.ETag, out.Created, nil
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

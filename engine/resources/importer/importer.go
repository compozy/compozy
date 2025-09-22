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
	taskuc "github.com/compozy/compozy/engine/task/uc"
	tooluc "github.com/compozy/compozy/engine/tool/uc"
	wfuc "github.com/compozy/compozy/engine/workflow/uc"
	"gopkg.in/yaml.v3"
)

// Strategy defines import conflict behavior
type Strategy string

// -----------------------------------------------------------------------------
// Well-known directory names
// -----------------------------------------------------------------------------
const (
	SeedOnly           Strategy = "seed_only"
	OverwriteConflicts Strategy = "overwrite_conflicts"
)

// well-known directory names
const (
	dirWorkflows = "workflows"
	dirAgents    = "agents"
	dirTools     = "tools"
	dirTasks     = "tasks"
	dirSchemas   = "schemas"
	dirMCPs      = "mcps"
	dirModels    = "models"
	dirMemories  = "memories"
	dirProject   = "project"
)

const defaultYAMLListCap = 16

// Result summarizes import operation
type Result struct {
	Imported    map[resources.ResourceType]int
	Skipped     map[resources.ResourceType]int
	Overwritten map[resources.ResourceType]int
}

func typeToDir(typ resources.ResourceType) (string, bool) {
	switch typ {
	case resources.ResourceWorkflow:
		return dirWorkflows, true
	case resources.ResourceAgent:
		return dirAgents, true
	case resources.ResourceTool:
		return dirTools, true
	case resources.ResourceTask:
		return dirTasks, true
	case resources.ResourceSchema:
		return dirSchemas, true
	case resources.ResourceMCP:
		return dirMCPs, true
	case resources.ResourceModel:
		return dirModels, true
	case resources.ResourceMemory:
		return dirMemories, true
	case resources.ResourceProject:
		return dirProject, true
	default:
		return "", false
	}
}

func newResult() *Result {
	return &Result{
		Imported:    map[resources.ResourceType]int{},
		Skipped:     map[resources.ResourceType]int{},
		Overwritten: map[resources.ResourceType]int{},
	}
}

func contextWithUpdatedBy(ctx context.Context, updatedBy string) context.Context {
	trimmed := strings.TrimSpace(updatedBy)
	if trimmed == "" {
		return ctx
	}
	return userctx.WithUser(ctx, &model.User{ID: core.ID(trimmed)})
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
	ctx = contextWithUpdatedBy(ctx, updatedBy)
	res := newResult()
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
	for _, typ := range types {
		out, err := ImportTypeFromDir(ctx, project, store, rootDir, strategy, "", typ)
		if err != nil {
			return nil, err
		}
		for k, v := range out.Imported {
			res.Imported[k] += v
		}
		for k, v := range out.Skipped {
			res.Skipped[k] += v
		}
		for k, v := range out.Overwritten {
			res.Overwritten[k] += v
		}
	}
	return res, nil
}

// ImportTypeFromDir reads YAML files for a specific resource type under rootDir.
func ImportTypeFromDir(
	ctx context.Context,
	project string,
	store resources.ResourceStore,
	rootDir string,
	strategy Strategy,
	updatedBy string,
	typ resources.ResourceType,
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
	dirName, ok := typeToDir(typ)
	if !ok {
		return nil, fmt.Errorf("unsupported resource type: %s", typ)
	}
	ctx = contextWithUpdatedBy(ctx, updatedBy)
	res := newResult()
	absDir := filepath.Join(rootDir, dirName)
	info, err := os.Stat(absDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
			return res, nil
		}
		return nil, fmt.Errorf("stat %s: %w", absDir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", absDir)
	}
	files, err := listYAMLFiles(absDir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return res, nil
	}
	bodies, ids, err := parseTypeFiles(files)
	if err != nil {
		return nil, err
	}
	imp, skp, owr, err := applyForType(ctx, store, project, typ, bodies, ids, strategy)
	if err != nil {
		return nil, err
	}
	res.Imported[typ] = imp
	res.Skipped[typ] = skp
	res.Overwritten[typ] = owr
	return res, nil
}

func listYAMLFiles(dir string) ([]string, error) {
	files := make([]string, 0, defaultYAMLListCap)
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
	switch strategy {
	case SeedOnly:
		return applySeedOnly(ctx, store, project, typ, bodies, ids)
	case OverwriteConflicts:
		return applyOverwriteConflicts(ctx, store, project, typ, bodies, ids)
	default:
		return 0, 0, 0, fmt.Errorf("unknown strategy: %s", strategy)
	}
}

func applySeedOnly(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	typ resources.ResourceType,
	bodies []map[string]any,
	ids []string,
) (imported int, skipped int, overwritten int, err error) {
	for i := range bodies {
		id := ids[i]
		body := bodies[i]
		key := resources.ResourceKey{Project: project, Type: typ, ID: id}
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
	}
	return imported, skipped, overwritten, nil
}

func applyOverwriteConflicts(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	typ resources.ResourceType,
	bodies []map[string]any,
	ids []string,
) (imported int, skipped int, overwritten int, err error) {
	for i := range bodies {
		id := ids[i]
		body := bodies[i]
		key := resources.ResourceKey{Project: project, Type: typ, ID: id}
		_, existingETag, getErr := store.Get(ctx, key)
		if getErr != nil {
			if errors.Is(getErr, resources.ErrNotFound) {
				if _, _, err := upsertResource(ctx, store, typ, project, id, "", body); err != nil {
					return 0, 0, 0, fmt.Errorf("create %s/%s: %w", string(typ), id, err)
				}
				imported++
				continue
			}
			return 0, 0, 0, fmt.Errorf("get existing %s/%s: %w", string(typ), id, getErr)
		}
		newETag, created, err := upsertResource(ctx, store, typ, project, id, string(existingETag), body)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("upsert %s/%s: %w", string(typ), id, err)
		}
		if created {
			imported++
			continue
		}
		if newETag == existingETag {
			skipped++
			continue
		}
		overwritten++
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
	resources.ResourceTask:     taskUpsert,
	resources.ResourceSchema:   schemaUpsert,
	resources.ResourceModel:    modelUpsert,
	resources.ResourceMemory:   memoryUpsert,
	resources.ResourceProject:  projectUpsert,
	resources.ResourceMCP:      mcpUpsert,
}

var _ = ensureStandardUpsertHandlers()

func ensureStandardUpsertHandlers() bool {
	required := []resources.ResourceType{
		resources.ResourceWorkflow,
		resources.ResourceAgent,
		resources.ResourceTool,
		resources.ResourceTask,
		resources.ResourceSchema,
		resources.ResourceModel,
		resources.ResourceMemory,
		resources.ResourceProject,
		resources.ResourceMCP,
	}
	for _, typ := range required {
		if _, ok := standardUpsertHandlers[typ]; !ok {
			panic(fmt.Sprintf("importer: missing standard handler for type %s", typ))
		}
	}
	return true
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
		return handler(ctx, store, project, id, strings.TrimSpace(ifMatch), body)
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
		IfMatch: ifMatch,
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
		IfMatch: ifMatch,
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
		IfMatch: ifMatch,
	}
	out, err := tooluc.NewUpsert(store).Execute(ctx, input)
	if err != nil {
		return "", false, err
	}
	return out.ETag, out.Created, nil
}

func taskUpsert(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	id string,
	ifMatch string,
	body map[string]any,
) (resources.ETag, bool, error) {
	input := &taskuc.UpsertInput{
		Project: project,
		ID:      id,
		Body:    body,
		IfMatch: ifMatch,
	}
	out, err := taskuc.NewUpsert(store).Execute(ctx, input)
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
		IfMatch: ifMatch,
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
		IfMatch: ifMatch,
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
		IfMatch: ifMatch,
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
		IfMatch: ifMatch,
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
		IfMatch: ifMatch,
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

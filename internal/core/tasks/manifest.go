package tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"gopkg.in/yaml.v3"
)

const (
	TaskGraphManifestFileName = "_tasks.md"
	TaskGraphManifestVersion  = "compozy.tasks/v2"
)

var (
	ErrTaskGraphManifestMissing = errors.New("task graph manifest missing")
	ErrTaskGraphManifestInvalid = errors.New("task graph manifest invalid")
)

type TaskGraphManifest struct {
	SchemaVersion string        `yaml:"schema_version"`
	Workflow      string        `yaml:"workflow"`
	Graph         TaskGraphSpec `yaml:"graph"`
	Path          string        `yaml:"-"`
	Body          string        `yaml:"-"`
}

type TaskGraphSpec struct {
	Nodes []TaskGraphNode `yaml:"nodes"`
	Edges []TaskGraphEdge `yaml:"edges"`
}

type TaskGraphNode struct {
	ID   string `yaml:"id"`
	File string `yaml:"file"`
}

type TaskGraphEdge struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

type TaskGraphTaskFile struct {
	ID      string
	File    string
	AbsPath string
	Number  int
	Entry   model.TaskEntry
}

type TaskGraphManifestValidationError struct {
	Issues []Issue
}

func (e *TaskGraphManifestValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return ErrTaskGraphManifestInvalid.Error()
	}
	first := e.Issues[0]
	return fmt.Sprintf("%s: %s %s: %s", ErrTaskGraphManifestInvalid, first.Path, first.Field, first.Message)
}

func (e *TaskGraphManifestValidationError) Unwrap() error {
	return ErrTaskGraphManifestInvalid
}

func ReadTaskGraphManifest(tasksDir string) (TaskGraphManifest, error) {
	resolvedDir, err := filepath.Abs(strings.TrimSpace(tasksDir))
	if err != nil {
		return TaskGraphManifest{}, fmt.Errorf("resolve tasks dir: %w", err)
	}
	path := filepath.Join(resolvedDir, TaskGraphManifestFileName)
	content, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return TaskGraphManifest{}, fmt.Errorf("%w: %s", ErrTaskGraphManifestMissing, path)
		}
		return TaskGraphManifest{}, fmt.Errorf("read task graph manifest %s: %w", path, err)
	}
	var manifest TaskGraphManifest
	body, err := frontmatter.Parse(string(content), &manifest)
	if err != nil {
		if errors.Is(err, frontmatter.ErrHeaderNotFound) {
			return TaskGraphManifest{}, fmt.Errorf("%w: %s", ErrTaskGraphManifestMissing, path)
		}
		return TaskGraphManifest{}, fmt.Errorf("parse task graph manifest %s: %w", path, err)
	}
	manifest.Path = path
	manifest.Body = body
	normalizeTaskGraphManifest(&manifest)
	return manifest, nil
}

func LoadValidatedTaskGraphManifest(
	ctx context.Context,
	tasksDir string,
	workflowSlug string,
) (TaskGraphManifest, []TaskGraphTaskFile, error) {
	manifest, err := ReadTaskGraphManifest(tasksDir)
	if err != nil {
		return TaskGraphManifest{}, nil, err
	}
	tasks, issues, err := ValidateTaskGraphManifest(ctx, tasksDir, workflowSlug, manifest)
	if err != nil {
		return TaskGraphManifest{}, nil, err
	}
	if len(issues) > 0 {
		return TaskGraphManifest{}, nil, &TaskGraphManifestValidationError{Issues: issues}
	}
	return manifest, tasks, nil
}

func ValidateTaskGraphManifest(
	ctx context.Context,
	tasksDir string,
	workflowSlug string,
	manifest TaskGraphManifest,
) ([]TaskGraphTaskFile, []Issue, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, nil, fmt.Errorf("validate task graph manifest: %w", err)
	}
	resolvedDir, err := filepath.Abs(strings.TrimSpace(tasksDir))
	if err != nil {
		return nil, nil, fmt.Errorf("resolve tasks dir: %w", err)
	}
	manifestPath := manifest.Path
	if strings.TrimSpace(manifestPath) == "" {
		manifestPath = filepath.Join(resolvedDir, TaskGraphManifestFileName)
	}
	workflow := strings.TrimSpace(workflowSlug)
	if workflow == "" {
		workflow = filepath.Base(resolvedDir)
	}

	issues := make([]Issue, 0)
	if manifest.SchemaVersion != TaskGraphManifestVersion {
		issues = append(issues, Issue{
			Path:    manifestPath,
			Field:   "schema_version",
			Message: fmt.Sprintf("schema_version must be %q", TaskGraphManifestVersion),
		})
	}
	if manifest.Workflow == "" {
		issues = append(issues, Issue{Path: manifestPath, Field: "workflow", Message: "workflow is required"})
	} else if workflow != "" && manifest.Workflow != workflow {
		issues = append(issues, Issue{
			Path:    manifestPath,
			Field:   "workflow",
			Message: fmt.Sprintf("workflow %q must match %q", manifest.Workflow, workflow),
		})
	}
	if len(manifest.Graph.Nodes) == 0 {
		issues = append(
			issues,
			Issue{Path: manifestPath, Field: "graph.nodes", Message: "at least one node is required"},
		)
	}

	taskFiles, nodeIssues, err := loadTaskGraphNodeFiles(ctx, resolvedDir, manifestPath, manifest.Graph.Nodes)
	if err != nil {
		return nil, nil, err
	}
	issues = append(issues, nodeIssues...)
	issues = append(issues, validateTaskGraphEdges(manifestPath, manifest.Graph.Nodes, manifest.Graph.Edges)...)
	if len(nodeIssues) == 0 {
		issues = append(issues, validateTaskGraphAcyclic(manifestPath, manifest.Graph.Nodes, manifest.Graph.Edges)...)
	}
	slices.SortStableFunc(issues, func(a, b Issue) int {
		if cmp := strings.Compare(a.Path, b.Path); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Field, b.Field)
	})
	return taskFiles, issues, nil
}

func normalizeTaskGraphManifest(manifest *TaskGraphManifest) {
	if manifest == nil {
		return
	}
	manifest.SchemaVersion = strings.TrimSpace(manifest.SchemaVersion)
	manifest.Workflow = strings.TrimSpace(manifest.Workflow)
	for idx := range manifest.Graph.Nodes {
		manifest.Graph.Nodes[idx].ID = strings.TrimSpace(manifest.Graph.Nodes[idx].ID)
		manifest.Graph.Nodes[idx].File = filepath.ToSlash(strings.TrimSpace(manifest.Graph.Nodes[idx].File))
	}
	for idx := range manifest.Graph.Edges {
		manifest.Graph.Edges[idx].From = strings.TrimSpace(manifest.Graph.Edges[idx].From)
		manifest.Graph.Edges[idx].To = strings.TrimSpace(manifest.Graph.Edges[idx].To)
	}
}

func loadTaskGraphNodeFiles(
	ctx context.Context,
	tasksDir string,
	manifestPath string,
	nodes []TaskGraphNode,
) ([]TaskGraphTaskFile, []Issue, error) {
	taskRoot, err := os.OpenRoot(tasksDir)
	if err != nil {
		return nil, nil, fmt.Errorf("open task graph root: %w", err)
	}
	defer taskRoot.Close()

	issues := make([]Issue, 0)
	files := make([]TaskGraphTaskFile, 0, len(nodes))
	state := newTaskGraphNodeValidationState(len(nodes))
	for index, node := range nodes {
		if err := context.Cause(ctx); err != nil {
			return nil, nil, fmt.Errorf("validate task graph manifest: %w", err)
		}
		fieldPrefix := fmt.Sprintf("graph.nodes[%d]", index)
		id, file, number, nodeIssues, skip := validateTaskGraphNodeRef(manifestPath, fieldPrefix, node, state)
		issues = append(issues, nodeIssues...)
		if skip {
			continue
		}
		taskFile, fileIssues, err := loadTaskGraphNodeFile(
			taskRoot,
			tasksDir,
			manifestPath,
			fieldPrefix,
			id,
			file,
			number,
		)
		if err != nil {
			return nil, nil, err
		}
		issues = append(issues, fileIssues...)
		if taskFile.AbsPath == "" {
			continue
		}
		files = append(files, taskFile)
	}
	return files, issues, nil
}

type taskGraphNodeValidationState struct {
	seenIDs   map[string]struct{}
	seenFiles map[string]string
}

func newTaskGraphNodeValidationState(size int) *taskGraphNodeValidationState {
	return &taskGraphNodeValidationState{
		seenIDs:   make(map[string]struct{}, size),
		seenFiles: make(map[string]string, size),
	}
}

func validateTaskGraphNodeRef(
	manifestPath string,
	fieldPrefix string,
	node TaskGraphNode,
	state *taskGraphNodeValidationState,
) (string, string, int, []Issue, bool) {
	id := strings.TrimSpace(node.ID)
	file := filepath.ToSlash(strings.TrimSpace(node.File))
	issues := validateTaskGraphNodeRequiredFields(manifestPath, fieldPrefix, id, file)
	if id == "" || file == "" {
		return id, file, 0, issues, true
	}
	issues = append(issues, validateTaskGraphNodeIdentity(manifestPath, fieldPrefix, id, file)...)
	issues = append(issues, validateTaskGraphNodeUniqueness(manifestPath, fieldPrefix, id, file, state)...)
	number := ExtractTaskNumber(filepath.Base(filepath.FromSlash(file)))
	if number <= 0 {
		issues = append(
			issues,
			Issue{Path: manifestPath, Field: fieldPrefix + ".file", Message: "node file must match task_NN.md"},
		)
	}
	return id, file, number, issues, false
}

func validateTaskGraphNodeRequiredFields(manifestPath string, fieldPrefix string, id string, file string) []Issue {
	issues := make([]Issue, 0, 2)
	if id == "" {
		issues = append(issues, Issue{Path: manifestPath, Field: fieldPrefix + ".id", Message: "node id is required"})
	}
	if file == "" {
		issues = append(
			issues,
			Issue{Path: manifestPath, Field: fieldPrefix + ".file", Message: "node file is required"},
		)
	}
	return issues
}

func validateTaskGraphNodeIdentity(manifestPath string, fieldPrefix string, id string, file string) []Issue {
	issues := make([]Issue, 0, 2)
	if got := TaskIdentityFromName(id); got != id {
		issues = append(issues, Issue{
			Path:    manifestPath,
			Field:   fieldPrefix + ".id",
			Message: fmt.Sprintf("node id %q must be canonical task_NN", id),
		})
	}
	if fileID := TaskIdentityFromName(file); fileID != id {
		issues = append(issues, Issue{
			Path:    manifestPath,
			Field:   fieldPrefix + ".file",
			Message: fmt.Sprintf("file %q must match node id %q", file, id),
		})
	}
	return issues
}

func validateTaskGraphNodeUniqueness(
	manifestPath string,
	fieldPrefix string,
	id string,
	file string,
	state *taskGraphNodeValidationState,
) []Issue {
	issues := make([]Issue, 0, 2)
	if _, exists := state.seenIDs[id]; exists {
		issues = append(issues, Issue{
			Path:    manifestPath,
			Field:   fieldPrefix + ".id",
			Message: fmt.Sprintf("duplicate node id %q", id),
		})
	} else {
		state.seenIDs[id] = struct{}{}
	}
	if previous, exists := state.seenFiles[file]; exists {
		issues = append(issues, Issue{
			Path:    manifestPath,
			Field:   fieldPrefix + ".file",
			Message: fmt.Sprintf("file %q is already assigned to %q", file, previous),
		})
	} else {
		state.seenFiles[file] = id
	}
	return issues
}

func loadTaskGraphNodeFile(
	taskRoot *os.Root,
	tasksDir string,
	manifestPath string,
	fieldPrefix string,
	id string,
	file string,
	number int,
) (TaskGraphTaskFile, []Issue, error) {
	absPath, err := resolveTaskGraphNodeFilePath(tasksDir, file)
	if err != nil {
		return TaskGraphTaskFile{}, []Issue{{
			Path:    manifestPath,
			Field:   fieldPrefix + ".file",
			Message: err.Error(),
		}}, nil
	}
	content, err := taskRoot.ReadFile(filepath.FromSlash(file))
	if err != nil {
		return TaskGraphTaskFile{}, []Issue{{
			Path:    absPath,
			Field:   "file",
			Message: fmt.Sprintf("task file is required: %v", err),
		}}, nil
	}
	entry, err := ParseTaskFile(string(content))
	if err != nil {
		return TaskGraphTaskFile{}, nil, WrapParseError(absPath, err)
	}
	issues := make([]Issue, 0, 1)
	if taskFrontMatterHasKey(string(content), "dependencies") {
		issues = append(issues, Issue{
			Path:    absPath,
			Field:   "dependencies",
			Message: "dependencies must live in _tasks.md graph.edges for schema compozy.tasks/v2",
		})
	}
	entry.ID = id
	return TaskGraphTaskFile{ID: id, File: file, AbsPath: absPath, Number: number, Entry: entry}, issues, nil
}

func resolveTaskGraphNodeFilePath(tasksDir string, file string) (string, error) {
	root, err := filepath.Abs(strings.TrimSpace(tasksDir))
	if err != nil {
		return "", fmt.Errorf("resolve task root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve task root: %w", err)
	}
	path := filepath.FromSlash(strings.TrimSpace(file))
	if filepath.IsAbs(path) {
		return "", errors.New("task node file must resolve within task root")
	}
	candidate := filepath.Join(root, path)
	if !pathIsContained(root, candidate) {
		return "", errors.New("task node file must resolve within task root")
	}
	if _, err := os.Lstat(candidate); err == nil {
		resolved, resolveErr := filepath.EvalSymlinks(candidate)
		if resolveErr != nil {
			return "", fmt.Errorf("resolve task node file: %w", resolveErr)
		}
		if !pathIsContained(resolvedRoot, resolved) {
			return "", errors.New("task node file must resolve within task root")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect task node file: %w", err)
	}
	return candidate, nil
}

func pathIsContained(root string, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(relative)
}

func validateTaskGraphEdges(manifestPath string, nodes []TaskGraphNode, edges []TaskGraphEdge) []Issue {
	issues := make([]Issue, 0)
	nodeSet := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		if node.ID != "" {
			nodeSet[node.ID] = struct{}{}
		}
	}
	seenEdges := make(map[string]struct{}, len(edges))
	for index, edge := range edges {
		fieldPrefix := fmt.Sprintf("graph.edges[%d]", index)
		from := strings.TrimSpace(edge.From)
		to := strings.TrimSpace(edge.To)
		if from == "" {
			issues = append(
				issues,
				Issue{Path: manifestPath, Field: fieldPrefix + ".from", Message: "edge source is required"},
			)
		}
		if to == "" {
			issues = append(
				issues,
				Issue{Path: manifestPath, Field: fieldPrefix + ".to", Message: "edge target is required"},
			)
		}
		if from == "" || to == "" {
			continue
		}
		if from == to {
			issues = append(
				issues,
				Issue{
					Path:    manifestPath,
					Field:   fieldPrefix,
					Message: fmt.Sprintf("self-edge %q -> %q is not allowed", from, to),
				},
			)
		}
		if _, ok := nodeSet[from]; !ok {
			issues = append(
				issues,
				Issue{
					Path:    manifestPath,
					Field:   fieldPrefix + ".from",
					Message: fmt.Sprintf("edge source %q is not a graph node", from),
				},
			)
		}
		if _, ok := nodeSet[to]; !ok {
			issues = append(
				issues,
				Issue{
					Path:    manifestPath,
					Field:   fieldPrefix + ".to",
					Message: fmt.Sprintf("edge target %q is not a graph node", to),
				},
			)
		}
		key := from + "\x00" + to
		if _, exists := seenEdges[key]; exists {
			issues = append(
				issues,
				Issue{
					Path:    manifestPath,
					Field:   fieldPrefix,
					Message: fmt.Sprintf("duplicate edge %q -> %q", from, to),
				},
			)
		} else {
			seenEdges[key] = struct{}{}
		}
	}
	return issues
}

func validateTaskGraphAcyclic(manifestPath string, nodes []TaskGraphNode, edges []TaskGraphEdge) []Issue {
	ids, predecessors, successors := buildTaskGraphAdjacency(nodes, edges)
	if taskGraphIsAcyclic(ids, predecessors, successors) {
		return nil
	}
	return []Issue{taskGraphCycleIssue(manifestPath, ids, predecessors)}
}

func buildTaskGraphAdjacency(
	nodes []TaskGraphNode,
	edges []TaskGraphEdge,
) ([]string, map[string]int, map[string][]string) {
	ids := make([]string, 0, len(nodes))
	predecessors := make(map[string]int, len(nodes))
	successors := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		id := strings.TrimSpace(node.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
		predecessors[id] = 0
	}
	slices.Sort(ids)
	for _, edge := range edges {
		from := strings.TrimSpace(edge.From)
		to := strings.TrimSpace(edge.To)
		if from == "" || to == "" || from == to {
			continue
		}
		if _, ok := predecessors[from]; !ok {
			continue
		}
		if _, ok := predecessors[to]; !ok {
			continue
		}
		successors[from] = append(successors[from], to)
		predecessors[to]++
	}
	return ids, predecessors, successors
}

func taskGraphIsAcyclic(ids []string, predecessors map[string]int, successors map[string][]string) bool {
	ready := make([]string, 0, len(ids))
	for _, id := range ids {
		if predecessors[id] == 0 {
			ready = append(ready, id)
		}
	}
	seen := 0
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		seen++
		slices.Sort(successors[id])
		for _, successor := range successors[id] {
			predecessors[successor]--
			if predecessors[successor] == 0 {
				ready = append(ready, successor)
				slices.Sort(ready)
			}
		}
	}
	return seen == len(ids)
}

func taskGraphCycleIssue(manifestPath string, ids []string, predecessors map[string]int) Issue {
	remaining := make([]string, 0)
	for _, id := range ids {
		if predecessors[id] > 0 {
			remaining = append(remaining, id)
		}
	}
	return Issue{
		Path:    manifestPath,
		Field:   "graph.edges",
		Message: fmt.Sprintf("graph contains a cycle involving: %s", strings.Join(remaining, ", ")),
	}
}

func taskFrontMatterHasKey(content string, key string) bool {
	var node yaml.Node
	if _, err := frontmatter.Parse(content, &node); err != nil {
		return false
	}
	mapping := &node
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return false
		}
		mapping = node.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return false
	}
	for idx := 0; idx+1 < len(mapping.Content); idx += 2 {
		keyNode := mapping.Content[idx]
		if keyNode.Kind == yaml.ScalarNode && strings.EqualFold(strings.TrimSpace(keyNode.Value), key) {
			return true
		}
	}
	return false
}

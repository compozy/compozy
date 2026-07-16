package tasks

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
)

// taskGraphFileMeta is the canonical front matter for a compozy.tasks/v2 task
// file. It intentionally omits dependencies: for schema compozy.tasks/v2 the
// dependency relationships live only in the `_tasks.md` graph edges, never in
// individual task front matter (see loadTaskGraphNodeFile).
type taskGraphFileMeta struct {
	Status     string `yaml:"status"`
	Title      string `yaml:"title"`
	TaskType   string `yaml:"type"`
	Complexity string `yaml:"complexity,omitempty"`
}

// IsTaskGraphManifestContent reports whether content is a compozy.tasks/v2 task
// graph manifest (YAML front matter graph) rather than a legacy Markdown-table
// task list. It keys on the schema version or the presence of graph nodes so a
// plain task file (status/title/type/complexity front matter) is not mistaken
// for a manifest.
func IsTaskGraphManifestContent(content string) bool {
	var manifest TaskGraphManifest
	if _, err := frontmatter.Parse(content, &manifest); err != nil {
		return false
	}
	return strings.TrimSpace(manifest.SchemaVersion) == TaskGraphManifestVersion ||
		len(manifest.Graph.Nodes) > 0
}

// RenderTaskGraphTaskFile formats a compozy.tasks/v2 task file: front matter
// with only task-owned metadata (status, title, type, complexity) followed by
// body. It never emits a dependencies key, which v2 validation rejects.
func RenderTaskGraphTaskFile(status, title, taskType, complexity, body string) (string, error) {
	return frontmatter.Format(taskGraphFileMeta{
		Status:     strings.TrimSpace(status),
		Title:      strings.TrimSpace(title),
		TaskType:   strings.TrimSpace(taskType),
		Complexity: strings.TrimSpace(complexity),
	}, body)
}

// AppendTaskGraphNode adds a node (and its dependency edges) to a
// compozy.tasks/v2 manifest and returns the rewritten file content. deps are
// prerequisite task identities (task_NN) that must finish before id can start;
// each becomes an edge dep -> id. Existing nodes/edges are preserved and
// deduplicated. It returns an error if a dependency does not resolve to an
// existing graph node, so an invalid manifest is never written.
func AppendTaskGraphNode(content, id, file string, deps []string) (string, error) {
	var manifest TaskGraphManifest
	body, err := frontmatter.Parse(content, &manifest)
	if err != nil {
		return "", fmt.Errorf("parse task graph manifest: %w", err)
	}

	id = strings.TrimSpace(id)
	file = filepath.ToSlash(strings.TrimSpace(file))
	if id == "" || file == "" {
		return "", errors.New("task graph node requires id and file")
	}

	nodeIDs := make(map[string]struct{}, len(manifest.Graph.Nodes)+1)
	for _, node := range manifest.Graph.Nodes {
		nodeIDs[strings.TrimSpace(node.ID)] = struct{}{}
	}
	if _, exists := nodeIDs[id]; !exists {
		manifest.Graph.Nodes = append(manifest.Graph.Nodes, TaskGraphNode{ID: id, File: file})
		nodeIDs[id] = struct{}{}
	}

	edgeKey := func(from, to string) string { return from + "\x00" + to }
	seenEdges := make(map[string]struct{}, len(manifest.Graph.Edges)+len(deps))
	for _, edge := range manifest.Graph.Edges {
		seenEdges[edgeKey(strings.TrimSpace(edge.From), strings.TrimSpace(edge.To))] = struct{}{}
	}
	for _, dep := range deps {
		from := TaskIdentityFromName(strings.TrimSpace(dep))
		if from == "" || from == id {
			continue
		}
		if _, ok := nodeIDs[from]; !ok {
			return "", fmt.Errorf("task graph dependency %q is not a graph node", dep)
		}
		key := edgeKey(from, id)
		if _, ok := seenEdges[key]; ok {
			continue
		}
		manifest.Graph.Edges = append(manifest.Graph.Edges, TaskGraphEdge{From: from, To: id})
		seenEdges[key] = struct{}{}
	}

	return frontmatter.Format(manifest, body)
}

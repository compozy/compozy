package taskgroups

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"gopkg.in/yaml.v3"
)

var (
	taskGroupIDPattern                 = regexp.MustCompile(`^TG-[0-9]{3}$`)
	taskGroupDirectoryBriefPattern     = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	taskGroupDirectorySeparatorPattern = regexp.MustCompile(`[^a-z0-9]+`)
	taskGroupHeadingPattern            = regexp.MustCompile(`^## \[([ x])\] ([^ ]+) —[ \t]*(.*?)[\r\n]*$`)
	markdownDependencyLine             = regexp.MustCompile("^  - `?(TG-[0-9]{3})`? —[ \\t]*(.*?)[\\r\\n]*$")
)

type manifest struct {
	SchemaVersion string        `yaml:"schema_version"`
	Initiative    string        `yaml:"initiative"`
	Graph         manifestGraph `yaml:"graph"`
}

type manifestGraph struct {
	Nodes []manifestNode `yaml:"nodes"`
	Edges []manifestEdge `yaml:"edges"`
}

type manifestNode struct {
	ID        string `yaml:"id"`
	Directory string `yaml:"directory"`
}

type manifestEdge struct {
	From      string `yaml:"from"`
	To        string `yaml:"to"`
	Rationale string `yaml:"rationale"`
}

type markdownTaskGroup struct {
	ID           string
	Title        string
	Completed    bool
	Reference    string
	Outcome      string
	OwnedScope   []string
	Dependencies []Dependency
}

// ParsePlan parses and validates a canonical Task Group manifest.
func ParsePlan(content string) (Plan, error) {
	return parsePlan(content, "", "")
}

// ValidatePlan parses and validates a canonical Task Group manifest.
func ValidatePlan(content string) (Plan, error) {
	return ParsePlan(content)
}

// ParsePlanForInitiative parses a plan and requires its initiative field to match expectedInitiative.
func ParsePlanForInitiative(content, expectedInitiative string) (Plan, error) {
	return parsePlan(content, expectedInitiative, "")
}

func parsePlan(content, expectedInitiative, path string) (Plan, error) {
	var raw manifest
	body, err := frontmatter.Parse(content, &raw)
	if err != nil {
		return Plan{}, newError(ErrInvalidPlan, expectedInitiative, "", path, []Issue{{
			Path: path, Field: "frontmatter", Message: err.Error(),
		}})
	}

	normalizeManifest(&raw)
	issues := validateManifest(raw, expectedInitiative, path)
	bodyTaskGroups, bodyIssues := parseMarkdownTaskGroups(body, path)
	issues = append(issues, bodyIssues...)
	issues = append(issues, validatePlanSurfaces(raw, bodyTaskGroups, path)...)
	if len(issues) > 0 {
		return Plan{}, newError(ErrInvalidPlan, raw.Initiative, "", path, issues)
	}

	taskGroups := make([]TaskGroup, 0, len(raw.Graph.Nodes))
	incoming := incomingDependencies(raw.Graph.Edges)
	for _, node := range raw.Graph.Nodes {
		bodyTaskGroup := bodyTaskGroups[node.ID]
		taskGroups = append(taskGroups, TaskGroup{
			ID:           node.ID,
			Title:        bodyTaskGroup.Title,
			Outcome:      bodyTaskGroup.Outcome,
			Reference:    bodyTaskGroup.Reference,
			Directory:    node.Directory,
			Completed:    bodyTaskGroup.Completed,
			Dependencies: slices.Clone(incoming[node.ID]),
			OwnedScope:   slices.Clone(bodyTaskGroup.OwnedScope),
		})
	}
	slices.SortFunc(taskGroups, func(left, right TaskGroup) int { return strings.Compare(left.ID, right.ID) })
	edges := allDependencies(raw.Graph.Edges)
	checksum := sha256.Sum256([]byte(content))
	return Plan{
		SchemaVersion: raw.SchemaVersion,
		Initiative:    raw.Initiative,
		TaskGroups:    taskGroups,
		Edges:         edges,
		Path:          path,
		Checksum:      hex.EncodeToString(checksum[:]),
		raw:           []byte(content),
	}, nil
}

func normalizeManifest(raw *manifest) {
	if raw == nil {
		return
	}
	raw.SchemaVersion = strings.TrimSpace(raw.SchemaVersion)
	raw.Initiative = strings.TrimSpace(raw.Initiative)
	for index := range raw.Graph.Nodes {
		raw.Graph.Nodes[index].ID = strings.TrimSpace(raw.Graph.Nodes[index].ID)
		raw.Graph.Nodes[index].Directory = strings.TrimSpace(
			strings.ReplaceAll(raw.Graph.Nodes[index].Directory, "\\", "/"),
		)
	}
	for index := range raw.Graph.Edges {
		raw.Graph.Edges[index].From = strings.TrimSpace(raw.Graph.Edges[index].From)
		raw.Graph.Edges[index].To = strings.TrimSpace(raw.Graph.Edges[index].To)
		raw.Graph.Edges[index].Rationale = strings.TrimSpace(raw.Graph.Edges[index].Rationale)
	}
}

func validateManifest(raw manifest, expectedInitiative, path string) []Issue {
	issues := validateManifestHeader(raw, expectedInitiative, path)
	nodes, nodeIssues := validateManifestNodes(raw.Graph.Nodes, path)
	issues = append(issues, nodeIssues...)
	edgeIssues := validateManifestEdges(raw.Graph.Edges, nodes, path)
	issues = append(issues, edgeIssues...)
	if len(nodes) == len(raw.Graph.Nodes) {
		issues = append(issues, validateManifestCycle(raw, path)...)
	}
	return issues
}

func validateManifestHeader(raw manifest, expectedInitiative, path string) []Issue {
	issues := make([]Issue, 0, 3)
	if raw.SchemaVersion != SchemaVersion {
		issues = append(
			issues,
			Issue{Path: path, Field: "schema_version", Message: fmt.Sprintf("must be %q", SchemaVersion)},
		)
	}
	if raw.Initiative == "" {
		issues = append(issues, Issue{Path: path, Field: "initiative", Message: "is required"})
	} else if expectedInitiative != "" && raw.Initiative != expectedInitiative {
		issues = append(
			issues,
			Issue{
				Path:    path,
				Field:   "initiative",
				Message: fmt.Sprintf("must match containing initiative %q", expectedInitiative),
			},
		)
	}
	if len(raw.Graph.Nodes) == 0 {
		issues = append(
			issues,
			Issue{Path: path, Field: "graph.nodes", Message: "must contain at least one task group"},
		)
	}
	return issues
}

func validateManifestNodes(nodesInput []manifestNode, path string) (map[string]struct{}, []Issue) {
	nodes := make(map[string]struct{}, len(nodesInput))
	issues := make([]Issue, 0)
	for index, node := range nodesInput {
		prefix := fmt.Sprintf("graph.nodes[%d]", index)
		if !taskGroupIDPattern.MatchString(node.ID) {
			issues = append(issues, Issue{Path: path, Field: prefix + ".id", Message: "must match TG-NNN"})
		} else if _, exists := nodes[node.ID]; exists {
			issues = append(
				issues,
				Issue{Path: path, Field: prefix + ".id", Message: fmt.Sprintf("duplicate task group ID %q", node.ID)},
			)
		} else {
			nodes[node.ID] = struct{}{}
		}
		if !validTaskGroupDirectory(node.ID, node.Directory) {
			issues = append(
				issues,
				Issue{
					Path:  path,
					Field: prefix + ".directory",
					Message: fmt.Sprintf(
						"must be _task_groups/%s or _task_groups/%s-<brief>",
						node.ID,
						taskGroupOrdinal(node.ID),
					),
				},
			)
		}
	}
	return nodes, issues
}

func validTaskGroupDirectory(taskGroupID, directory string) bool {
	if !taskGroupIDPattern.MatchString(taskGroupID) {
		return false
	}
	if directory == "_task_groups/"+taskGroupID {
		return true
	}
	prefix := "_task_groups/" + taskGroupOrdinal(taskGroupID) + "-"
	if !strings.HasPrefix(directory, prefix) {
		return false
	}
	return taskGroupDirectoryBriefPattern.MatchString(strings.TrimPrefix(directory, prefix))
}

func taskGroupOrdinal(taskGroupID string) string {
	return strings.TrimPrefix(taskGroupID, "TG-")
}

func readableTaskGroupDirectory(taskGroupID, title string) string {
	brief := strings.ToLower(strings.TrimSpace(title))
	brief = taskGroupDirectorySeparatorPattern.ReplaceAllString(brief, "-")
	brief = strings.Trim(brief, "-")
	if brief == "" {
		brief = "task-group"
	}
	return "_task_groups/" + taskGroupOrdinal(taskGroupID) + "-" + brief
}

func validateManifestEdges(edges []manifestEdge, nodes map[string]struct{}, path string) []Issue {
	issues := make([]Issue, 0)
	seenEdges := make(map[string]struct{}, len(edges))
	for index, edge := range edges {
		prefix := fmt.Sprintf("graph.edges[%d]", index)
		if edge.From == "" || edge.To == "" {
			issues = append(issues, Issue{Path: path, Field: prefix, Message: "must name both endpoints"})
			continue
		}
		if edge.From == edge.To {
			issues = append(
				issues,
				Issue{Path: path, Field: prefix, Message: fmt.Sprintf("self-dependency %q is not allowed", edge.From)},
			)
		}
		if _, exists := nodes[edge.From]; !exists {
			issues = append(
				issues,
				Issue{Path: path, Field: prefix + ".from", Message: fmt.Sprintf("unknown task group %q", edge.From)},
			)
		}
		if _, exists := nodes[edge.To]; !exists {
			issues = append(
				issues,
				Issue{Path: path, Field: prefix + ".to", Message: fmt.Sprintf("unknown task group %q", edge.To)},
			)
		}
		if edge.Rationale == "" {
			issues = append(issues, Issue{Path: path, Field: prefix + ".rationale", Message: "is required"})
		}
		key := edge.From + "\x00" + edge.To
		if _, exists := seenEdges[key]; exists {
			issues = append(
				issues,
				Issue{
					Path:    path,
					Field:   prefix,
					Message: fmt.Sprintf("duplicate dependency %s -> %s", edge.From, edge.To),
				},
			)
		} else {
			seenEdges[key] = struct{}{}
		}
	}
	return issues
}

func validateManifestCycle(raw manifest, path string) []Issue {
	cycle := graphCycle(raw.Graph.Nodes, raw.Graph.Edges)
	if len(cycle) == 0 {
		return nil
	}
	return []Issue{{Path: path, Field: "graph.edges", Message: "graph contains cycle: " + strings.Join(cycle, " -> ")}}
}

func parseMarkdownTaskGroups(body, path string) (map[string]markdownTaskGroup, []Issue) {
	lines := strings.SplitAfter(body, "\n")
	taskGroups := make(map[string]markdownTaskGroup)
	issues := make([]Issue, 0)
	for index := 0; index < len(lines); {
		match := taskGroupHeadingPattern.FindStringSubmatch(lines[index])
		if match == nil {
			index++
			continue
		}
		taskGroup, next, taskGroupIssues := parseMarkdownTaskGroup(lines, index, match, path)
		issues = append(issues, taskGroupIssues...)
		if _, exists := taskGroups[taskGroup.ID]; exists {
			issues = append(
				issues,
				Issue{Path: path, Field: "body." + taskGroup.ID, Message: "duplicate Markdown task group heading"},
			)
		} else {
			taskGroups[taskGroup.ID] = taskGroup
		}
		index = next
	}
	return taskGroups, issues
}

func parseMarkdownTaskGroup(lines []string, start int, match []string, path string) (markdownTaskGroup, int, []Issue) {
	taskGroup := markdownTaskGroup{
		ID:        strings.TrimSpace(match[2]),
		Title:     strings.TrimSpace(match[3]),
		Completed: match[1] == "x",
	}
	next := nextMarkdownHeading(lines, start+1)
	parseMarkdownTaskGroupFields(&taskGroup, lines[start+1:next])
	return taskGroup, next, validateMarkdownTaskGroup(taskGroup, path)
}

func nextMarkdownHeading(lines []string, start int) int {
	for index := start; index < len(lines); index++ {
		if strings.HasPrefix(strings.TrimRight(lines[index], "\r\n"), "## ") {
			return index
		}
	}
	return len(lines)
}

func parseMarkdownTaskGroupFields(taskGroup *markdownTaskGroup, lines []string) {
	for index, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r\n")
		switch {
		case strings.HasPrefix(line, "- Reference:"):
			taskGroup.Reference = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "- Reference:")), "`")
		case strings.HasPrefix(line, "- Outcome:"):
			taskGroup.Outcome = strings.TrimSpace(strings.TrimPrefix(line, "- Outcome:"))
		case strings.HasPrefix(line, "- Owns:"):
			taskGroup.OwnedScope = markdownListItems(lines[index+1:])
		case strings.HasPrefix(line, "- Dependencies:"):
			taskGroup.Dependencies = markdownDependencies(taskGroup.ID, line, lines[index+1:])
		}
	}
}

func markdownListItems(lines []string) []string {
	items := make([]string, 0)
	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r\n")
		if isMarkdownSectionBoundary(line) {
			break
		}
		if strings.HasPrefix(line, "  - ") {
			if value := strings.TrimSpace(strings.TrimPrefix(line, "  - ")); value != "" {
				items = append(items, value)
			}
		}
	}
	return items
}

func markdownDependencies(taskGroupID, heading string, lines []string) []Dependency {
	if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(heading, "- Dependencies:")), "none") {
		return nil
	}
	dependencies := make([]Dependency, 0)
	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r\n")
		if isMarkdownSectionBoundary(line) {
			break
		}
		match := markdownDependencyLine.FindStringSubmatch(line)
		if match != nil {
			dependencies = append(
				dependencies,
				Dependency{
					From:      match[1],
					To:        taskGroupID,
					Rationale: strings.TrimSpace(match[2]),
				},
			)
		}
	}
	return dependencies
}

func isMarkdownSectionBoundary(line string) bool {
	return strings.HasPrefix(line, "## ") || (strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "  - "))
}

func validateMarkdownTaskGroup(taskGroup markdownTaskGroup, path string) []Issue {
	issues := make([]Issue, 0)
	if !taskGroupIDPattern.MatchString(taskGroup.ID) {
		issues = append(
			issues,
			Issue{Path: path, Field: "body.id", Message: fmt.Sprintf("invalid task group ID %q", taskGroup.ID)},
		)
	}
	if taskGroup.Title == "" {
		issues = append(issues, Issue{Path: path, Field: "body." + taskGroup.ID + ".title", Message: "is required"})
	}
	if taskGroup.Reference == "" {
		issues = append(issues, Issue{Path: path, Field: "body." + taskGroup.ID + ".reference", Message: "is required"})
	}
	if taskGroup.Outcome == "" {
		issues = append(issues, Issue{Path: path, Field: "body." + taskGroup.ID + ".outcome", Message: "is required"})
	}
	if len(taskGroup.OwnedScope) == 0 {
		issues = append(
			issues,
			Issue{Path: path, Field: "body." + taskGroup.ID + ".owns", Message: "must contain owned scope"},
		)
	}
	return issues
}

func validatePlanSurfaces(raw manifest, body map[string]markdownTaskGroup, path string) []Issue {
	issues := make([]Issue, 0)
	nodes := make(map[string]manifestNode, len(raw.Graph.Nodes))
	for _, node := range raw.Graph.Nodes {
		nodes[node.ID] = node
	}
	for id, bodyTaskGroup := range body {
		if _, exists := nodes[id]; !exists {
			issues = append(
				issues,
				Issue{Path: path, Field: "body." + id, Message: "Markdown task group has no YAML graph node"},
			)
			continue
		}
		if bodyTaskGroup.Reference != raw.Initiative+"/"+id {
			issues = append(
				issues,
				Issue{
					Path:    path,
					Field:   "body." + id + ".reference",
					Message: fmt.Sprintf("must be %q", raw.Initiative+"/"+id),
				},
			)
		}
	}
	for id := range nodes {
		if _, exists := body[id]; !exists {
			issues = append(
				issues,
				Issue{Path: path, Field: "graph.nodes." + id, Message: "YAML task group has no Markdown heading"},
			)
		}
	}

	graphIncoming := incomingDependencies(raw.Graph.Edges)
	for id, bodyTaskGroup := range body {
		if !sameDependencies(graphIncoming[id], bodyTaskGroup.Dependencies) {
			issues = append(
				issues,
				Issue{
					Path:    path,
					Field:   "body." + id + ".dependencies",
					Message: "must mirror YAML graph dependencies and rationales",
				},
			)
		}
	}
	for _, edge := range raw.Graph.Edges {
		producer, exists := body[edge.From]
		if exists && producer.Outcome == "" {
			issues = append(
				issues,
				Issue{
					Path:    path,
					Field:   "graph.edges",
					Message: fmt.Sprintf("dependency %s -> %s consumes empty producer outcome", edge.From, edge.To),
				},
			)
		}
	}
	return issues
}

func sameDependencies(left, right []Dependency) bool {
	if len(left) != len(right) {
		return false
	}
	left = slices.Clone(left)
	right = slices.Clone(right)
	slices.SortFunc(left, compareDependency)
	slices.SortFunc(right, compareDependency)
	return slices.EqualFunc(left, right, func(a, b Dependency) bool {
		return a.From == b.From && a.To == b.To && a.Rationale == b.Rationale
	})
}

func incomingDependencies(edges []manifestEdge) map[string][]Dependency {
	incoming := make(map[string][]Dependency)
	for _, edge := range edges {
		incoming[edge.To] = append(
			incoming[edge.To],
			Dependency(edge),
		)
	}
	for id := range incoming {
		slices.SortFunc(incoming[id], compareDependency)
	}
	return incoming
}

func allDependencies(edges []manifestEdge) []Dependency {
	result := make([]Dependency, 0, len(edges))
	for _, edge := range edges {
		result = append(result, Dependency(edge))
	}
	slices.SortFunc(result, compareDependency)
	return result
}

func compareDependency(left, right Dependency) int {
	if result := strings.Compare(left.From, right.From); result != 0 {
		return result
	}
	if result := strings.Compare(left.To, right.To); result != 0 {
		return result
	}
	return strings.Compare(left.Rationale, right.Rationale)
}

func graphCycle(nodes []manifestNode, edges []manifestEdge) []string {
	adjacent := make(map[string][]string, len(nodes))
	for _, edge := range edges {
		adjacent[edge.From] = append(adjacent[edge.From], edge.To)
	}
	for id := range adjacent {
		slices.Sort(adjacent[id])
	}
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		ids = append(ids, node.ID)
	}
	slices.Sort(ids)
	state := make(map[string]uint8, len(ids))
	stack := make([]string, 0, len(ids))
	var visit func(string) []string
	visit = func(id string) []string {
		state[id] = 1
		stack = append(stack, id)
		for _, next := range adjacent[id] {
			switch state[next] {
			case 0:
				if cycle := visit(next); len(cycle) > 0 {
					return cycle
				}
			case 1:
				start := slices.Index(stack, next)
				cycle := append([]string(nil), stack[start:]...)
				return append(cycle, next)
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
		return nil
	}
	for _, id := range ids {
		if state[id] == 0 {
			if cycle := visit(id); len(cycle) > 0 {
				return cycle
			}
		}
	}
	return nil
}

// RenderPlan renders a normalized plan into the canonical hybrid Markdown form.
func RenderPlan(plan Plan) ([]byte, error) {
	taskGroups, err := renderableTaskGroups(plan)
	if err != nil {
		return nil, err
	}
	raw := renderManifest(plan, taskGroups)
	header, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal task group plan: %w", err)
	}
	return []byte("---\n" + string(header) + "---\n\n" + renderPlanBody(plan, taskGroups)), nil
}

// RenderTaskGroupExcerpt renders one validated task group as a standalone Markdown excerpt.
func RenderTaskGroupExcerpt(plan Plan, taskGroupID string) ([]byte, error) {
	taskGroup, found := plan.TaskGroup(strings.TrimSpace(taskGroupID))
	if !found {
		return nil, taskGroupNotFound(Ref{Initiative: plan.Initiative, TaskGroupID: taskGroupID}, plan)
	}
	var body strings.Builder
	renderTaskGroupBody(&body, plan, taskGroup)
	return []byte(body.String()), nil
}

func renderableTaskGroups(plan Plan) ([]TaskGroup, error) {
	if plan.SchemaVersion == "" {
		plan.SchemaVersion = SchemaVersion
	}
	if plan.SchemaVersion != SchemaVersion {
		return nil, newError(
			ErrInvalidPlan,
			plan.Initiative,
			"",
			plan.Path,
			[]Issue{{
				Field: "schema_version", Message: fmt.Sprintf("must be %q", SchemaVersion),
			}},
		)
	}
	if !safeInitiative(plan.Initiative) {
		return nil, newError(
			ErrInvalidPlan,
			plan.Initiative,
			"",
			plan.Path,
			[]Issue{{
				Field: "initiative", Message: "must be one safe task-root component",
			}},
		)
	}
	taskGroups := slices.Clone(plan.TaskGroups)
	slices.SortFunc(taskGroups, func(left, right TaskGroup) int { return strings.Compare(left.ID, right.ID) })
	for index := range taskGroups {
		taskGroup := &taskGroups[index]
		if !renderableTaskGroup(*taskGroup) {
			return nil, newError(
				ErrInvalidPlan,
				plan.Initiative,
				taskGroup.ID,
				plan.Path,
				[]Issue{{
					Field: "task group", Message: "cannot render incomplete task group",
				}},
			)
		}
	}
	return taskGroups, nil
}

func renderableTaskGroup(taskGroup TaskGroup) bool {
	return taskGroupIDPattern.MatchString(taskGroup.ID) &&
		(taskGroup.Directory == "" || validTaskGroupDirectory(taskGroup.ID, taskGroup.Directory)) &&
		strings.TrimSpace(taskGroup.Title) != "" &&
		strings.TrimSpace(taskGroup.Outcome) != "" &&
		len(taskGroup.OwnedScope) > 0
}

func renderManifest(plan Plan, taskGroups []TaskGroup) manifest {
	raw := manifest{SchemaVersion: SchemaVersion, Initiative: plan.Initiative}
	for index := range taskGroups {
		taskGroup := &taskGroups[index]
		directory := taskGroup.Directory
		if directory == "" {
			directory = readableTaskGroupDirectory(taskGroup.ID, taskGroup.Title)
		}
		raw.Graph.Nodes = append(raw.Graph.Nodes, manifestNode{ID: taskGroup.ID, Directory: directory})
	}
	for _, edge := range plan.Edges {
		raw.Graph.Edges = append(raw.Graph.Edges, manifestEdge(edge))
	}
	slices.SortFunc(raw.Graph.Edges, func(left, right manifestEdge) int {
		return compareDependency(Dependency(left), Dependency(right))
	})
	return raw
}

func renderPlanBody(plan Plan, taskGroups []TaskGroup) string {
	var body strings.Builder
	body.WriteString("# ")
	body.WriteString(plan.Initiative)
	body.WriteString(" Task Groups\n\n")
	for index := range taskGroups {
		renderTaskGroupBody(&body, plan, taskGroups[index])
	}
	return body.String()
}

func renderTaskGroupBody(body *strings.Builder, plan Plan, taskGroup TaskGroup) {
	checkbox := " "
	if taskGroup.Completed {
		checkbox = "x"
	}
	fmt.Fprintf(body, "## [%s] %s — %s\n\n", checkbox, taskGroup.ID, taskGroup.Title)
	fmt.Fprintf(body, "- Reference: `%s/%s`\n", plan.Initiative, taskGroup.ID)
	fmt.Fprintf(body, "- Outcome: %s\n", taskGroup.Outcome)
	body.WriteString("- Owns:\n")
	for _, scope := range taskGroup.OwnedScope {
		fmt.Fprintf(body, "  - %s\n", scope)
	}
	dependencies := incomingForTaskGroup(plan.Edges, taskGroup.ID)
	if len(dependencies) == 0 {
		body.WriteString("- Dependencies: None\n")
	} else {
		body.WriteString("- Dependencies:\n")
		for _, dependency := range dependencies {
			fmt.Fprintf(body, "  - `%s` — %s\n", dependency.From, dependency.Rationale)
		}
	}
	body.WriteByte('\n')
}

func incomingForTaskGroup(edges []Dependency, id string) []Dependency {
	result := make([]Dependency, 0)
	for _, edge := range edges {
		if edge.To == id {
			result = append(result, edge)
		}
	}
	slices.SortFunc(result, compareDependency)
	return result
}

// ValidateTaskGroupRemoval reports task groups whose dependencies would be broken by removal.
func ValidateTaskGroupRemoval(plan Plan, taskGroupID string) []Issue {
	issues := make([]Issue, 0)
	for _, edge := range plan.Edges {
		if edge.From == taskGroupID || edge.To == taskGroupID {
			issues = append(issues, Issue{
				Path:    plan.Path,
				Field:   "graph.edges",
				Message: fmt.Sprintf("task group %q affects dependency %s -> %s", taskGroupID, edge.From, edge.To),
			})
		}
	}
	return sortedIssues(issues)
}

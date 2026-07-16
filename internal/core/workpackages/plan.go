package workpackages

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
	packageIDPattern       = regexp.MustCompile(`^WP-[0-9]{3}$`)
	packageHeadingPattern  = regexp.MustCompile(`^## \[([ x])\] ([^ ]+) —[ \t]*(.*?)[\r\n]*$`)
	markdownDependencyLine = regexp.MustCompile("^  - `?(WP-[0-9]{3})`? —[ \\t]*(.*?)[\\r\\n]*$")
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

type markdownPackage struct {
	ID           string
	Title        string
	Completed    bool
	Reference    string
	Outcome      string
	OwnedScope   []string
	Dependencies []Dependency
}

// ParsePlan parses and validates a canonical Work Package manifest.
func ParsePlan(content string) (Plan, error) {
	return parsePlan(content, "", "")
}

// ValidatePlan parses and validates a canonical Work Package manifest.
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
	bodyPackages, bodyIssues := parseMarkdownPackages(body, path)
	issues = append(issues, bodyIssues...)
	issues = append(issues, validatePlanSurfaces(raw, bodyPackages, path)...)
	if len(issues) > 0 {
		return Plan{}, newError(ErrInvalidPlan, raw.Initiative, "", path, issues)
	}

	packages := make([]Package, 0, len(raw.Graph.Nodes))
	incoming := incomingDependencies(raw.Graph.Edges)
	for _, node := range raw.Graph.Nodes {
		bodyPackage := bodyPackages[node.ID]
		packages = append(packages, Package{
			ID:           node.ID,
			Title:        bodyPackage.Title,
			Outcome:      bodyPackage.Outcome,
			Reference:    bodyPackage.Reference,
			Directory:    node.Directory,
			Completed:    bodyPackage.Completed,
			Dependencies: slices.Clone(incoming[node.ID]),
			OwnedScope:   slices.Clone(bodyPackage.OwnedScope),
		})
	}
	slices.SortFunc(packages, func(left, right Package) int { return strings.Compare(left.ID, right.ID) })
	edges := allDependencies(raw.Graph.Edges)
	checksum := sha256.Sum256([]byte(content))
	return Plan{
		SchemaVersion: raw.SchemaVersion,
		Initiative:    raw.Initiative,
		Packages:      packages,
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
		issues = append(issues, Issue{Path: path, Field: "graph.nodes", Message: "must contain at least one package"})
	}
	return issues
}

func validateManifestNodes(nodesInput []manifestNode, path string) (map[string]struct{}, []Issue) {
	nodes := make(map[string]struct{}, len(nodesInput))
	issues := make([]Issue, 0)
	for index, node := range nodesInput {
		prefix := fmt.Sprintf("graph.nodes[%d]", index)
		if !packageIDPattern.MatchString(node.ID) {
			issues = append(issues, Issue{Path: path, Field: prefix + ".id", Message: "must match WP-NNN"})
		} else if _, exists := nodes[node.ID]; exists {
			issues = append(
				issues,
				Issue{Path: path, Field: prefix + ".id", Message: fmt.Sprintf("duplicate package ID %q", node.ID)},
			)
		} else {
			nodes[node.ID] = struct{}{}
		}
		if node.Directory != "_packages/"+node.ID {
			issues = append(
				issues,
				Issue{Path: path, Field: prefix + ".directory", Message: fmt.Sprintf("must be _packages/%s", node.ID)},
			)
		}
	}
	return nodes, issues
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
				Issue{Path: path, Field: prefix + ".from", Message: fmt.Sprintf("unknown package %q", edge.From)},
			)
		}
		if _, exists := nodes[edge.To]; !exists {
			issues = append(
				issues,
				Issue{Path: path, Field: prefix + ".to", Message: fmt.Sprintf("unknown package %q", edge.To)},
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

func parseMarkdownPackages(body, path string) (map[string]markdownPackage, []Issue) {
	lines := strings.SplitAfter(body, "\n")
	packages := make(map[string]markdownPackage)
	issues := make([]Issue, 0)
	for index := 0; index < len(lines); {
		match := packageHeadingPattern.FindStringSubmatch(lines[index])
		if match == nil {
			index++
			continue
		}
		pkg, next, packageIssues := parseMarkdownPackage(lines, index, match, path)
		issues = append(issues, packageIssues...)
		if _, exists := packages[pkg.ID]; exists {
			issues = append(
				issues,
				Issue{Path: path, Field: "body." + pkg.ID, Message: "duplicate Markdown package heading"},
			)
		} else {
			packages[pkg.ID] = pkg
		}
		index = next
	}
	return packages, issues
}

func parseMarkdownPackage(lines []string, start int, match []string, path string) (markdownPackage, int, []Issue) {
	pkg := markdownPackage{
		ID:        strings.TrimSpace(match[2]),
		Title:     strings.TrimSpace(match[3]),
		Completed: match[1] == "x",
	}
	next := nextMarkdownHeading(lines, start+1)
	parseMarkdownPackageFields(&pkg, lines[start+1:next])
	return pkg, next, validateMarkdownPackage(pkg, path)
}

func nextMarkdownHeading(lines []string, start int) int {
	for index := start; index < len(lines); index++ {
		if strings.HasPrefix(strings.TrimRight(lines[index], "\r\n"), "## ") {
			return index
		}
	}
	return len(lines)
}

func parseMarkdownPackageFields(pkg *markdownPackage, lines []string) {
	for index, rawLine := range lines {
		line := strings.TrimRight(rawLine, "\r\n")
		switch {
		case strings.HasPrefix(line, "- Reference:"):
			pkg.Reference = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "- Reference:")), "`")
		case strings.HasPrefix(line, "- Outcome:"):
			pkg.Outcome = strings.TrimSpace(strings.TrimPrefix(line, "- Outcome:"))
		case strings.HasPrefix(line, "- Owns:"):
			pkg.OwnedScope = markdownListItems(lines[index+1:])
		case strings.HasPrefix(line, "- Dependencies:"):
			pkg.Dependencies = markdownDependencies(pkg.ID, line, lines[index+1:])
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

func markdownDependencies(packageID, heading string, lines []string) []Dependency {
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
					To:        packageID,
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

func validateMarkdownPackage(pkg markdownPackage, path string) []Issue {
	issues := make([]Issue, 0)
	if !packageIDPattern.MatchString(pkg.ID) {
		issues = append(
			issues,
			Issue{Path: path, Field: "body.id", Message: fmt.Sprintf("invalid package ID %q", pkg.ID)},
		)
	}
	if pkg.Title == "" {
		issues = append(issues, Issue{Path: path, Field: "body." + pkg.ID + ".title", Message: "is required"})
	}
	if pkg.Reference == "" {
		issues = append(issues, Issue{Path: path, Field: "body." + pkg.ID + ".reference", Message: "is required"})
	}
	if pkg.Outcome == "" {
		issues = append(issues, Issue{Path: path, Field: "body." + pkg.ID + ".outcome", Message: "is required"})
	}
	if len(pkg.OwnedScope) == 0 {
		issues = append(
			issues,
			Issue{Path: path, Field: "body." + pkg.ID + ".owns", Message: "must contain owned scope"},
		)
	}
	return issues
}

func validatePlanSurfaces(raw manifest, body map[string]markdownPackage, path string) []Issue {
	issues := make([]Issue, 0)
	nodes := make(map[string]manifestNode, len(raw.Graph.Nodes))
	for _, node := range raw.Graph.Nodes {
		nodes[node.ID] = node
	}
	for id, bodyPackage := range body {
		if _, exists := nodes[id]; !exists {
			issues = append(
				issues,
				Issue{Path: path, Field: "body." + id, Message: "Markdown package has no YAML graph node"},
			)
			continue
		}
		if bodyPackage.Reference != raw.Initiative+"/"+id {
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
				Issue{Path: path, Field: "graph.nodes." + id, Message: "YAML package has no Markdown heading"},
			)
		}
	}

	graphIncoming := incomingDependencies(raw.Graph.Edges)
	for id, bodyPackage := range body {
		if !sameDependencies(graphIncoming[id], bodyPackage.Dependencies) {
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
	packages, err := renderablePackages(plan)
	if err != nil {
		return nil, err
	}
	raw := renderManifest(plan, packages)
	header, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("marshal work package plan: %w", err)
	}
	return []byte("---\n" + string(header) + "---\n\n" + renderPlanBody(plan, packages)), nil
}

func renderablePackages(plan Plan) ([]Package, error) {
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
	packages := slices.Clone(plan.Packages)
	slices.SortFunc(packages, func(left, right Package) int { return strings.Compare(left.ID, right.ID) })
	for index := range packages {
		pkg := &packages[index]
		if !renderablePackage(*pkg) {
			return nil, newError(
				ErrInvalidPlan,
				plan.Initiative,
				pkg.ID,
				plan.Path,
				[]Issue{{
					Field: "package", Message: "cannot render incomplete package",
				}},
			)
		}
	}
	return packages, nil
}

func renderablePackage(pkg Package) bool {
	return packageIDPattern.MatchString(pkg.ID) &&
		strings.TrimSpace(pkg.Title) != "" &&
		strings.TrimSpace(pkg.Outcome) != "" &&
		len(pkg.OwnedScope) > 0
}

func renderManifest(plan Plan, packages []Package) manifest {
	raw := manifest{SchemaVersion: SchemaVersion, Initiative: plan.Initiative}
	for index := range packages {
		pkg := &packages[index]
		raw.Graph.Nodes = append(raw.Graph.Nodes, manifestNode{ID: pkg.ID, Directory: "_packages/" + pkg.ID})
	}
	for _, edge := range plan.Edges {
		raw.Graph.Edges = append(raw.Graph.Edges, manifestEdge(edge))
	}
	slices.SortFunc(raw.Graph.Edges, func(left, right manifestEdge) int {
		return compareDependency(Dependency(left), Dependency(right))
	})
	return raw
}

func renderPlanBody(plan Plan, packages []Package) string {
	var body strings.Builder
	body.WriteString("# ")
	body.WriteString(plan.Initiative)
	body.WriteString(" Work Packages\n\n")
	for index := range packages {
		pkg := &packages[index]
		checkbox := " "
		if pkg.Completed {
			checkbox = "x"
		}
		fmt.Fprintf(&body, "## [%s] %s — %s\n\n", checkbox, pkg.ID, pkg.Title)
		fmt.Fprintf(&body, "- Reference: `%s/%s`\n", plan.Initiative, pkg.ID)
		fmt.Fprintf(&body, "- Outcome: %s\n", pkg.Outcome)
		body.WriteString("- Owns:\n")
		for _, scope := range pkg.OwnedScope {
			fmt.Fprintf(&body, "  - %s\n", scope)
		}
		dependencies := incomingForPackage(plan.Edges, pkg.ID)
		if len(dependencies) == 0 {
			body.WriteString("- Dependencies: None\n")
		} else {
			body.WriteString("- Dependencies:\n")
			for _, dependency := range dependencies {
				fmt.Fprintf(&body, "  - `%s` — %s\n", dependency.From, dependency.Rationale)
			}
		}
		body.WriteByte('\n')
	}
	return body.String()
}

func incomingForPackage(edges []Dependency, id string) []Dependency {
	result := make([]Dependency, 0)
	for _, edge := range edges {
		if edge.To == id {
			result = append(result, edge)
		}
	}
	slices.SortFunc(result, compareDependency)
	return result
}

// ValidatePackageRemoval reports packages whose dependencies would be broken by removal.
func ValidatePackageRemoval(plan Plan, packageID string) []Issue {
	issues := make([]Issue, 0)
	for _, edge := range plan.Edges {
		if edge.From == packageID || edge.To == packageID {
			issues = append(issues, Issue{
				Path:    plan.Path,
				Field:   "graph.edges",
				Message: fmt.Sprintf("package %q affects dependency %s -> %s", packageID, edge.From, edge.To),
			})
		}
	}
	return sortedIssues(issues)
}

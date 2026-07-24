package prompt

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

type scopedSpecification struct {
	path    string
	content string
}

func buildExecutionScopeSection(scope *model.ExecutionScope) (string, error) {
	if scope == nil {
		return "", nil
	}
	if strings.TrimSpace(scope.SpecDir) == "" || strings.TrimSpace(scope.OperationalDir) == "" ||
		strings.TrimSpace(scope.WorkflowRef) == "" {
		return "", errors.New("build execution scope prompt: incomplete execution scope")
	}

	specifications, err := loadScopedSpecifications(scope.SpecDir)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString("<execution_scope>\n")
	fmt.Fprintf(&sb, "- Workflow reference: `%s`\n", scope.WorkflowRef)
	fmt.Fprintf(&sb, "- Canonical specification directory: `%s`\n", NormalizeForPrompt(scope.SpecDir))
	fmt.Fprintf(&sb, "- Selected task group operational directory: `%s`\n", NormalizeForPrompt(scope.OperationalDir))
	fmt.Fprintf(&sb, "- Selected task group task directory: `%s`\n", NormalizeForPrompt(scope.TasksDir))
	fmt.Fprintf(&sb, "- Selected task group review directory: `%s`\n", NormalizeForPrompt(scope.ReviewsDir))
	fmt.Fprintf(&sb, "- Selected task group memory directory: `%s`\n", NormalizeForPrompt(scope.MemoryDir))
	sb.WriteString("- Do not read or write sibling task group tasks, reviews, memory, journals, or run artifacts.\n")
	sb.WriteString("</execution_scope>\n\n")
	sb.WriteString("<initiative_specifications>\n")
	for _, specification := range specifications {
		fmt.Fprintf(&sb, "### `%s`\n\n%s\n\n", NormalizeForPrompt(specification.path), specification.content)
	}
	sb.WriteString("</initiative_specifications>")
	return sb.String(), nil
}

func loadScopedSpecifications(specDir string) ([]scopedSpecification, error) {
	required := []string{"_prd.md", "_techspec.md"}
	optional := []string{"_user_stories.md", "_tests.md", "_task_groups.md"}
	documents := make([]scopedSpecification, 0, len(required)+len(optional))
	for _, name := range required {
		document, err := readScopedSpecification(filepath.Join(specDir, name), true)
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	for _, name := range optional {
		document, err := readScopedSpecification(filepath.Join(specDir, name), false)
		if err != nil {
			return nil, err
		}
		if document.path != "" {
			documents = append(documents, document)
		}
	}

	adrDir := filepath.Join(specDir, "adrs")
	entries, err := os.ReadDir(adrDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return documents, nil
		}
		return nil, fmt.Errorf("read canonical ADR directory %s: %w", adrDir, err)
	}
	adrNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		adrNames = append(adrNames, entry.Name())
	}
	sort.Strings(adrNames)
	for _, name := range adrNames {
		document, err := readScopedSpecification(filepath.Join(adrDir, name), true)
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, nil
}

func readScopedSpecification(path string, required bool) (scopedSpecification, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if !required && errors.Is(err, fs.ErrNotExist) {
			return scopedSpecification{}, nil
		}
		return scopedSpecification{}, fmt.Errorf("read canonical specification %s: %w", path, err)
	}
	return scopedSpecification{path: path, content: string(content)}, nil
}

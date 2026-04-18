package tasks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
)

const metaFileName = "_meta.md"

func MetaPath(tasksDir string) string {
	return filepath.Join(tasksDir, metaFileName)
}

func ReadTaskMeta(tasksDir string) (model.TaskMeta, error) {
	body, err := os.ReadFile(MetaPath(tasksDir))
	if err != nil {
		return model.TaskMeta{}, fmt.Errorf("read task meta: %w", err)
	}
	return parseTaskMeta(string(body))
}

func WriteTaskMeta(tasksDir string, meta model.TaskMeta) error {
	content, err := formatTaskMeta(meta)
	if err != nil {
		return fmt.Errorf("format task meta: %w", err)
	}
	if err := os.WriteFile(MetaPath(tasksDir), []byte(content), 0o600); err != nil {
		return fmt.Errorf("write task meta: %w", err)
	}
	return nil
}

func RefreshTaskMeta(tasksDir string) (model.TaskMeta, error) {
	meta, err := SnapshotTaskMeta(tasksDir)
	if err != nil {
		return model.TaskMeta{}, err
	}

	if err := WriteTaskMeta(tasksDir, meta); err != nil {
		return model.TaskMeta{}, err
	}
	return meta, nil
}

func SnapshotTaskMeta(tasksDir string) (model.TaskMeta, error) {
	now := time.Now().UTC()
	meta := model.TaskMeta{
		CreatedAt: now,
		UpdatedAt: now,
	}

	existingMeta, err := ReadTaskMeta(tasksDir)
	switch {
	case err == nil:
		meta.CreatedAt = existingMeta.CreatedAt
	case errors.Is(err, os.ErrNotExist):
		// First refresh creates the metadata file.
	default:
		return model.TaskMeta{}, err
	}

	total, completed, err := countTasks(tasksDir)
	if err != nil {
		return model.TaskMeta{}, err
	}

	meta.Total = total
	meta.Completed = completed
	meta.Pending = total - completed

	return meta, nil
}

func MarkTaskCompleted(tasksDir, taskFileName string) error {
	root, err := os.OpenRoot(strings.TrimSpace(tasksDir))
	if err != nil {
		return fmt.Errorf("open tasks root: %w", err)
	}
	defer root.Close()

	taskName, err := resolveTaskName(taskFileName)
	if err != nil {
		return err
	}

	content, err := root.ReadFile(taskName)
	if err != nil {
		return fmt.Errorf("read task file %s: %w", taskName, err)
	}

	task, err := ParseTaskFile(string(content))
	if err != nil {
		return WrapParseError(filepath.Join(strings.TrimSpace(tasksDir), taskName), err)
	}
	if strings.EqualFold(strings.TrimSpace(task.Status), "completed") {
		return nil
	}

	rewritten, err := frontmatter.RewriteStringField(string(content), "status", "completed")
	if err != nil {
		return fmt.Errorf("rewrite task status %s: %w", taskName, err)
	}
	if err := root.WriteFile(taskName, []byte(rewritten), 0o600); err != nil {
		return fmt.Errorf("write task file %s: %w", taskName, err)
	}
	return nil
}

func resolveTaskName(taskFileName string) (string, error) {
	name := filepath.Base(strings.TrimSpace(taskFileName))
	if ExtractTaskNumber(name) == 0 {
		return "", fmt.Errorf("invalid task file name %q", taskFileName)
	}
	return name, nil
}

func formatTaskMeta(meta model.TaskMeta) (string, error) {
	type taskMetaFrontMatter struct {
		CreatedAt time.Time `yaml:"created_at"`
		UpdatedAt time.Time `yaml:"updated_at"`
	}

	summary := strings.Join([]string{
		"## Summary",
		fmt.Sprintf("- Total: %d", meta.Total),
		fmt.Sprintf("- Completed: %d", meta.Completed),
		fmt.Sprintf("- Pending: %d", meta.Pending),
		"",
	}, "\n")

	return frontmatter.Format(taskMetaFrontMatter{
		CreatedAt: meta.CreatedAt.UTC(),
		UpdatedAt: meta.UpdatedAt.UTC(),
	}, summary)
}

func parseTaskMeta(content string) (model.TaskMeta, error) {
	type taskMetaFrontMatter struct {
		CreatedAt time.Time `yaml:"created_at"`
		UpdatedAt time.Time `yaml:"updated_at"`
	}

	var frontMatter taskMetaFrontMatter
	body, err := frontmatter.Parse(content, &frontMatter)
	if err != nil {
		return model.TaskMeta{}, fmt.Errorf("parse task meta front matter: %w", err)
	}

	meta := model.TaskMeta{
		CreatedAt: frontMatter.CreatedAt,
		UpdatedAt: frontMatter.UpdatedAt,
	}
	if meta.CreatedAt.IsZero() || meta.UpdatedAt.IsZero() {
		return model.TaskMeta{}, errors.New("task meta front matter is incomplete")
	}

	if err := parseTaskMetaSummary(strings.Split(body, "\n"), &meta); err != nil {
		return model.TaskMeta{}, err
	}
	if meta.Total != meta.Completed+meta.Pending {
		return model.TaskMeta{}, errors.New("task meta counts are inconsistent")
	}

	return meta, nil
}

func parseTaskMetaSummary(lines []string, meta *model.TaskMeta) error {
	counts := map[string]*int{
		"Total":     &meta.Total,
		"Completed": &meta.Completed,
		"Pending":   &meta.Pending,
	}
	reCount := regexp.MustCompile(`^- (Total|Completed|Pending): (\d+)$`)
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		matches := reCount.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		value, err := strconv.Atoi(matches[2])
		if err != nil {
			return fmt.Errorf("parse %s count: %w", matches[1], err)
		}
		*counts[matches[1]] = value
	}
	return nil
}

func countTasks(tasksDir string) (int, int, error) {
	total := 0
	completed := 0
	err := walkTaskFiles(tasksDir, func(_ model.IssueEntry, task model.TaskEntry) error {
		total++
		if IsTaskCompleted(task) {
			completed++
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	return total, completed, nil
}

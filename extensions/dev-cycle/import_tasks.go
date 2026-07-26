package devcycle

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type importTasksInput struct {
	Pattern string `json:"pattern"`
}

type importTasksOutput struct {
	Tasks []markdownTaskPayload `json:"tasks"`
	Count int                   `json:"count"`
}

type taskSetNotFoundError struct {
	Pattern string
}

func (e *taskSetNotFoundError) Error() string {
	return "dev-cycle: task set not found"
}

func importTasks(input importTasksInput) (importTasksOutput, error) {
	pattern := strings.TrimSpace(input.Pattern)
	if pattern == "" {
		return importTasksOutput{}, fmt.Errorf("%w: import_tasks pattern is required", looppkg.ErrValidation)
	}
	result, err := importMarkdownTasks(pattern)
	if err != nil {
		return importTasksOutput{}, err
	}
	return importTasksOutput{
		Tasks: result.Tasks,
		Count: len(result.Tasks),
	}, nil
}

func importTasksToolError(id toolspkg.ToolID, input importTasksInput, err error) error {
	pattern := operatorTaskPattern(input.Pattern)
	cause := fmt.Sprintf("The task set for %s could not be imported.", pattern)
	recovery := "Correct the task manifest and matching task files, then retry the run."
	reason := toolspkg.ReasonSchemaInvalid
	if strings.TrimSpace(input.Pattern) == "" {
		cause = "The task import pattern is required."
		recovery = "Provide a task file pattern, then retry the run."
	}
	if notFound, ok := errors.AsType[*taskSetNotFoundError](err); ok {
		pattern = operatorTaskPattern(notFound.Pattern)
		cause = fmt.Sprintf("No task set matched %s.", pattern)
		recovery = "Create the matching task set or correct the Loop input, then retry the run."
		reason = toolspkg.ReasonDependencyMissing
	}
	return toolspkg.NewOperatorToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		cause,
		errors.Join(toolspkg.ErrToolInvalidInput, err),
		cause,
		recovery,
		reason,
	)
}

func operatorTaskPattern(pattern string) string {
	clean := filepath.ToSlash(filepath.Clean(strings.TrimSpace(pattern)))
	if index := strings.LastIndex(clean, "/.compozy/"); index >= 0 {
		return clean[index+1:]
	}
	if !filepath.IsAbs(clean) {
		return clean
	}
	return filepath.Base(clean)
}

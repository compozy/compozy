package workpackages

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

// BuildExecutionScope converts one resolved target into paths safe for an
// existing task or review lifecycle.
func BuildExecutionScope(target Target) (model.ExecutionScope, error) {
	if target.Mode != TargetModePackage {
		return model.ExecutionScope{}, newError(
			ErrSelectionRequired,
			target.Ref.Initiative,
			target.Ref.PackageID,
			target.InitiativeDir,
			[]Issue{{Field: "reference", Message: "a complete workflow target is required"}},
		)
	}

	scope := model.ExecutionScope{
		SpecDir:        strings.TrimSpace(target.SpecDir),
		OperationalDir: strings.TrimSpace(target.PackageDir),
		WorkflowRef:    strings.TrimSpace(target.DisplayRef),
		TasksDir:       strings.TrimSpace(target.TasksDir),
		ReviewsDir:     strings.TrimSpace(target.ReviewsDir),
		MemoryDir:      strings.TrimSpace(target.MemoryDir),
	}
	if scope.WorkflowRef == "" {
		scope.WorkflowRef = target.Ref.String()
	}
	if scope.SpecDir == "" || scope.OperationalDir == "" || scope.WorkflowRef == "" ||
		scope.TasksDir == "" || scope.ReviewsDir == "" || scope.MemoryDir == "" {
		return model.ExecutionScope{}, fmt.Errorf("build execution scope: resolved target is incomplete")
	}
	return scope, nil
}

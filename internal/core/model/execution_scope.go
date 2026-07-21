package model

import (
	"fmt"
	"path/filepath"
)

// ExecutionScope separates immutable initiative specifications from one
// workflow's mutable operational artifacts.
type ExecutionScope struct {
	SpecDir        string
	OperationalDir string
	WorkflowRef    string
	TasksDir       string
	ReviewsDir     string
	MemoryDir      string
}

// ReviewDir returns the task-group-local directory for one review round.
func (s ExecutionScope) ReviewDir(round int) string {
	return filepath.Join(s.ReviewsDir, fmt.Sprintf("reviews-%03d", round))
}

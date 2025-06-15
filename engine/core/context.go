package core

import (
	"context"
	"fmt"
)

// Context key for project name
type ProjectNameKey struct{}

// WithProjectName adds project name to context
func WithProjectName(ctx context.Context, projectName string) context.Context {
	return context.WithValue(ctx, ProjectNameKey{}, projectName)
}

// GetProjectName extracts project name from context
func GetProjectName(ctx context.Context) (string, error) {
	projectName, ok := ctx.Value(ProjectNameKey{}).(string)
	if !ok || projectName == "" {
		return "", fmt.Errorf("project name not found in context")
	}
	return projectName, nil
}

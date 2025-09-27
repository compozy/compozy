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

// Context key for request id correlation across tool invocations
type RequestIDKey struct{}

// WithRequestID adds a request identifier to context for correlation
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey{}, requestID)
}

// GetRequestID extracts the request identifier from context
func GetRequestID(ctx context.Context) (string, error) {
	id, ok := ctx.Value(RequestIDKey{}).(string)
	if !ok || id == "" {
		return "", fmt.Errorf("request id not found in context")
	}
	return id, nil
}

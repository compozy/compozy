package tools

import (
	"context"
	"fmt"
	"strings"
)

type approvalProfileContextKey struct{}

// WithApprovalProfile binds a public approval read or mutation to one profile owner.
func WithApprovalProfile(ctx context.Context, profileID string) context.Context {
	return context.WithValue(ctx, approvalProfileContextKey{}, strings.TrimSpace(profileID))
}

// ApprovalProfile returns the profile owner required by public approval operations.
func ApprovalProfile(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("%w: approval profile context is required", ErrApprovalInvalid)
	}
	profileID, ok := ctx.Value(approvalProfileContextKey{}).(string)
	if !ok {
		return "", fmt.Errorf("%w: approval profile is required", ErrApprovalInvalid)
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return "", fmt.Errorf("%w: approval profile is required", ErrApprovalInvalid)
	}
	return profileID, nil
}

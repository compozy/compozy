// Package userctx provides utilities for storing and retrieving authenticated user
// information from context.Context. It is used by the authentication middleware
// to inject user data into the request context and by handlers to access the
// authenticated user.
package userctx

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/engine/auth/model"
)

// userKey is the context key for user information
type userKey struct{}

// WithUser adds user information to context
func WithUser(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, userKey{}, user)
}

// UserFromContext extracts user information from context
func UserFromContext(ctx context.Context) (*model.User, bool) {
	user, ok := ctx.Value(userKey{}).(*model.User)
	return user, ok
}

// MustUserFromContext extracts user information from context, panics if not found.
// WARNING: This function will panic if no user is present in the context.
// Only use this in handlers that are protected by authentication middleware.
// For safer access, use UserFromContext which returns a boolean indicating presence.
func MustUserFromContext(ctx context.Context) (*model.User, error) {
	user, ok := UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("user not found in context")
	}
	return user, nil
}

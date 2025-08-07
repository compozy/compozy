package service

import "context"

// NpmService defines the interface for interacting with npm.

type NpmService interface {
	Publish(ctx context.Context, path string) error
}

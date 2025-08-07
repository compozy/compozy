package service

import "context"

// GoReleaserService defines the interface for interacting with goreleaser.

type GoReleaserService interface {
	Run(ctx context.Context, args ...string) error
}

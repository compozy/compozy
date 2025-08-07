package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// goReleaserService implements the GoReleaserService interface
type goReleaserService struct{}

// NewGoReleaserService creates a new GoReleaserService
func NewGoReleaserService() GoReleaserService {
	return &goReleaserService{}
}

// Run executes goreleaser with the provided arguments
func (s *goReleaserService) Run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "goreleaser", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("goreleaser failed: %w", err)
	}
	return nil
}

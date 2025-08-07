package service

import (
	"context"
	"fmt"
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("goreleaser failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

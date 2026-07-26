package devcycle

import (
	"context"
	"fmt"
	"strings"
)

const gitHeadRef = "HEAD"

type gitProvider struct {
	runner commandRunner
}

type gitPushInput struct {
	Remote              string `json:"remote,omitempty"`
	Branch              string `json:"branch,omitempty"`
	RequireHeadAdvanced bool   `json:"require_head_advanced,omitempty"`
	ExpectedHead        string `json:"expected_head,omitempty"`
}

type gitPushOutput struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	Head   string `json:"head"`
	Pushed bool   `json:"pushed"`
}

func newGitProvider(runner commandRunner) *gitProvider {
	return &gitProvider{runner: runner}
}

func (p *gitProvider) Push(ctx context.Context, input gitPushInput) (gitPushOutput, error) {
	if _, err := p.runner.LookPath("git"); err != nil {
		return gitPushOutput{}, fmt.Errorf("dev-cycle: git executable is required for git_push: %w", err)
	}
	remote := strings.TrimSpace(input.Remote)
	if remote == "" {
		remote = defaultGitRemote
	}
	branch := strings.TrimSpace(input.Branch)
	if branch == "" {
		output, err := p.runner.Run(ctx, "git", []string{gitRevParseArg, "--abbrev-ref", gitHeadRef}, "")
		if err != nil {
			return gitPushOutput{}, err
		}
		branch = strings.TrimSpace(string(output))
	}
	if branch == "" || branch == gitHeadRef {
		return gitPushOutput{}, fmt.Errorf("dev-cycle: branch is required for git_push")
	}
	headOutput, err := p.runner.Run(ctx, "git", []string{gitRevParseArg, gitHeadRef}, "")
	if err != nil {
		return gitPushOutput{}, err
	}
	head := strings.TrimSpace(string(headOutput))
	expected := strings.TrimSpace(input.ExpectedHead)
	if input.RequireHeadAdvanced && expected != "" && head == expected {
		return gitPushOutput{}, fmt.Errorf(
			"dev-cycle: refusing git push because HEAD did not advance from %s",
			expected,
		)
	}
	if input.RequireHeadAdvanced && expected == "" {
		return gitPushOutput{}, fmt.Errorf("dev-cycle: expected_head is required when require_head_advanced is true")
	}
	if _, err := p.runner.Run(ctx, "git", []string{"push", "--", remote, branch}, ""); err != nil {
		return gitPushOutput{}, err
	}
	return gitPushOutput{
		Remote: remote,
		Branch: branch,
		Head:   head,
		Pushed: true,
	}, nil
}

package devcycle

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

const (
	defaultGitRemote      = "origin"
	codeRabbitFixed       = "fixed"
	codeRabbitDocumented  = "documented"
	codeRabbitTriageValid = "valid"
)

type commandRunner interface {
	Run(ctx context.Context, name string, args []string, dir string) ([]byte, error)
	LookPath(file string) (string, error)
}

type defaultCommandRunner struct{}

func (defaultCommandRunner) Run(ctx context.Context, name string, args []string, dir string) ([]byte, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("dev-cycle: command name is required")
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if strings.TrimSpace(dir) != "" {
		cmd.Dir = dir
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(output.String()))
	}
	return output.Bytes(), nil
}

func (defaultCommandRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

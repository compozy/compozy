//go:build mage

package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func gitShowFile(ctx context.Context, repoDir string, ref string, path string) (string, bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "show", ref+":"+path)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return string(output), true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 128 {
		return "", false, nil
	}
	return "", false, fmt.Errorf("read %s from %s: %w\n%s", path, ref, err, output)
}

func gitHasDiff(ctx context.Context, repoDir string, paths ...string) (bool, error) {
	args := []string{"-C", repoDir, "diff", "--quiet", "--"}
	args = append(args, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("check web assets diff: %w", err)
}

func gitCommandOutput(ctx context.Context, dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output)), nil
}

func gitTags(dir string) ([]string, error) {
	cmd := exec.Command("git", "-C", dir, "tag", "--list", "v*")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list assets tags in %q: %w", dir, err)
	}
	lines := strings.Split(string(output), "\n")
	tags := make([]string, 0, len(lines))
	for _, line := range lines {
		tag := strings.TrimSpace(line)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

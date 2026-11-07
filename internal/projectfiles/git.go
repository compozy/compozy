package projectfiles

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"strings"

	"github.com/compozy/compozy/internal/worktree"
)

// GitReadArgs applies the shared read-only policy, discovering filter drivers afresh on each call.
func GitReadArgs(
	ctx context.Context,
	runner worktree.StreamingGitRunner,
	root, command string,
	args ...string,
) ([]string, error) {
	prefix := []string{
		"--no-pager",
		"--no-optional-locks",
		"-c",
		"core.fsmonitor=false",
		"-c",
		"core.hooksPath=/dev/null",
		"-c",
		"core.untrackedCache=false",
		"-c",
		"core.quotepath=false",
		"-c",
		"color.ui=false",
		"-c",
		"diff.mnemonicPrefix=false",
		"-c",
		"diff.noprefix=false",
		"-c",
		"diff.external=",
	}
	if command == "diff" || command == "status" || command == "ls-files" {
		configArgs := append(
			append([]string{}, prefix...),
			"config",
			"--null",
			"--get-regexp",
			`^filter\..*\.(clean|process|smudge)$`,
		)
		var config bytes.Buffer
		_, _, err := runner.RunStreaming(ctx, root, func(out worktree.GitOutput) {
			if out.Stream == worktree.GitOutputStdout {
				config.Write(out.Chunk)
			}
		}, configArgs...)
		// git config exits 1 when no keys match; other failures must not weaken the policy.
		if err != nil {
			exit, ok := errors.AsType[*exec.ExitError](err)
			if !ok || exit.ExitCode() != 1 {
				return nil, err
			}
		}
		seen := map[string]bool{}
		for record := range strings.SplitSeq(config.String(), "\x00") {
			key, _, _ := strings.Cut(record, "\n")
			end := strings.LastIndexByte(key, '.')
			if !strings.HasPrefix(key, "filter.") || end <= 7 {
				continue
			}
			driver := key[:end]
			if seen[driver] {
				continue
			}
			seen[driver] = true
			for _, setting := range []string{".clean=", ".process=", ".smudge=", ".required=false"} {
				prefix = append(prefix, "-c", driver+setting)
			}
		}
	}
	prefix = append(prefix, command)
	switch command {
	case "diff":
		prefix = append(prefix, "--no-ext-diff", "--no-textconv")
	case "grep":
		prefix = append(prefix, "--no-textconv")
	}
	return append(prefix, args...), nil
}
func (r *FileReader) git(ctx context.Context, root, command string, args ...string) ([]byte, error) {
	argv, err := GitReadArgs(ctx, r.gitRunner, root, command, args...)
	if err != nil {
		return nil, err
	}
	var data bytes.Buffer
	_, _, err = r.gitRunner.RunStreaming(ctx, root, func(out worktree.GitOutput) {
		if out.Stream == worktree.GitOutputStdout {
			data.Write(out.Chunk)
		}
	}, argv...)
	return data.Bytes(), err
}

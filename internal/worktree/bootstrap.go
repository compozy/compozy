package worktree

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/procutil"
	"golang.org/x/sys/execabs"
)

const setupDiagnosticLimit = 16 * 1024

func (s *Service) copyBootstrapFiles(
	ctx context.Context,
	workspace Workspace,
	item Worktree,
	explicitPath bool,
) error {
	settings, root := s.worktreeSettings(workspace)
	if len(settings.CopyList) == 0 {
		return nil
	}
	if err := s.revalidateMutationPath(ctx, workspace, item, root, explicitPath); err != nil {
		return err
	}
	args := []string{"ls-files", "--others", "--ignored", "--exclude-standard", "-z", "--"}
	args = append(args, settings.CopyList...)
	stdout, stderr, err := s.runner.Run(ctx, workspace.Root, args...)
	if err != nil {
		return fmt.Errorf("worktree: enumerate bootstrap files: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	for _, raw := range bytes.Split(stdout, []byte{0}) {
		relative := string(raw)
		if relative == "" {
			continue
		}
		if err := s.revalidateMutationPath(ctx, workspace, item, root, explicitPath); err != nil {
			return err
		}
		if err := s.copyBootstrapFile(workspace.Root, item.Path, relative); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensurePlacementParent(
	ctx context.Context,
	workspace Workspace,
	item Worktree,
	placementRoot string,
	explicitPath bool,
) error {
	if err := s.revalidateMutationPath(ctx, workspace, item, placementRoot, explicitPath); err != nil {
		return err
	}
	parent := filepath.Dir(item.Path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return refusal(ErrConfigInvalid, "worktrees.root is not writable: "+err.Error())
	}
	if err := s.revalidateMutationPath(ctx, workspace, item, placementRoot, explicitPath); err != nil {
		return err
	}
	probe, err := os.CreateTemp(parent, ".compozy-write-probe-")
	if err != nil {
		return refusal(ErrConfigInvalid, "worktrees.root is not writable: "+err.Error())
	}
	probePath := probe.Name()
	if cleanupErr := errors.Join(probe.Close(), os.Remove(probePath)); cleanupErr != nil {
		return fmt.Errorf("worktree: clean placement write probe: %w", cleanupErr)
	}
	return nil
}

func (s *Service) copyBootstrapFile(sourceRoot, targetRoot, relative string) error {
	source := filepath.Join(sourceRoot, filepath.Clean(relative))
	_, canonicalSource, err := fileutil.ResolveExistingPathWithinRoot(sourceRoot, source)
	if err != nil {
		return fmt.Errorf("worktree: resolve bootstrap source: %w", err)
	}
	canonicalTarget, err := fileutil.CanonicalExistingDirectory(targetRoot)
	if err != nil {
		return fmt.Errorf("worktree: resolve bootstrap target: %w", err)
	}
	insideTarget, err := fileutil.PathWithinRoot(canonicalTarget, canonicalSource)
	if err != nil {
		return fmt.Errorf("worktree: compare bootstrap roots: %w", err)
	}
	if insideTarget {
		return nil
	}
	info, err := os.Lstat(canonicalSource)
	if err != nil {
		return fmt.Errorf("worktree: inspect bootstrap source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	destination := filepath.Join(canonicalTarget, filepath.Clean(relative))
	contained, err := fileutil.PathWithinRoot(canonicalTarget, destination)
	if err != nil || !contained {
		return refusal(ErrConfigInvalid, "worktrees.copy_list escaped the target root")
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("worktree: create bootstrap parent: %w", err)
	}
	canonicalParent, err := fileutil.CanonicalExistingDirectory(filepath.Dir(destination))
	if err != nil {
		return fmt.Errorf("worktree: resolve bootstrap destination parent: %w", err)
	}
	parentContained, err := fileutil.PathWithinRoot(canonicalTarget, canonicalParent)
	if err != nil || !parentContained {
		return refusal(ErrConfigInvalid, "worktrees.copy_list destination escaped the target root")
	}
	destination = filepath.Join(canonicalParent, filepath.Base(destination))
	input, err := os.Open(canonicalSource)
	if err != nil {
		return fmt.Errorf("worktree: open bootstrap source: %w", err)
	}
	defer func() {
		if closeErr := input.Close(); closeErr != nil {
			s.logger.Warn("worktree bootstrap source close failed", "error", closeErr)
		}
	}()
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if errors.Is(err, os.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("worktree: create bootstrap destination: %w", err)
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	return errors.Join(copyErr, closeErr)
}

func (s *Service) runSetup(
	ctx context.Context,
	workspace Workspace,
	item Worktree,
	explicitPath bool,
) (SetupState, string) {
	settings, root := s.worktreeSettings(workspace)
	command := strings.TrimSpace(settings.SetupCommand)
	if command == "" {
		return SetupNone, ""
	}
	if err := s.revalidateMutationPath(ctx, workspace, item, root, explicitPath); err != nil {
		return SetupFailed, diagnostics.RedactAndBound(err.Error(), setupDiagnosticLimit)
	}
	shell := strings.TrimSpace(s.getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}
	setupCtx, cancel := context.WithTimeout(ctx, settings.SetupTimeout)
	defer cancel()
	cmd := execabs.CommandContext(setupCtx, shell, "-c", command)
	cmd.Dir = item.Path
	cmd.Stdin = nil
	cmd.Env = setupEnvironment(s.environ(), workspace, item, shell)
	procutil.ConfigureCommandProcessGroup(cmd)
	stdout := newCappedBuffer(setupDiagnosticLimit)
	stderr := newCappedBuffer(setupDiagnosticLimit)
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if err := cmd.Start(); err != nil {
		return SetupFailed, diagnostics.RedactAndBound(err.Error(), setupDiagnosticLimit)
	}
	if err := procutil.RegisterCommandProcessGroup(cmd); err != nil {
		killErr := procutil.KillCommandProcessGroupAndWait(cmd, time.Second)
		waitErr := cmd.Wait()
		return SetupFailed, diagnostics.RedactAndBound(errors.Join(err, killErr, waitErr).Error(), setupDiagnosticLimit)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- cmd.Wait() }()
	var runErr error
	select {
	case runErr = <-waitCh:
	case <-setupCtx.Done():
		killErr := procutil.KillCommandProcessGroupAndWait(cmd, time.Second)
		runErr = errors.Join(setupCtx.Err(), killErr, <-waitCh)
	}
	if runErr == nil {
		return SetupOK, ""
	}
	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		detail = strings.TrimSpace(stdout.String())
	}
	if detail == "" {
		return SetupFailed, diagnostics.RedactAndBound(runErr.Error(), setupDiagnosticLimit)
	}
	return SetupFailed, diagnostics.RedactAndBound(fmt.Sprintf("%v: %s", runErr, detail), setupDiagnosticLimit)
}

func setupEnvironment(parent []string, workspace Workspace, item Worktree, shell string) []string {
	allowed := make([]string, 0, len(parent)+5)
	for _, entry := range parent {
		name, _, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		upper := strings.ToUpper(strings.TrimSpace(name))
		if upper == "PATH" || upper == "HOME" || upper == "LANG" || upper == "TMPDIR" || strings.HasPrefix(upper, "LC_") {
			allowed = append(allowed, entry)
		}
	}
	return append(allowed,
		"SHELL="+shell,
		"COMPOZY_WORKSPACE_ROOT="+workspace.Root,
		"COMPOZY_WORKTREE_PATH="+item.Path,
		"COMPOZY_WORKTREE_ID="+item.ID,
		"COMPOZY_WORKTREE_BRANCH="+item.Branch,
	)
}

func (s *Service) validateCreatePath(
	ctx context.Context,
	workspace Workspace,
	candidate string,
	explicit bool,
	currentID string,
) error {
	canonicalCandidate, err := canonicalFuturePath(candidate)
	if err != nil {
		if !explicit {
			return refusal(ErrConfigInvalid, "worktrees.root cannot resolve the derived path: "+err.Error())
		}
		return refusal(ErrPathExists, "explicit path cannot be resolved: "+err.Error())
	}
	_, placementRoot := s.worktreeSettings(workspace)
	if !explicit {
		canonicalRoot, rootErr := canonicalFuturePath(placementRoot)
		contained, err := fileutil.PathWithinRoot(canonicalRoot, canonicalCandidate)
		if rootErr != nil || err != nil || !contained {
			return refusal(ErrConfigInvalid, "worktrees.root does not contain the derived path")
		}
	}
	workspaces := []Workspace{workspace}
	if s.resolver != nil {
		workspaces, err = s.resolver.ListWorktreeWorkspaces(ctx)
		if err != nil {
			return fmt.Errorf("worktree: list workspace roots for containment: %w", err)
		}
	}
	for _, registered := range workspaces {
		workspaceRoot, rootErr := canonicalFuturePath(registered.Root)
		if rootErr != nil {
			return fmt.Errorf("worktree: resolve workspace root: %w", rootErr)
		}
		if explicit && pathsOverlap(workspaceRoot, canonicalCandidate) {
			return refusal(ErrPathExists, "explicit path overlaps a workspace root")
		}
		rows, listErr := s.store.List(ctx, registered.ID)
		if listErr != nil {
			return fmt.Errorf("worktree: list paths for containment: %w", listErr)
		}
		for _, row := range rows {
			if row.ID == currentID || row.State == StateRemoved || row.State == StateDismissed {
				continue
			}
			rowPath, pathErr := canonicalFuturePath(row.Path)
			if pathErr != nil {
				return fmt.Errorf("worktree: resolve registered worktree path: %w", pathErr)
			}
			if pathsOverlap(rowPath, canonicalCandidate) {
				return refusal(ErrPathExists, "path overlaps an existing worktree")
			}
		}
	}
	return nil
}

func (s *Service) revalidateMutationPath(
	ctx context.Context,
	workspace Workspace,
	item Worktree,
	placementRoot string,
	explicitPath bool,
) error {
	canonicalRoot, rootErr := canonicalFuturePath(placementRoot)
	canonicalPath, pathErr := canonicalFuturePath(item.Path)
	if rootErr != nil || pathErr != nil {
		return refusal(ErrConfigInvalid, "worktree path cannot be canonicalized")
	}
	if !explicitPath {
		derived, err := fileutil.PathWithinRoot(canonicalRoot, canonicalPath)
		if err != nil {
			return fmt.Errorf("worktree: compare placement root: %w", err)
		}
		if !derived {
			return refusal(ErrConfigInvalid, "worktrees.root no longer contains the derived path")
		}
	}
	return s.validateCreatePath(ctx, workspace, item.Path, explicitPath, item.ID)
}

func canonicalFuturePath(value string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(value))
	if err != nil {
		return "", err
	}
	current := absolute
	var remainder []string
	for {
		resolved, resolveErr := filepath.EvalSymlinks(current)
		if resolveErr == nil {
			for index := len(remainder) - 1; index >= 0; index-- {
				resolved = filepath.Join(resolved, remainder[index])
			}
			return filepath.Clean(resolved), nil
		}
		if !errors.Is(resolveErr, os.ErrNotExist) {
			return "", resolveErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", resolveErr
		}
		remainder = append(remainder, filepath.Base(current))
		current = parent
	}
}

func pathsOverlap(first, second string) bool {
	firstContains, firstErr := fileutil.PathWithinRoot(filepath.Clean(first), filepath.Clean(second))
	secondContains, secondErr := fileutil.PathWithinRoot(filepath.Clean(second), filepath.Clean(first))
	return firstErr == nil && secondErr == nil && (firstContains || secondContains)
}

//go:build e2e

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
)

const (
	currentVersion = "0.3.0"
	nextVersion    = "0.3.1"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			panic(errors.Join(err, writeErr))
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("runtimeupdate", flag.ContinueOnError)
	home := flags.String("home", "", "isolated CompozyOS home")
	replacement := flags.String("replacement", "", "replacement runtime path")
	delay := flags.Duration("phase-delay", 2*time.Second, "observable phase delay")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("runtime update fixture: parse flags: %w", err)
	}
	if *home == "" || *replacement == "" || *delay < 0 {
		return errors.New("runtime update fixture: home, replacement, and a non-negative phase delay are required")
	}
	paths, err := compozyconfig.ResolveHomePathsFrom(*home)
	if err != nil {
		return fmt.Errorf("runtime update fixture: resolve home: %w", err)
	}
	target := filepath.Join(paths.BinDir, runtimeBinaryName())
	digest, err := fileDigest(*replacement)
	if err != nil {
		return err
	}
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		return fmt.Errorf("runtime update fixture: resolve holder identity: %w", err)
	}
	generation, err := compozyupdate.NewExecutorGeneration()
	if err != nil {
		return fmt.Errorf("runtime update fixture: create executor generation: %w", err)
	}
	store, err := compozyupdate.NewOperationStore(paths, nil)
	if err != nil {
		return fmt.Errorf("runtime update fixture: open operation store: %w", err)
	}
	operation, err := store.Acquire(ctx, compozyupdate.OperationRequest{
		RequestedBy: compozyupdate.ActorWeb,
		Targets:     []compozyupdate.Target{compozyupdate.TargetRuntime},
		Runtime: &compozyupdate.RuntimeOperationState{
			ArtifactIdentity: compozyupdate.ArtifactIdentity{
				FromVersion: currentVersion,
				ToVersion:   nextVersion,
				ReleaseTag:  "v" + nextVersion,
				Asset:       filepath.Base(*replacement),
				Digest:      "sha256:" + digest,
			},
			InstallMethod: compozyupdate.InstallMethodDesktopApp,
			Phase:         compozyupdate.PhasePending,
		},
		Holder: compozyupdate.Holder{
			PID:                os.Getpid(),
			PIDStartTime:       startedAt,
			Surface:            compozyupdate.ActorDaemon,
			ExecutorGeneration: generation,
			LeaseExpiresAt:     time.Now().UTC().Add(10 * time.Minute),
		},
		Deadline: time.Now().UTC().Add(30 * time.Minute),
	})
	if err != nil {
		return fmt.Errorf("runtime update fixture: acquire operation: %w", err)
	}
	if _, err := fmt.Fprintln(os.Stdout, operation.ID); err != nil {
		return fmt.Errorf("runtime update fixture: publish operation id: %w", err)
	}

	for _, transition := range []compozyupdate.Transition{
		{Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon, Target: compozyupdate.TargetRuntime, Phase: compozyupdate.PhaseDownloading, Percent: 0, Outcome: "started"},
		{Kind: compozyupdate.TransitionProgress, Actor: compozyupdate.ActorDaemon, Target: compozyupdate.TargetRuntime, Percent: 65, Outcome: "progress"},
		{Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon, Target: compozyupdate.TargetRuntime, Phase: compozyupdate.PhaseVerifying, Percent: -1, Outcome: "started"},
	} {
		time.Sleep(*delay)
		operation, err = store.Transition(ctx, operation.ID, generation, operation.Revision, transition)
		if err != nil {
			return fmt.Errorf("runtime update fixture: advance to %q: %w", transition.Phase, err)
		}
	}

	backup := target + ".e2e-backup"
	time.Sleep(*delay)
	operation, err = store.Transition(ctx, operation.ID, generation, operation.Revision, compozyupdate.Transition{
		Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon, Target: compozyupdate.TargetRuntime,
		Phase: compozyupdate.PhaseSwapping, Percent: -1, BackupPath: backup, Outcome: "started",
	})
	if err != nil {
		return fmt.Errorf("runtime update fixture: journal swap: %w", err)
	}
	if err := swapRuntime(target, *replacement, backup); err != nil {
		return err
	}
	time.Sleep(*delay)
	operation, err = store.Transition(ctx, operation.ID, generation, operation.Revision, compozyupdate.Transition{
		Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon, Target: compozyupdate.TargetRuntime,
		Phase: compozyupdate.PhaseRestarting, Percent: -1, Outcome: "started",
	})
	if err != nil {
		return fmt.Errorf("runtime update fixture: journal restart: %w", err)
	}
	time.Sleep(*delay)

	command := exec.CommandContext(
		ctx,
		target,
		"daemon",
		"update-coordinator",
		"--operation-id",
		operation.ID,
		"--executor-generation",
		generation,
	)
	command.Env = os.Environ()
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("runtime update fixture: run coordinator: %w", err)
	}
	return nil
}

func runtimeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "compozy.exe"
	}
	return "compozy"
}

func fileDigest(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("runtime update fixture: read replacement: %w", err)
	}
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:]), nil
}

func swapRuntime(target string, replacement string, backup string) error {
	if err := os.Rename(target, backup); err != nil {
		return fmt.Errorf("runtime update fixture: move current runtime to backup: %w", err)
	}
	if err := os.Rename(replacement, target); err != nil {
		rollbackErr := os.Rename(backup, target)
		return errors.Join(
			fmt.Errorf("runtime update fixture: install replacement runtime: %w", err),
			rollbackErr,
		)
	}
	if err := os.Chmod(target, 0o700); err != nil {
		return fmt.Errorf("runtime update fixture: make replacement executable: %w", err)
	}
	return nil
}

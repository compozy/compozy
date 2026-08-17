//go:build e2e

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
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
	flags := flag.NewFlagSet("appstage", flag.ContinueOnError)
	home := flags.String("home", "", "isolated CompozyOS home")
	fromVersion := flags.String("from-version", "", "installed app version")
	toVersion := flags.String("to-version", "", "target app version")
	asset := flags.String("asset", "", "recorded updater asset")
	digest := flags.String("digest", "", "recorded updater digest")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("app stage fixture: parse flags: %w", err)
	}
	if *home == "" || *fromVersion == "" || *toVersion == "" || *asset == "" || *digest == "" {
		return errors.New("app stage fixture: home and complete artifact identity are required")
	}
	paths, err := compozyconfig.ResolveHomePathsFrom(*home)
	if err != nil {
		return fmt.Errorf("app stage fixture: resolve home: %w", err)
	}
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		return fmt.Errorf("app stage fixture: resolve holder identity: %w", err)
	}
	generation, err := compozyupdate.NewExecutorGeneration()
	if err != nil {
		return fmt.Errorf("app stage fixture: create executor generation: %w", err)
	}
	attemptID, err := compozyupdate.NewExecutorGeneration()
	if err != nil {
		return fmt.Errorf("app stage fixture: create attempt id: %w", err)
	}
	store, err := compozyupdate.NewOperationStore(paths, nil)
	if err != nil {
		return fmt.Errorf("app stage fixture: open operation store: %w", err)
	}
	operation, err := store.Acquire(ctx, compozyupdate.OperationRequest{
		RequestedBy: compozyupdate.ActorWeb,
		Targets:     []compozyupdate.Target{compozyupdate.TargetApp},
		App: &compozyupdate.AppOperationState{
			ArtifactIdentity: compozyupdate.ArtifactIdentity{
				FromVersion: *fromVersion,
				ToVersion:   *toVersion,
				ReleaseTag:  "v" + *toVersion,
				Asset:       *asset,
				Digest:      *digest,
			},
			AttemptID: attemptID,
			Phase:     compozyupdate.PhasePending,
		},
		Holder: compozyupdate.Holder{
			PID:                os.Getpid(),
			PIDStartTime:       startedAt,
			Surface:            compozyupdate.ActorDaemon,
			ExecutorGeneration: generation,
			LeaseExpiresAt:     time.Now().UTC().Add(2 * time.Minute),
		},
		Deadline: time.Now().UTC().Add(30 * time.Minute),
	})
	if err != nil {
		return fmt.Errorf("app stage fixture: acquire operation: %w", err)
	}
	operation, err = store.Transition(ctx, operation.ID, generation, operation.Revision, compozyupdate.Transition{
		Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon,
		Target: compozyupdate.TargetApp, Phase: compozyupdate.PhaseStaged, Percent: 100, Outcome: "staged",
	})
	if err != nil {
		return fmt.Errorf("app stage fixture: stage operation: %w", err)
	}
	_, err = store.Transition(ctx, operation.ID, generation, operation.Revision, compozyupdate.Transition{
		Kind: compozyupdate.TransitionWaitForApp, Actor: compozyupdate.ActorDaemon,
		Target: compozyupdate.TargetApp, Percent: -1, Outcome: "waiting",
	})
	if err != nil {
		return fmt.Errorf("app stage fixture: release operation to shell: %w", err)
	}
	return nil
}

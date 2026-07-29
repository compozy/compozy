package extensionpkg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/compozy/compozy/internal/procutil"
)

const (
	buildCommandShutdownGrace    = 500 * time.Millisecond
	buildCommandProcessGroupWait = time.Second
)

type buildCommandOutput struct {
	Stdout []byte
	Stderr []byte
}

type buildCommandRunner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, dir string, argv []string) (buildCommandOutput, error)
}

type osBuildCommandRunner struct{}

func (osBuildCommandRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (osBuildCommandRunner) Run(
	ctx context.Context,
	dir string,
	argv []string,
) (buildCommandOutput, error) {
	if ctx == nil {
		return buildCommandOutput{}, errors.New("extension: command context is required")
	}
	if len(argv) == 0 || argv[0] == "" {
		return buildCommandOutput{}, fmt.Errorf("extension: command is required")
	}
	// #nosec G204 -- commands are selected from detected toolchains or explicit operator input.
	command := exec.CommandContext(context.WithoutCancel(ctx), argv[0], argv[1:]...)
	command.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	configureBuildCommand(command)
	if err := command.Start(); err != nil {
		return buildCommandOutput{}, fmt.Errorf("extension: start build command: %w", err)
	}
	if err := procutil.RegisterCommandProcessGroup(command); err != nil {
		return buildCommandOutput{}, errors.Join(
			fmt.Errorf("extension: register build command process group: %w", err),
			cleanupStartedBuildCommand(command),
		)
	}

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- command.Wait()
	}()

	select {
	case waitErr := <-waitCh:
		groupErr := forceBuildCommandExit(command, buildCommandProcessGroupWait)
		output := buildCommandOutput{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
		if waitErr != nil {
			return output, errors.Join(
				fmt.Errorf("%w (stdout: %s; stderr: %s)", waitErr, stdout.String(), stderr.String()),
				groupErr,
			)
		}
		if groupErr != nil {
			return output, fmt.Errorf("extension: clean build command process group: %w", groupErr)
		}
		return output, nil
	case <-ctx.Done():
		shutdownErr := stopBuildCommand(command, waitCh)
		return buildCommandOutput{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, errors.Join(ctx.Err(), shutdownErr)
	}
}

func cleanupStartedBuildCommand(command *exec.Cmd) error {
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- command.Wait()
	}()
	return stopBuildCommand(command, waitCh)
}

func stopBuildCommand(command *exec.Cmd, waitCh <-chan error) error {
	terminateErr := terminateBuildCommand(command)
	timer := time.NewTimer(buildCommandShutdownGrace)
	defer timer.Stop()

	select {
	case waitErr := <-waitCh:
		return errors.Join(
			terminateErr,
			waitErr,
			forceBuildCommandExit(command, buildCommandProcessGroupWait),
		)
	case <-timer.C:
		killErr := killBuildCommand(command)
		waitErr := <-waitCh
		return errors.Join(
			terminateErr,
			killErr,
			waitErr,
			forceBuildCommandExit(command, buildCommandProcessGroupWait),
		)
	}
}

package acp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	contract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subprocess"
)

const defaultNPXPrepareTimeout = 5 * time.Minute

// npm owns cache repair; serialize preparation so session starts do not race installs.
var npxPrepareSlot = make(chan struct{}, 1)

// Recognize resolved runner paths on Unix and Windows.
func isNPX(command string) bool {
	base := strings.ToLower(filepath.Base(command))
	return base == "npx" || base == "npx.cmd" || base == "npx.exe"
}

// A no-op call resolves packages without starting the adapter or its ACP deadline.
func npxPrepareArgs(args []string) ([]string, string) {
	var positional string
	var flags []string
	hasPackage := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if slices.Contains([]string{"--no-install", "--no", "--yes=false"}, arg) {
			return nil, ""
		}
		if arg == "--" {
			if !hasPackage && i+1 < len(args) {
				positional = args[i+1]
				flags = append(flags, "--package="+positional)
			}
			break
		}
		if !strings.HasPrefix(arg, "-") {
			if !hasPackage {
				positional = arg
				flags = append(flags, "--package="+arg)
			}
			break
		}
		if arg == "-c" || arg == "--call" || strings.HasPrefix(arg, "--call=") {
			if !hasPackage {
				return nil, ""
			}
			break
		}
		flags = append(flags, arg)
		name, _, hasValue := strings.Cut(arg, "=")
		if name == "-p" || name == "--package" {
			hasPackage = true
		}
		if !hasValue {
			switch name {
			case "-y",
				"--yes",
				"--no",
				"--no-install",
				"--quiet",
				"-q",
				"--offline",
				"--prefer-offline",
				"--prefer-online":
			case "-p",
				"--package",
				"--cache",
				"--registry",
				"--userconfig",
				"--prefix",
				"--script-shell",
				"--loglevel",
				"-w",
				"--workspace":
				i++
				if i >= len(args) {
					return nil, ""
				}
				flags = append(flags, args[i])
			default:
				return nil, ""
			}
		}
	}
	if len(flags) == 0 {
		return nil, ""
	}
	return append(flags, "--call", "exit 0"), positional
}

// A binary name can be supplied by a differently named local package.
func npxLocalBinary(name string, spec LaunchSpec) bool {
	if name == "" || strings.ContainsAny(name, "@/\\") {
		return false
	}
	for dir := spec.Cwd; dir != ""; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, "node_modules", ".bin", name)
		if _, err := subprocess.ResolveExecutable(candidate, spec.Env, spec.Cwd); err == nil {
			return true
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	_, err := subprocess.ResolveExecutable(name, spec.Env, spec.Cwd)
	return err == nil
}

// NPXStartupDiagnostic recovers preparation guidance from the persisted failure boundary.
func NPXStartupDiagnostic(failure *store.SessionFailure) (contract.DiagnosticItem, bool) {
	var empty contract.DiagnosticItem
	if failure == nil || (failure.Kind != store.FailureStartup && failure.Kind != store.FailureHandshake &&
		failure.Kind != store.FailureTimeout) {
		return empty, false
	}
	var stage string
	switch failure.ReasonCode {
	case store.FailureReasonNPXPrepare:
		stage = "prepare"
	case store.FailureReasonNPXInitialize:
		stage = "initialize"
	case store.FailureReasonNPXResolve:
		stage = "resolve"
	default:
		return empty, false
	}
	item, ok := diagnostics.ItemFromError(npxDiagnostic(stage, nil))
	item.DataFreshness = contract.FreshnessStale
	return item, ok
}

// Persist origin independently of redacted, truncated failure text.
func npxFailureReason(err error) store.FailureReasonCode {
	item, ok := diagnostics.ItemFromError(err)
	if !ok {
		return ""
	}
	switch item.ID {
	case "provider.adapter.prepare":
		return store.FailureReasonNPXPrepare
	case "provider.adapter.initialize":
		return store.FailureReasonNPXInitialize
	case "provider.adapter.resolve":
		return store.FailureReasonNPXResolve
	default:
		return ""
	}
}

// Package installation must finish before the ACP handshake budget starts.
func (l *localLauncher) prepareNPX(ctx context.Context, spec LaunchSpec) error {
	if !isNPX(spec.ResolvedExecutable) {
		return nil
	}
	args, positional := npxPrepareArgs(spec.Args)
	if args == nil || npxLocalBinary(positional, spec) {
		return nil
	}
	prepareCtx, cancel := context.WithTimeout(ctx, defaultNPXPrepareTimeout)
	defer cancel()
	select {
	case npxPrepareSlot <- struct{}{}:
		defer func() { <-npxPrepareSlot }()
	case <-prepareCtx.Done():
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return npxDiagnostic("prepare", prepareCtx.Err())
	}
	started := time.Now()
	err := l.runNPXPreparation(prepareCtx, spec, args)
	l.logger.Info("acp.start.stage", "stage", "adapter_prepare", "outcome", stageOutcome(err, false),
		"duration_ms", time.Since(started).Milliseconds())
	if err != nil {
		if ctx.Err() != nil {
			return errors.Join(ctx.Err(), err)
		}
		return npxDiagnostic("prepare", err)
	}
	return nil
}

// Managed shutdown owns the installer tree and joins its output drain.
func (l *localLauncher) runNPXPreparation(ctx context.Context, spec LaunchSpec, args []string) error {
	process, err := subprocess.Launch(ctx, subprocess.LaunchConfig{
		Command: spec.ResolvedExecutable, Executable: spec.ResolvedExecutable,
		Args: args, Dir: spec.Cwd, Env: spec.Env, Logger: l.logger,
		DisableTransport: true, ShutdownTimeout: l.stopTimeout, Niceness: l.niceness,
	})
	if err != nil {
		return err
	}
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		if _, copyErr := io.Copy(io.Discard, process.Stdout()); copyErr != nil {
			l.logger.Debug("acp: preparation output closed")
		}
	}()
	defer func() { <-drained }()
	select {
	case <-process.Done():
		return process.Wait()
	case <-ctx.Done():
		stopCtx, cancel := context.WithTimeout(context.Background(), l.stopTimeout)
		defer cancel()
		return errors.Join(ctx.Err(), process.Shutdown(stopCtx))
	}
}

// Keep permanent package errors distinct from network and deadline failures.
func npxDiagnostic(stage string, cause error) error {
	if stage == "prepare" && cause != nil {
		for _, code := range []string{"E404", "ETARGET", "EINVALIDTAGNAME", "EINVALIDPACKAGENAME"} {
			if strings.Contains(cause.Error(), "code "+code) {
				stage = "resolve"
				break
			}
		}
	}
	code := contract.CodeProviderTransientFailure
	message := "npx adapter package preparation failed. Check npm registry connectivity and permissions, " +
		"then retry the session; npm repairs abandoned install locks automatically. Do not clear the npm cache."
	if stage == "initialize" {
		message = "npx launched but ACP initialize timed out. Retry the session; if it persists, " +
			"run the configured adapter command locally to finish or diagnose npm installation, " +
			"then retry without restarting the daemon."
	}
	if stage == "resolve" {
		code = contract.CodeProviderClassificationUnknown
		message = "npx could not resolve the configured package or version. Correct the provider command, " +
			"package name, version, or registry configuration before starting the session again."
	}
	item := diagnostics.NewItem(diagnostics.ItemSpec{
		ID: "provider.adapter." + stage, Code: code,
		Category: contract.CategoryProvider, Severity: contract.SeverityWarn,
		Title: "ACP adapter " + stage + " failed", Message: message,
		DataFreshness: contract.FreshnessLive,
	}, diagnostics.WithEvidence(map[string]any{"stage": stage, "runner": "npx"}))
	return diagnostics.NewStructuredError(item, fmt.Errorf("acp: npx %s: %w", stage, cause))
}

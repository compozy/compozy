package daytona

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/compozy/agh/internal/sandbox"
)

func (p *daytonaProvider) SyncToRuntime(
	ctx context.Context,
	state sandbox.SessionState,
	opts sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	if ctx == nil {
		return sandbox.SyncResult{}, fmt.Errorf("sandbox/daytona: sync to runtime context is required")
	}
	if state.Backend != sandbox.BackendDaytona {
		return sandbox.SyncResult{}, fmt.Errorf("sandbox/daytona: sync to runtime backend = %q", state.Backend)
	}
	providerState, err := decodeProviderState(state.ProviderState)
	if err != nil {
		return sandbox.SyncResult{}, err
	}
	if providerState.LocalRootDir == "" || providerState.RuntimeRootDir == "" {
		return sandbox.SyncResult{}, fmt.Errorf("sandbox/daytona: sync to runtime missing root mapping")
	}
	roots := append(
		[]syncRoot{{local: providerState.LocalRootDir, runtime: providerState.RuntimeRootDir}},
		additionalSyncRoots(providerState.LocalAdditionalDirs, providerState.RuntimeAdditionalDirs)...,
	)
	sandboxInfo := sandboxInfo{
		ID:                 providerState.SandboxID,
		APIURL:             providerState.APIURL,
		SSHHost:            providerState.SSHHost,
		SSHAccessExpiresAt: state.SSHAccessExpiresAt,
	}
	result := sandbox.SyncResult{}
	for _, root := range roots {
		stats, err := p.syncOneToRuntime(ctx, sandboxInfo, root, opts)
		result.FilesSynced += stats.FilesSynced
		result.BytesTransferred += stats.BytesTransferred
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return result, err
		}
	}
	return result, nil
}

func (p *daytonaProvider) SyncFromRuntime(
	ctx context.Context,
	state sandbox.SessionState,
	opts sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	if ctx == nil {
		return sandbox.SyncResult{}, fmt.Errorf("sandbox/daytona: sync from runtime context is required")
	}
	if state.Backend != sandbox.BackendDaytona {
		return sandbox.SyncResult{}, fmt.Errorf(
			"sandbox/daytona: sync from runtime backend = %q",
			state.Backend,
		)
	}
	providerState, err := decodeProviderState(state.ProviderState)
	if err != nil {
		return sandbox.SyncResult{}, err
	}
	if providerState.LocalRootDir == "" || providerState.RuntimeRootDir == "" {
		return sandbox.SyncResult{}, fmt.Errorf("sandbox/daytona: sync from runtime missing root mapping")
	}
	roots := append(
		[]syncRoot{{local: providerState.LocalRootDir, runtime: providerState.RuntimeRootDir}},
		additionalSyncRoots(providerState.LocalAdditionalDirs, providerState.RuntimeAdditionalDirs)...,
	)
	sandboxInfo := sandboxInfo{
		ID:                 providerState.SandboxID,
		APIURL:             providerState.APIURL,
		SSHHost:            providerState.SSHHost,
		SSHAccessExpiresAt: state.SSHAccessExpiresAt,
	}
	result := sandbox.SyncResult{}
	for _, root := range roots {
		stats, err := p.syncOneFromRuntime(ctx, sandboxInfo, root, opts)
		result.FilesSynced += stats.FilesSynced
		result.BytesTransferred += stats.BytesTransferred
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return result, err
		}
	}
	return result, nil
}

type syncRoot struct {
	local   string
	runtime string
}

func additionalSyncRoots(localDirs []string, runtimeDirs []string) []syncRoot {
	limit := min(len(runtimeDirs), len(localDirs))
	roots := make([]syncRoot, 0, limit)
	for i := range limit {
		roots = append(roots, syncRoot{local: localDirs[i], runtime: runtimeDirs[i]})
	}
	return roots
}

func (p *daytonaProvider) syncOneToRuntime(
	ctx context.Context,
	sandboxInfo sandboxInfo,
	root syncRoot,
	opts sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	archive, stats, err := buildTarArchive(ctx, filepath.Clean(root.local), opts.ExcludePatterns)
	if err != nil {
		result := sandbox.SyncResult{}
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	defer func() {
		if closeErr := archive.Close(); closeErr != nil {
			p.logger.Warn("sandbox/daytona: close tar archive temp file failed", "error", closeErr)
		}
		if removeErr := os.Remove(archive.Name()); removeErr != nil {
			p.logger.Warn("sandbox/daytona: remove tar archive temp file failed", "error", removeErr)
		}
	}()
	info, err := archive.Stat()
	if err != nil {
		result := sandbox.SyncResult{}
		err = fmt.Errorf("sandbox/daytona: stat tar archive temp file: %w", err)
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}

	session, err := p.shellTransport.Dial(ctx, sandboxInfo, remoteExtractCommand(root.runtime, info.Size()))
	if err != nil {
		return sandbox.SyncResult{}, fmt.Errorf(
			"sandbox/daytona: open sync-to-runtime SSH stream for %q: %w",
			root.runtime,
			err,
		)
	}
	_, writeErr := io.Copy(session, archive)
	waitErr := session.Wait()
	if waitErr != nil {
		waitErr = fmt.Errorf(
			"sandbox/daytona: remote extract %q failed: %w stderr=%q",
			root.runtime,
			waitErr,
			session.Stderr(),
		)
	}
	if err := session.Close(); err != nil {
		p.logger.Warn("sandbox/daytona: close sync-to-runtime SSH session failed", "error", err)
	}
	result := sandbox.SyncResult{FilesSynced: stats.Files, BytesTransferred: stats.Bytes}
	if err := joinSyncErrors(writeErr, waitErr); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	p.logger.Info(
		"sandbox/daytona: synced workspace to runtime",
		"reason",
		string(opts.Reason),
		"local",
		root.local,
		"runtime",
		root.runtime,
		"files",
		stats.Files,
		"bytes",
		stats.Bytes,
	)
	return result, nil
}

func (p *daytonaProvider) syncOneFromRuntime(
	ctx context.Context,
	sandboxInfo sandboxInfo,
	root syncRoot,
	opts sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	session, err := p.shellTransport.Dial(ctx, sandboxInfo, remoteArchiveCommand(root.runtime))
	if err != nil {
		return sandbox.SyncResult{}, fmt.Errorf(
			"sandbox/daytona: open sync-from-runtime SSH stream for %q: %w",
			root.runtime,
			err,
		)
	}
	stats, extractErr := extractTar(filepath.Clean(root.local), session)
	var waitErr error
	if extractErr != nil {
		waitErr = session.Stop(ctx)
	} else {
		waitErr = session.Wait()
	}
	if waitErr != nil {
		waitErr = fmt.Errorf(
			"sandbox/daytona: remote archive %q failed: %w stderr=%q",
			root.runtime,
			waitErr,
			session.Stderr(),
		)
	}
	if err := session.Close(); err != nil {
		p.logger.Warn("sandbox/daytona: close sync-from-runtime SSH session failed", "error", err)
	}
	result := sandbox.SyncResult{FilesSynced: stats.Files, BytesTransferred: stats.Bytes}
	if err := joinSyncErrors(extractErr, waitErr); err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result, err
	}
	p.logger.Info(
		"sandbox/daytona: synced runtime to workspace",
		"reason",
		string(opts.Reason),
		"local",
		root.local,
		"runtime",
		root.runtime,
		"files",
		stats.Files,
		"bytes",
		stats.Bytes,
	)
	return result, nil
}

func joinSyncErrors(errs ...error) error {
	return errors.Join(errs...)
}

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

var errRemoteDaemonNotRunning = errors.New("cli: remote CompozyOS daemon is not running")

type remoteDaemonResolution struct {
	status    DaemonStatus
	ownership *sshDaemonOwnership
	lease     sshOwnershipLease
}

func (r remoteDaemonResolution) started() bool { return r.ownership != nil && r.lease != nil }

func resolveRemoteDaemon(
	ctx context.Context,
	executor sshExecutor,
	target sshTarget,
	options connectSSHOptions,
	deps commandDeps,
) (remoteDaemonResolution, error) {
	status, err := readRemoteDaemonStatus(ctx, executor, target, options.remoteHome)
	if err == nil {
		owned, ownershipErr := inspectSSHDaemonOwnership(ctx, executor, target, options.remoteHome)
		if ownershipErr != nil {
			return remoteDaemonResolution{}, ownershipErr
		}
		if owned {
			return remoteDaemonResolution{}, newSSHBusyError()
		}
		return remoteDaemonResolution{status: status}, nil
	}
	if !errors.Is(err, errRemoteDaemonNotRunning) {
		if classified := classifySSHFailure(err); classified != err {
			return remoteDaemonResolution{}, classified
		}
		return remoteDaemonResolution{}, fmt.Errorf("cli: inspect remote daemon status: %w", err)
	}
	if !options.remoteStart {
		return remoteDaemonResolution{}, &gatewayClientError{
			code: sshUnreachableCode, statusCode: http.StatusServiceUnavailable,
			message: "remote CompozyOS daemon is not running and automatic start is disabled",
			cause:   err,
		}
	}
	ownership, err := sshDaemonOwnershipForToken(options.ownerToken)
	if err != nil {
		return remoteDaemonResolution{}, err
	}
	lease, err := executor.StartOwnershipLease(ctx, target, options.remoteHome, ownership)
	if err != nil {
		return remoteDaemonResolution{}, err
	}
	startOutput, err := executor.Run(
		ctx, target, remoteCompozyCommand(options.remoteHome, "daemon", "start", "-o", "json"),
	)
	if err != nil {
		status, statusErr := waitForOwnedRemoteDaemon(
			context.WithoutCancel(ctx), executor, target, options.remoteHome, deps,
		)
		if statusErr == nil {
			return remoteDaemonResolution{status: status, ownership: ownership, lease: lease}, nil
		}
		return remoteDaemonResolution{}, cleanupOwnedRemoteDaemon(
			ctx, deps, executor, target, options.remoteHome, ownership, lease, DaemonStatus{},
			errors.Join(classifySSHFailure(err), statusErr),
		)
	}
	var ownedStatus DaemonStatus
	ownedStatus, err = decodeRemoteDaemonStatus(startOutput)
	if err == nil {
		if identityErr := validateRemoteDaemonIdentity(ownedStatus); identityErr != nil {
			err = identityErr
		}
	}
	if err == nil {
		if listenerErr := validateRemoteDaemonListener(ownedStatus); listenerErr != nil {
			return remoteDaemonResolution{}, cleanupOwnedRemoteDaemon(
				ctx, deps, executor, target, options.remoteHome, ownership, lease, ownedStatus, listenerErr,
			)
		}
		return remoteDaemonResolution{status: ownedStatus, ownership: ownership, lease: lease}, nil
	}

	status, statusErr := waitForOwnedRemoteDaemon(ctx, executor, target, options.remoteHome, deps)
	if statusErr != nil {
		return remoteDaemonResolution{}, cleanupOwnedRemoteDaemon(
			ctx, deps, executor, target, options.remoteHome, ownership, lease, ownedStatus,
			errors.Join(err, statusErr),
		)
	}
	return remoteDaemonResolution{status: status, ownership: ownership, lease: lease}, nil
}

func sshDaemonOwnershipForToken(ownerToken string) (*sshDaemonOwnership, error) {
	token := strings.TrimSpace(ownerToken)
	if token != "" {
		return &sshDaemonOwnership{token: token}, nil
	}
	return newSSHDaemonOwnership(nil)
}

func waitForOwnedRemoteDaemon(
	ctx context.Context,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
	deps commandDeps,
) (DaemonStatus, error) {
	deadline := time.Now().Add(deps.startTimeout)
	for {
		status, err := readRemoteDaemonStatus(ctx, executor, target, remoteHome)
		if err == nil {
			if identityErr := validateRemoteDaemonIdentity(status); identityErr != nil {
				return DaemonStatus{}, identityErr
			}
			return status, nil
		}
		if time.Now().After(deadline) {
			return DaemonStatus{}, &gatewayClientError{
				code: sshUnreachableCode, statusCode: http.StatusServiceUnavailable,
				message: "remote CompozyOS daemon did not become ready before the deadline",
				cause:   err,
			}
		}
		select {
		case <-ctx.Done():
			return DaemonStatus{}, ctx.Err()
		case <-time.After(deps.pollInterval):
		}
	}
}

func readRemoteDaemonStatus(
	ctx context.Context,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
) (DaemonStatus, error) {
	status, err := readRemoteDaemonStatusRaw(ctx, executor, target, remoteHome)
	if err != nil {
		return DaemonStatus{}, err
	}
	statusState := strings.TrimSpace(status.Status)
	if statusState != "" && !strings.EqualFold(statusState, "running") {
		return DaemonStatus{}, errRemoteDaemonNotRunning
	}
	if err := validateRemoteDaemonIdentity(status); err != nil {
		return DaemonStatus{}, err
	}
	if err := validateRemoteDaemonListener(status); err != nil {
		return DaemonStatus{}, err
	}
	return status, nil
}

func readRemoteDaemonStatusRaw(
	ctx context.Context,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
) (DaemonStatus, error) {
	output, err := executor.Run(
		ctx,
		target,
		remoteCompozyCommand(remoteHome, "status", "-o", "json"),
	)
	if err != nil {
		if isRemoteDaemonUnavailable(err) {
			return DaemonStatus{}, errRemoteDaemonNotRunning
		}
		return DaemonStatus{}, err
	}
	var status StatusRecord
	if err := json.Unmarshal(output, &status); err != nil {
		return DaemonStatus{}, fmt.Errorf("cli: decode remote runtime status: %w", err)
	}
	return status.Daemon, nil
}

func isRemoteDaemonUnavailable(err error) bool {
	var commandErr *sshCommandError
	if !errors.As(err, &commandErr) {
		return false
	}
	var payload contract.ErrorPayload
	if err := json.Unmarshal([]byte(commandErr.output), &payload); err != nil {
		return false
	}
	if payload.Diagnostic == nil {
		return false
	}
	return payload.Diagnostic.Code == contract.CodeDaemonUnavailable
}

func validateRemoteDaemonLoopback(status DaemonStatus) error {
	host := strings.TrimSpace(status.HTTPHost)
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if ip != nil && ip.IsLoopback() {
		return nil
	}
	return errors.New("cli: refusing SSH reuse because the remote daemon HTTP listener is not loopback-only")
}

func decodeRemoteDaemonStatus(output []byte) (DaemonStatus, error) {
	var status DaemonStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return DaemonStatus{}, fmt.Errorf("cli: decode remote daemon status: %w", err)
	}
	return status, nil
}

func validateRemoteDaemonListener(status DaemonStatus) error {
	if strings.TrimSpace(status.HTTPHost) == "" || status.HTTPPort <= 0 || status.HTTPPort > 65535 {
		return errors.New("cli: remote daemon status has no valid HTTP listener")
	}
	return validateRemoteDaemonLoopback(status)
}

func validateRemoteDaemonIdentity(status DaemonStatus) error {
	if status.PID <= 0 || status.StartedAt.IsZero() {
		return errors.New("cli: remote daemon status has no stable process identity")
	}
	return nil
}

func waitForForwardedDaemon(ctx context.Context, deps commandDeps, target ClientTarget) error {
	client, err := deps.newClient(target)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(deps.startTimeout)
	for {
		if _, err = client.DaemonStatus(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return &gatewayClientError{
				code: sshUnreachableCode, statusCode: http.StatusServiceUnavailable,
				message: "SSH forward did not become ready before the deadline",
				cause:   err,
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(deps.pollInterval):
		}
	}
}

func stopOwnedRemoteDaemon(
	ctx context.Context,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
	expected DaemonStatus,
	ownership *sshDaemonOwnership,
) error {
	if ownership == nil || strings.TrimSpace(ownership.token) == "" {
		return errors.New("cli: SSH daemon ownership token is required before stop")
	}
	if err := validateRemoteDaemonIdentity(expected); err != nil {
		return fmt.Errorf("cli: verify SSH-owned remote daemon identity: %w", err)
	}
	current, err := readRemoteDaemonStatusRaw(ctx, executor, target, remoteHome)
	if err != nil {
		return fmt.Errorf("cli: read SSH-owned remote daemon before stop: %w", classifySSHFailure(err))
	}
	if current.PID != expected.PID || !current.StartedAt.Equal(expected.StartedAt) {
		return nil
	}
	output, err := executor.Run(
		ctx,
		target,
		sshOwnershipCommand(sshOwnershipStopScript, remoteHome, ownership.token),
	)
	if err != nil {
		return fmt.Errorf("cli: stop SSH-owned remote daemon: %w", classifySSHFailure(err))
	}
	if strings.TrimSpace(string(output)) == sshOwnershipNotOwner {
		return nil
	}
	return nil
}

func cleanupOwnedRemoteDaemon(
	ctx context.Context,
	deps commandDeps,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
	ownership *sshDaemonOwnership,
	lease sshOwnershipLease,
	expected DaemonStatus,
	cause error,
) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deps.stopTimeout)
	defer cancel()
	stopErr := stopSSHOwnedRemoteDaemon(stopCtx, executor, target, remoteHome, expected, ownership)
	closeErr := closeSSHOwnershipLease(lease)
	return errors.Join(cause, stopErr, closeErr)
}

func stopSSHOwnedRemoteDaemon(
	ctx context.Context,
	executor sshExecutor,
	target sshTarget,
	remoteHome string,
	expected DaemonStatus,
	ownership *sshDaemonOwnership,
) error {
	if err := validateRemoteDaemonIdentity(expected); err == nil {
		return stopOwnedRemoteDaemon(ctx, executor, target, remoteHome, expected, ownership)
	}
	current, err := readRemoteDaemonStatusRaw(ctx, executor, target, remoteHome)
	if err == nil {
		if identityErr := validateRemoteDaemonIdentity(current); identityErr != nil {
			return fmt.Errorf("cli: verify SSH-owned remote daemon identity: %w", identityErr)
		}
		return stopOwnedRemoteDaemon(ctx, executor, target, remoteHome, current, ownership)
	}
	if ownership == nil || strings.TrimSpace(ownership.token) == "" {
		return errors.New("cli: SSH daemon ownership token is required before cleanup")
	}
	_, stopErr := executor.Run(ctx, target, sshOwnershipCommand(sshOwnershipStopScript, remoteHome, ownership.token))
	if stopErr == nil {
		return nil
	}
	return errors.Join(
		fmt.Errorf("cli: read SSH-owned remote daemon before cleanup: %w", classifySSHFailure(err)),
		fmt.Errorf("cli: stop SSH-owned remote daemon after incomplete start: %w", classifySSHFailure(stopErr)),
	)
}

func persistSSHProfile(
	ctx context.Context,
	deps commandDeps,
	homePaths compozyconfig.HomePaths,
	profile compozyconfig.GatewayConnectionConfig,
	overwrite bool,
) (returnErr error) {
	lock, gatewayConfig, err := acquireRecoveredGatewayProfileState(ctx, homePaths, deps)
	if err != nil {
		return err
	}
	defer func() {
		if releaseErr := lock.Release(); releaseErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("cli: release gateway profile lock: %w", releaseErr))
		}
	}()
	if _, exists := findGatewayProfile(gatewayConfig.Connections, profile.Name); exists && !overwrite {
		return &gatewayClientError{
			code:       gatewayProfileConflictCode,
			statusCode: http.StatusConflict,
			message: fmt.Sprintf(
				"gateway profile %q already exists; choose another name or pass --overwrite",
				profile.Name,
			),
		}
	}
	return applyGatewayProfileState(deps, homePaths, profile.Name, &profile, gatewayConfig.ActiveConnection)
}

func remoteCompozyCommand(remoteHome string, arguments ...string) []string {
	command := make([]string, 0, len(arguments)+3)
	if home := strings.TrimSpace(remoteHome); home != "" {
		command = append(command, "env", "COMPOZY_HOME="+home)
	}
	command = append(command, "compozy")
	return append(command, arguments...)
}

func remoteCompozyLookupCommand() []string {
	return []string{"sh", "-c", "command -v compozy"}
}

func classifySSHFailure(err error) error {
	if err == nil {
		return nil
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "remote host identification has changed") ||
		strings.Contains(message, "host key verification failed") {
		return &gatewayClientError{
			code: sshHostKeyChangedCode, statusCode: http.StatusForbidden,
			message: "SSH host key verification failed; the changed key was not accepted",
			cause:   err,
		}
	}
	var commandErr *sshCommandError
	if errors.As(err, &commandErr) && commandErr.exitCode() == 255 {
		return &gatewayClientError{
			code: sshUnreachableCode, statusCode: http.StatusServiceUnavailable,
			message: "SSH host is unreachable or authentication failed",
			cause:   err,
		}
	}
	return err
}

func sshConnectionOutput(record sshConnectionRecord) outputBundle {
	return simpleGatewayOutput(record, func() string {
		return renderHumanSection("SSH Connection", []keyValue{
			{Label: "Profile", Value: record.Profile.Name},
			{Label: "Local URL", Value: record.LocalURL},
			{Label: "Remote HTTP", Value: record.RemoteHTTP},
			{Label: "Daemon", Value: record.Daemon},
		})
	}, func() string {
		return renderToonObject("ssh_connection", []string{
			"profile", "local_url", "remote_http", configDaemonKey,
		}, []string{
			record.Profile.Name,
			record.LocalURL,
			record.RemoteHTTP,
			record.Daemon,
		})
	})
}

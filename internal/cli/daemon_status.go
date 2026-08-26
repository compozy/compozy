package cli

import (
	"context"
	"errors"

	"os"

	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"

	"github.com/compozy/compozy/internal/version"
)

const daemonStopWaitMargin = time.Second

func waitForDaemonStop(
	ctx context.Context,
	deps commandDeps,
	runtime *runtimeContext,
	info compozydaemon.Info,
) (DaemonStatus, error) {
	if err := requirePollingContext(ctx); err != nil {
		return DaemonStatus{}, err
	}
	client, clientErr := clientFromDeps(deps)
	return pollUntil(
		ctx,
		daemonStopWaitTimeout(deps.stopTimeout, runtime.Config),
		deps.pollInterval,
		nil,
		"cli: daemon did not stop before timeout",
		func(pollCtx context.Context, _ pollEvent) (DaemonStatus, bool, error) {
			if _, running, err := daemonInfo(runtime.HomePaths, deps); err == nil && !running {
				return daemonStatusWithState(runtime, info, "stopped"), true, nil
			}
			if clientErr == nil {
				if _, err := client.DaemonStatus(pollCtx); err != nil {
					if _, running, infoErr := daemonInfo(runtime.HomePaths, deps); infoErr == nil && !running {
						return daemonStatusWithState(runtime, info, "stopped"), true, nil
					}
				}
			}
			return DaemonStatus{}, false, nil
		},
	)
}

func daemonStopWaitTimeout(configured time.Duration, config compozyconfig.Config) time.Duration {
	if configured != defaultStopTimeout {
		return configured
	}
	return max(
		configured,
		compozydaemon.GracefulShutdownTimeout(config)+daemonStopWaitMargin,
	)
}

func daemonStatusFromDeps(ctx context.Context, deps commandDeps, runtime *runtimeContext) (DaemonStatus, error) {
	client, err := clientFromDeps(deps)
	if err != nil {
		return DaemonStatus{}, err
	}
	status, statusErr := client.DaemonStatus(ctx)
	if statusErr == nil {
		return status, nil
	}
	if !daemonStatusCanFallback(statusErr) {
		return DaemonStatus{}, statusErr
	}

	info, running, err := daemonInfo(runtime.HomePaths, deps)
	if err != nil {
		return DaemonStatus{}, err
	}
	if !running {
		return daemonStatusWithState(runtime, info, "stopped"), nil
	}
	return daemonStatusWithState(runtime, info, "starting"), nil
}

func daemonStatusCanFallback(err error) bool {
	return isDaemonUnavailableTransportError(err)
}

func daemonInfo(homePaths compozyconfig.HomePaths, deps commandDeps) (compozydaemon.Info, bool, error) {
	info, err := deps.readDaemonInfo(homePaths.DaemonInfo)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		return compozydaemon.Info{}, false, nil
	default:
		return compozydaemon.Info{}, false, err
	}

	if !deps.processAlive(info.PID) {
		return info, false, nil
	}
	if !deps.processMatchesStartTime(info.PID, info.StartedAt) {
		return info, false, nil
	}
	return info, true, nil
}

func daemonStatusWithState(runtime *runtimeContext, info compozydaemon.Info, status string) DaemonStatus {
	networkStatus := daemonNetworkStatusFromInfo(&runtime.Config, info.Network)
	if strings.EqualFold(strings.TrimSpace(status), "stopped") {
		networkStatus = nil
	}

	return DaemonStatus{
		Status:         status,
		PID:            info.PID,
		StartedAt:      info.StartedAt,
		Socket:         runtime.Config.Daemon.Socket,
		HTTPHost:       runtime.Config.HTTP.Host,
		HTTPPort:       runtime.Config.HTTP.Port,
		ActiveSessions: 0,
		TotalSessions:  0,
		Version:        version.Current().Version,
		Network:        networkStatus,
	}
}

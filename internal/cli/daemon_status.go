package cli

import (
	"context"
	"errors"

	"os"

	"strings"

	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	aghdaemon "github.com/compozy/agh/internal/daemon"

	"github.com/compozy/agh/internal/version"
)

func waitForDaemonStop(
	ctx context.Context,
	deps commandDeps,
	runtime *runtimeContext,
	info aghdaemon.Info,
) (DaemonStatus, error) {
	waitCtx := ctx
	if waitCtx == nil {
		waitCtx = context.Background()
	}
	if _, hasDeadline := waitCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(waitCtx, deps.stopTimeout)
		defer cancel()
	}

	client, clientErr := clientFromDeps(deps)
	ticker := time.NewTicker(deps.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return DaemonStatus{}, errors.New("cli: daemon did not stop before timeout")
		case <-ticker.C:
			if _, running, err := daemonInfo(runtime.HomePaths, deps); err == nil && !running {
				return daemonStatusWithState(runtime, info, "stopped"), nil
			}
			if clientErr == nil {
				if _, err := client.DaemonStatus(waitCtx); err != nil {
					if _, running, infoErr := daemonInfo(runtime.HomePaths, deps); infoErr == nil && !running {
						return daemonStatusWithState(runtime, info, "stopped"), nil
					}
				}
			}
		}
	}
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

func daemonInfo(homePaths aghconfig.HomePaths, deps commandDeps) (aghdaemon.Info, bool, error) {
	info, err := deps.readDaemonInfo(homePaths.DaemonInfo)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		return aghdaemon.Info{}, false, nil
	default:
		return aghdaemon.Info{}, false, err
	}

	if !deps.processAlive(info.PID) {
		return info, false, nil
	}
	if !deps.processMatchesStartTime(info.PID, info.StartedAt) {
		return info, false, nil
	}
	return info, true, nil
}

func daemonStatusWithState(runtime *runtimeContext, info aghdaemon.Info, status string) DaemonStatus {
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

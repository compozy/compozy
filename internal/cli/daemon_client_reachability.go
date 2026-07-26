package cli

import (
	"context"

	aghdaemon "github.com/compozy/agh/internal/daemon"
)

const daemonRunningStatus = "running"

func daemonClientIfRunning(ctx context.Context, deps commandDeps) (DaemonClient, bool, error) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return nil, false, err
	}

	info, running, err := daemonInfo(runtime.HomePaths, deps)
	if err != nil {
		return nil, false, err
	}
	if !running {
		if info.PID <= 0 || !deps.processAlive(info.PID) {
			return nil, false, nil
		}

		client, clientErr := clientFromDeps(deps)
		if clientErr != nil {
			return nil, false, clientErr
		}
		status, statusErr := client.DaemonStatus(ctx)
		if statusErr != nil {
			if daemonStatusCanFallback(statusErr) {
				return nil, false, nil
			}
			return nil, false, statusErr
		}
		if status.Status != daemonRunningStatus {
			return nil, false, nil
		}
		return client, true, nil
	}
	if info == (aghdaemon.Info{}) {
		return nil, false, nil
	}

	client, err := clientFromDeps(deps)
	if err != nil {
		return nil, false, err
	}
	return client, true, nil
}

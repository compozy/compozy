package daemon

import (
	"context"

	"os"

	"github.com/compozy/agh/internal/api/httpapi"
	"github.com/compozy/agh/internal/api/udsapi"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/procutil"
)

func (d *Daemon) applyServerFactoryDefaults() {
	if d.httpFactory == nil {
		d.httpFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
			return httpapi.New(httpServerOptions(&deps)...)
		}
	}
	if d.udsFactory == nil {
		d.udsFactory = func(_ context.Context, deps RuntimeDeps) (Server, error) {
			return udsapi.New(udsServerOptions(&deps)...)
		}
	}
}

func (d *Daemon) applySystemDefaults() {
	if d.listProcesses == nil {
		d.listProcesses = listProcesses
	}
	if d.signalProcess == nil {
		d.signalProcess = procutil.Signal
	}
	if d.processAlive == nil {
		d.processAlive = procutil.Alive
	}
	if d.executable == nil {
		d.executable = os.Executable
	}
	if d.startDetached == nil {
		d.startDetached = defaultDetachedStart
	}
	if d.getenv == nil {
		d.getenv = os.Getenv
	}
	if d.closeLogger == nil {
		d.closeLogger = func() error { return nil }
	}
	if d.loadConfig == nil {
		d.loadConfig = func() (aghconfig.Config, error) {
			return loadConfigFromHome(d.homePaths)
		}
	}
}

func (d *Daemon) applyTimingDefaults() {
	if d.orphanGraceWait <= 0 {
		d.orphanGraceWait = orphanCleanupGraceWait
	}
	if d.orphanPollWait <= 0 {
		d.orphanPollWait = orphanCleanupPollWait
	}
}

func (d *Daemon) startObserverRetention(ctx context.Context) error {
	d.mu.Lock()
	observer := d.observer
	d.mu.Unlock()

	starter, ok := observer.(observerRetentionStarter)
	if !ok {
		return nil
	}
	return starter.StartRetention(ctx)
}

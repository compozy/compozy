package cli

import (
	"context"

	"os"
	"os/exec"

	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	aghdaemon "github.com/compozy/agh/internal/daemon"

	"github.com/compozy/agh/internal/procutil"
	aghupdate "github.com/compozy/agh/internal/update"
	"github.com/compozy/agh/internal/version"
)

func (d commandDeps) withDefaults() commandDeps {
	d = d.withRegistryDefaults()
	d = d.withRuntimeDefaults()
	d = d.withTimingDefaults()
	return d
}

func (d commandDeps) withRegistryDefaults() commandDeps {
	if d.loadConfig == nil {
		d.loadConfig = func() (aghconfig.Config, error) {
			return aghconfig.Load()
		}
	}
	if d.loadSkillRegistrySources == nil {
		d.loadSkillRegistrySources = defaultSkillRegistrySourceLoader
	}
	if d.resolveHome == nil {
		d.resolveHome = aghconfig.ResolveHomePaths
	}
	if d.resolveHomeForWorkspace == nil {
		d.resolveHomeForWorkspace = aghconfig.ResolveHomePathsForWorkspace
	}
	if d.ensureHome == nil {
		d.ensureHome = aghconfig.EnsureHomeLayout
	}
	return d
}

func (d commandDeps) withRuntimeDefaults() commandDeps {
	if d.runInstallWizard == nil {
		d.runInstallWizard = runInstallWizard
	}
	if d.runBridgeSetupWizard == nil {
		d.runBridgeSetupWizard = runBridgeSetupWizard
	}
	if d.generateBridgeSetupSecret == nil {
		d.generateBridgeSetupSecret = generateBridgeSetupSecret
	}
	if d.newClient == nil {
		d.newClient = NewClient
	}
	if d.runMCPServe == nil {
		d.runMCPServe = func(ctx context.Context, opts mcpServeOptions) error {
			return runMCPServe(ctx, d, opts)
		}
	}
	d = d.withProviderAuthDefaults()
	if d.newDaemon == nil {
		d.newDaemon = func() (daemonRunner, error) {
			return aghdaemon.New()
		}
	}
	if d.runRelaunchHelper == nil {
		d.runRelaunchHelper = aghdaemon.RunRelaunchHelper
	}
	if d.readDaemonInfo == nil {
		d.readDaemonInfo = aghdaemon.ReadInfo
	}
	if d.signalProcess == nil {
		d.signalProcess = procutil.Signal
	}
	if d.processAlive == nil {
		d.processAlive = procutil.Alive
	}
	if d.processMatchesStartTime == nil {
		d.processMatchesStartTime = procutil.MatchesStartTime
	}
	if d.executable == nil {
		d.executable = os.Executable
	}
	if d.getwd == nil {
		d.getwd = os.Getwd
	}
	if d.getenv == nil {
		d.getenv = os.Getenv
	}
	if d.lookPath == nil {
		d.lookPath = exec.LookPath
	}
	if d.inputIsTerminal == nil {
		d.inputIsTerminal = supportBundleInputIsTerminal
	}
	if d.spawnDetached == nil {
		d.spawnDetached = func(ctx context.Context, homePaths aghconfig.HomePaths) (daemonProcess, error) {
			return spawnDetachedDaemonProcess(ctx, homePaths, d.executable)
		}
	}
	if d.newUpdateManager == nil {
		d.newUpdateManager = func(homePaths aghconfig.HomePaths) (updateManager, error) {
			return aghupdate.NewManager(aghupdate.Config{
				HomePaths:      homePaths,
				CurrentVersion: version.Current().Version,
				ExecutablePath: d.executable,
				Getenv:         d.getenv,
			})
		}
	}
	return d
}

func (d commandDeps) withTimingDefaults() commandDeps {
	if d.now == nil {
		d.now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if d.pollInterval <= 0 {
		d.pollInterval = defaultPollInterval
	}
	if d.startTimeout <= 0 {
		d.startTimeout = defaultStartTimeout
	}
	if d.stopTimeout <= 0 {
		d.stopTimeout = defaultStopTimeout
	}
	return d
}

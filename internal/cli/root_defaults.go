package cli

import (
	"context"

	"os"
	"os/exec"

	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"

	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/compozy/compozy/internal/version"
)

func (d commandDeps) withDefaults() commandDeps {
	d = d.withRegistryDefaults()
	d = d.withClientTimeoutDefaults()
	d = d.withRuntimeDefaults()
	d = d.withBootstrapDefaults()
	d = d.withTimingDefaults()
	d = d.withMCPRunnerDefaults()
	return d
}

// withClientTimeoutDefaults wires the daemon client before any closure that
// captures commandDeps, and after the config loader it resolves the budget from.
// A later client default would leave the MCP runner holding a nil factory.
func (d commandDeps) withClientTimeoutDefaults() commandDeps {
	if d.getenv == nil {
		d.getenv = os.Getenv
	}
	if d.newClient == nil {
		d.newClient = func(target ClientTarget) (DaemonClient, error) {
			timeout, err := resolveClientRequestTimeout(d.getenv, d.loadConfig)
			if err != nil {
				return nil, err
			}
			return newClientWithTimeout(target, timeout)
		}
	}
	return d
}

func (d commandDeps) withRegistryDefaults() commandDeps {
	if d.loadConfig == nil {
		d.loadConfig = func() (compozyconfig.Config, error) {
			return compozyconfig.Load(d.windowManagerWarning)
		}
	}
	if d.resolveHome == nil {
		d.resolveHome = compozyconfig.ResolveHomePaths
	}
	if d.resolveHomeForWorkspace == nil {
		d.resolveHomeForWorkspace = compozyconfig.ResolveHomePathsForWorkspace
	}
	if d.ensureHome == nil {
		d.ensureHome = compozyconfig.EnsureHomeLayout
	}
	return d
}

func (d commandDeps) withRuntimeDefaults() commandDeps {
	d = d.withGatewayRuntimeDefaults()
	d = d.withDaemonRuntimeDefaults()
	d = d.withMCPRuntimeDefaults()
	return d
}

func (d commandDeps) withGatewayRuntimeDefaults() commandDeps {
	if d.runInstallWizard == nil {
		d.runInstallWizard = runInstallWizard
	}
	if d.writeGatewayCredential == nil {
		d.writeGatewayCredential = WriteGatewayCredential
	}
	if d.readGatewayCredential == nil {
		d.readGatewayCredential = ReadGatewayCredential
	}
	if d.removeGatewayCredential == nil {
		d.removeGatewayCredential = RemoveGatewayCredential
	}
	if d.setActiveGatewayProfile == nil {
		d.setActiveGatewayProfile = setActiveGatewayProfile
	}
	if d.applyGatewayProfileState == nil {
		d.applyGatewayProfileState = applyGatewayProfileConfigState
	}
	if d.newSSHExecutor == nil {
		d.newSSHExecutor = func() sshExecutor { return systemSSHExecutor{} }
	}
	return d
}

func (d commandDeps) withMCPRuntimeDefaults() commandDeps {
	return d.withProviderAuthDefaults()
}

// withMCPRunnerDefaults installs the MCP runner last: its closure captures the
// whole dependency set, so every other default has to exist first.
func (d commandDeps) withMCPRunnerDefaults() commandDeps {
	if d.runMCPServe == nil {
		d.runMCPServe = func(ctx context.Context, opts mcpServeOptions) error {
			return runMCPServe(ctx, d, opts)
		}
	}
	return d
}

func (d commandDeps) withDaemonRuntimeDefaults() commandDeps {
	if d.newDaemon == nil {
		d.newDaemon = func() (daemonRunner, error) {
			return compozydaemon.New()
		}
	}
	if d.runRelaunchHelper == nil {
		d.runRelaunchHelper = compozydaemon.RunRelaunchHelper
	}
	if d.readDaemonInfo == nil {
		d.readDaemonInfo = compozydaemon.ReadInfo
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
	if d.processOwnsFileLock == nil {
		d.processOwnsFileLock = procutil.OwnsFileLock
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
	if d.removeFile == nil {
		d.removeFile = os.Remove
	}
	if d.openBrowser == nil {
		d.openBrowser = openBrowser
	}
	if d.resolveAppInstallation == nil {
		d.resolveAppInstallation = resolvePlatformAppInstallation
	}
	if d.callAppControl == nil {
		d.callAppControl = callAppControl
	}
	if d.inputIsTerminal == nil {
		d.inputIsTerminal = supportBundleInputIsTerminal
	}
	if d.spawnDetached == nil {
		d.spawnDetached = func(ctx context.Context, homePaths compozyconfig.HomePaths) (daemonProcess, error) {
			return spawnDetachedDaemonProcess(ctx, homePaths, d.executable)
		}
	}
	if d.newUpdateManager == nil {
		d.newUpdateManager = func(homePaths compozyconfig.HomePaths) (updateManager, error) {
			return compozyupdate.NewManager(&compozyupdate.Config{
				HomePaths:      homePaths,
				CurrentVersion: version.Current().Version,
				ExecutablePath: d.executable,
				Getenv:         d.getenv,
			})
		}
	}
	return d
}

func (d commandDeps) withBootstrapDefaults() commandDeps {
	if d.inspectDaemonLock == nil {
		d.inspectDaemonLock = compozydaemon.InspectLockOwner
	}
	if d.spawnBootstrapRuntime == nil {
		d.spawnBootstrapRuntime = func(
			ctx context.Context,
			homePaths compozyconfig.HomePaths,
			binaryPath string,
		) (daemonProcess, error) {
			return spawnDetachedDaemonProcess(ctx, homePaths, func() (string, error) { return binaryPath, nil })
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
	if d.bootstrapAttachTimeout <= 0 {
		d.bootstrapAttachTimeout = bootstrapAttachReadyTimeout
	}
	return d
}

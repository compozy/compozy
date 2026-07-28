package e2elane

import "fmt"

const (
	lanesInternalDaemonPath         = "./internal/daemon"
	lanesInternalSandboxDaytonaPath = "./internal/sandbox/daytona"
	lanesInternalTestutilE2ePath    = "./internal/testutil/e2e"
)

type Lane string

const (
	LaneRuntime  Lane = "runtime"
	LaneWeb      Lane = "web"
	LaneCombined Lane = "combined"
	LaneNightly  Lane = "nightly"
)

const (
	WebDir                   = "web"
	DaemonServedWebScript    = "test:e2e:daemon-served"
	NightlyWebScript         = "test:e2e:nightly"
	RuntimeE2EPattern        = "^TestDaemonE2E"
	NightlyRuntimeE2EPattern = "^TestDaemonNightlyE2E"
	HTTPTransportE2EPattern  = "^TestHTTPTransport"
	UDSTransportE2EPattern   = "^TestUDSTransport"
	HarnessRuntimeE2EPattern = "^TestStartRuntimeHarness"
	DaytonaNightlyE2EPattern = "^TestDaytona(" +
		"ProviderIntegrationFullLifecycle|LauncherTransportValidation|SSHNonPTYValidation)$"
)

type GoSuite struct {
	Packages []string
	Run      string
}

type ScriptSuite struct {
	Dir    string
	Script string
}

type Plan struct {
	Lane                        Lane
	GoSuites                    []GoSuite
	ScriptSuites                []ScriptSuite
	RequiresDaemonServedBrowser bool
	IncludesCredentialedNightly bool
}

func cloneGoSuites(in []GoSuite) []GoSuite {
	if len(in) == 0 {
		return nil
	}

	out := make([]GoSuite, len(in))
	for i, suite := range in {
		out[i] = GoSuite{
			Packages: append([]string(nil), suite.Packages...),
			Run:      suite.Run,
		}
	}
	return out
}

func PlanForLane(lane Lane) (Plan, error) {
	switch lane {
	case LaneRuntime:
		return Plan{
			Lane:     lane,
			GoSuites: cloneGoSuites(runtimeGoSuites),
		}, nil
	case LaneWeb:
		return Plan{
			Lane:                        lane,
			ScriptSuites:                append([]ScriptSuite(nil), daemonServedWebSuites...),
			RequiresDaemonServedBrowser: true,
		}, nil
	case LaneCombined:
		return Plan{
			Lane:                        lane,
			GoSuites:                    cloneGoSuites(runtimeGoSuites),
			ScriptSuites:                append([]ScriptSuite(nil), daemonServedWebSuites...),
			RequiresDaemonServedBrowser: true,
		}, nil
	case LaneNightly:
		return Plan{
			Lane:     lane,
			GoSuites: append(cloneGoSuites(runtimeGoSuites), cloneGoSuites(nightlyGoSuites)...),
			ScriptSuites: append(
				append([]ScriptSuite(nil), daemonServedWebSuites...),
				nightlyWebSuites...),
			RequiresDaemonServedBrowser: true,
			IncludesCredentialedNightly: true,
		}, nil
	default:
		return Plan{}, fmt.Errorf("unknown e2e lane %q", lane)
	}
}

var runtimeGoSuites = []GoSuite{
	{
		Packages: []string{lanesInternalDaemonPath},
		Run:      RuntimeE2EPattern,
	},
	{
		Packages: []string{"./internal/api/httpapi"},
		Run:      HTTPTransportE2EPattern,
	},
	{
		Packages: []string{"./internal/api/udsapi"},
		Run:      UDSTransportE2EPattern,
	},
	{
		Packages: []string{lanesInternalTestutilE2ePath},
		Run:      HarnessRuntimeE2EPattern,
	},
}

var nightlyGoSuites = []GoSuite{
	{
		Packages: []string{lanesInternalDaemonPath},
		Run:      NightlyRuntimeE2EPattern,
	},
	{
		Packages: []string{lanesInternalSandboxDaytonaPath},
		Run:      DaytonaNightlyE2EPattern,
	},
}

var daemonServedWebSuites = []ScriptSuite{
	{
		Dir:    WebDir,
		Script: DaemonServedWebScript,
	},
}

var nightlyWebSuites = []ScriptSuite{
	{
		Dir:    WebDir,
		Script: NightlyWebScript,
	},
}

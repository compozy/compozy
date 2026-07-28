package e2elane

import (
	"reflect"
	"regexp"
	"testing"
)

func TestPlanForLaneMapsRuntimeWebCombinedAndNightlySlices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		lane        Lane
		wantGo      []GoSuite
		wantScripts []ScriptSuite
		wantDaemon  bool
		wantNightly bool
	}{
		{
			name:        "Should keep daemon and transport parity in the runtime lane",
			lane:        LaneRuntime,
			wantGo:      expectedRuntimeGoSuites(),
			wantScripts: nil,
		},
		{
			name:        "Should run daemon-served Playwright only in the web lane",
			lane:        LaneWeb,
			wantGo:      nil,
			wantScripts: expectedDaemonServedWebSuites(),
			wantDaemon:  true,
		},
		{
			name:        "Should join runtime and browser suites in the combined lane",
			lane:        LaneCombined,
			wantGo:      expectedRuntimeGoSuites(),
			wantScripts: expectedDaemonServedWebSuites(),
			wantDaemon:  true,
		},
		{
			name:        "Should add credentialed runtime and browser suites in the nightly lane",
			lane:        LaneNightly,
			wantGo:      append(expectedRuntimeGoSuites(), expectedNightlyGoSuites()...),
			wantScripts: append(expectedDaemonServedWebSuites(), expectedNightlyWebSuites()...),
			wantDaemon:  true,
			wantNightly: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			plan, err := PlanForLane(tt.lane)
			if err != nil {
				t.Fatalf("PlanForLane(%q) error = %v", tt.lane, err)
			}

			if plan.Lane != tt.lane {
				t.Fatalf("plan.Lane = %q, want %q", plan.Lane, tt.lane)
			}
			if !reflect.DeepEqual(plan.GoSuites, tt.wantGo) {
				t.Fatalf("plan.GoSuites = %#v, want %#v", plan.GoSuites, tt.wantGo)
			}
			if !reflect.DeepEqual(plan.ScriptSuites, tt.wantScripts) {
				t.Fatalf("plan.ScriptSuites = %#v, want %#v", plan.ScriptSuites, tt.wantScripts)
			}
			if plan.RequiresDaemonServedBrowser != tt.wantDaemon {
				t.Fatalf(
					"plan.RequiresDaemonServedBrowser = %v, want %v",
					plan.RequiresDaemonServedBrowser,
					tt.wantDaemon,
				)
			}
			if plan.IncludesCredentialedNightly != tt.wantNightly {
				t.Fatalf(
					"plan.IncludesCredentialedNightly = %v, want %v",
					plan.IncludesCredentialedNightly,
					tt.wantNightly,
				)
			}
		})
	}
}

func TestPlanForLaneKeepsCredentialedNightlyOutOfPRRequiredEntryPoints(t *testing.T) {
	t.Parallel()

	for _, lane := range []Lane{LaneRuntime, LaneWeb, LaneCombined} {
		t.Run("Should keep credentialed nightly suites out of "+string(lane), func(t *testing.T) {
			t.Parallel()

			plan, err := PlanForLane(lane)
			if err != nil {
				t.Fatalf("PlanForLane(%q) error = %v", lane, err)
			}

			if plan.IncludesCredentialedNightly {
				t.Fatalf("plan.IncludesCredentialedNightly = true, want false")
			}

			for _, suite := range plan.GoSuites {
				for _, pkg := range suite.Packages {
					if pkg == "./internal/sandbox/daytona" {
						t.Fatalf("plan.GoSuites unexpectedly included nightly package %q", pkg)
					}
				}
				if suite.Run == NightlyRuntimeE2EPattern {
					t.Fatalf("plan.GoSuites unexpectedly included nightly daemon pattern %q", suite.Run)
				}
			}
		})
	}
}

func TestLaneRunPatternsCompileAndMatchRepresentativeTests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
		matches []string
		rejects []string
	}{
		{
			name:    "Should match default daemon e2e tests only with the runtime daemon pattern",
			pattern: RuntimeE2EPattern,
			matches: []string{"TestDaemonE2EAutomationTaskBackedJobDelegatesTaskRun"},
			rejects: []string{"TestDaemonNightlyE2EAutomationTaskResumesIntoNetworkChannel"},
		},
		{
			name:    "Should match nightly daemon tests only with the nightly daemon pattern",
			pattern: NightlyRuntimeE2EPattern,
			matches: []string{"TestDaemonNightlyE2EAutomationTaskResumesIntoNetworkChannel"},
			rejects: []string{"TestDaemonE2EAutomationTaskBackedJobDelegatesTaskRun"},
		},
		{
			name:    "Should match HTTP transport e2e tests with the HTTP transport pattern",
			pattern: HTTPTransportE2EPattern,
			matches: []string{"TestHTTPTransportPromptStream"},
			rejects: []string{"TestUDSTransportPromptStream"},
		},
		{
			name:    "Should match UDS transport e2e tests with the UDS transport pattern",
			pattern: UDSTransportE2EPattern,
			matches: []string{"TestUDSTransportPromptStream"},
			rejects: []string{"TestHTTPTransportPromptStream"},
		},
		{
			name:    "Should match runtime harness tests with the harness pattern",
			pattern: HarnessRuntimeE2EPattern,
			matches: []string{"TestStartRuntimeHarnessWithDaemonBinary"},
			rejects: []string{"TestDaemonE2EAutomationTaskBackedJobDelegatesTaskRun"},
		},
		{
			name:    "Should match credentialed Daytona nightly tests with the Daytona pattern",
			pattern: DaytonaNightlyE2EPattern,
			matches: []string{
				"TestDaytonaProviderIntegrationFullLifecycle",
				"TestDaytonaLauncherTransportValidation",
				"TestDaytonaSSHNonPTYValidation",
			},
			rejects: []string{"TestDaytonaUnlistedScenario"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			compiled, err := regexp.Compile(tt.pattern)
			if err != nil {
				t.Fatalf("regexp.Compile(%q) error = %v", tt.pattern, err)
			}
			for _, name := range tt.matches {
				if !compiled.MatchString(name) {
					t.Fatalf("pattern %q did not match representative test %q", tt.pattern, name)
				}
			}
			for _, name := range tt.rejects {
				if compiled.MatchString(name) {
					t.Fatalf("pattern %q matched excluded test %q", tt.pattern, name)
				}
			}
		})
	}
}

func TestPlanForLaneRejectsUnknownLane(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unknown lanes", func(t *testing.T) {
		t.Parallel()

		if _, err := PlanForLane("mystery"); err == nil {
			t.Fatal("PlanForLane() error = nil, want non-nil")
		}
	})
}

func expectedRuntimeGoSuites() []GoSuite {
	return []GoSuite{
		{Packages: []string{"./internal/daemon"}, Run: "^TestDaemonE2E"},
		{Packages: []string{"./internal/api/httpapi"}, Run: "^TestHTTPTransport"},
		{Packages: []string{"./internal/api/udsapi"}, Run: "^TestUDSTransport"},
		{Packages: []string{"./internal/testutil/e2e"}, Run: "^TestStartRuntimeHarness"},
	}
}

func expectedNightlyGoSuites() []GoSuite {
	return []GoSuite{
		{Packages: []string{"./internal/daemon"}, Run: "^TestDaemonNightlyE2E"},
		{
			Packages: []string{"./internal/sandbox/daytona"},
			Run:      "^TestDaytona(ProviderIntegrationFullLifecycle|LauncherTransportValidation|SSHNonPTYValidation)$",
		},
	}
}

func expectedDaemonServedWebSuites() []ScriptSuite {
	return []ScriptSuite{{Dir: "web", Script: "test:e2e:daemon-served"}}
}

func expectedNightlyWebSuites() []ScriptSuite {
	return []ScriptSuite{{Dir: "web", Script: "test:e2e:nightly"}}
}

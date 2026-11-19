package daemon

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/user"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/session"
)

func TestBuildHostedMCPServiceLogsDisabledConfiguration(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		toolsEnabled     bool
		hostedMCPEnabled bool
		wantReason       string
	}{
		"tools disabled": {
			toolsEnabled:     false,
			hostedMCPEnabled: true,
			wantReason:       "tools_disabled",
		},
		"hosted MCP disabled": {
			toolsEnabled:     true,
			hostedMCPEnabled: false,
			wantReason:       "hosted_mcp_disabled",
		},
	}

	for name, testCase := range testCases {
		t.Run("Should log disabled hosted MCP gate for "+name, func(t *testing.T) {
			t.Parallel()

			cfg := compozyconfig.Config{Tools: compozyconfig.DefaultToolsConfig()}
			cfg.Tools.Enabled = testCase.toolsEnabled
			cfg.Tools.HostedMCPEnabled = testCase.hostedMCPEnabled
			var logOutput bytes.Buffer
			service, err := (&Daemon{}).buildHostedMCPService(&bootState{
				cfg:    cfg,
				logger: slog.New(slog.NewJSONHandler(&logOutput, nil)),
			})
			if err != nil {
				t.Fatalf("buildHostedMCPService() error = %v", err)
			}
			if service != nil {
				t.Fatalf("buildHostedMCPService() = %#v, want nil service", service)
			}

			got := logOutput.String()
			for _, want := range []string{
				`"msg":"daemon.hosted_mcp.disabled"`,
				`"reason":"` + testCase.wantReason + `"`,
				`"tools_enabled":` + boolString(testCase.toolsEnabled),
				`"hosted_mcp_enabled":` + boolString(testCase.hostedMCPEnabled),
			} {
				if !strings.Contains(got, want) {
					t.Fatalf("log output = %q, want substring %q", got, want)
				}
			}
		})
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// Invariant: the daemon adapter identifies hosted readiness failures without reclassifying cancellation.
// Owner: daemon hosted MCP launcher adapter; canonical suite: hosted_mcp_test.go.
func TestHostedMCPLauncherReadiness(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		cause error
	}{
		{"Should identify an expired bind as a readiness failure", mcppkg.ErrHostedBindTimeout},
		{"Should identify a failed projection as a readiness failure", mcppkg.ErrHostedProjectionFailed},
		{"Should preserve cancellation without classifying it as a readiness failure", context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
			service, err := mcppkg.NewHostedService(&mcppkg.HostedConfig{
				Enabled:        true,
				ExpectedBinary: executable,
				BindNonceTTL:   time.Minute,
				Now:            func() time.Time { return now },
			})
			if err != nil {
				t.Fatal(err)
			}
			launcher := hostedMCPLauncherAdapter{service: service}
			const id = "session-hosted-readiness"
			launch, err := launcher.Launch(t.Context(), session.HostedMCPLaunchRequest{SessionID: id, EagerTools: true})
			if err != nil {
				t.Fatal(err)
			}
			if err := launcher.ArmLaunch(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch tc.cause {
			case mcppkg.ErrHostedBindTimeout:
				now = now.Add(time.Minute)
			case mcppkg.ErrHostedProjectionFailed:
				index := slices.Index(launch.Args, "--bind-nonce")
				if index < 0 || index+1 >= len(launch.Args) {
					t.Fatal("hosted launch did not include a bind nonce")
				}
				currentUser, err := user.Current()
				if err != nil {
					t.Fatal(err)
				}
				peer := mcppkg.PeerInfo{
					PID: os.Getpid(), UID: os.Getuid(), GID: os.Getgid(), UserSID: currentUser.Uid,
					ExecutablePath: executable, Supported: true,
				}
				if _, err := service.Bind(
					ctx,
					mcppkg.HostedBindRequest{SessionID: id, Nonce: launch.Args[index+1]},
					peer,
				); !errors.Is(
					err,
					mcppkg.ErrHostedRegistryRequired,
				) {
					t.Fatalf("Bind() = %v, want missing projection registry", err)
				}
			case context.Canceled:
				cancel()
			}
			_, digestErr := launcher.WaitForBindToolDigest(ctx, id)
			for _, err := range []error{launcher.WaitForBind(ctx, id), digestErr} {
				_, readiness := errors.AsType[*session.HostedMCPReadinessError](err)
				if !errors.Is(err, tc.cause) || readiness != (tc.cause != context.Canceled) {
					t.Fatalf("readiness = %v (classified %t), want %v", err, readiness, tc.cause)
				}
			}
		})
	}
}

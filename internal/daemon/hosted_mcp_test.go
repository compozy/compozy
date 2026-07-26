package daemon

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
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

			cfg := aghconfig.Config{Tools: aghconfig.DefaultToolsConfig()}
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

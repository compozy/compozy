// Suite: extension CLI output contracts
// Invariant: extension bundles preserve machine parity while human output explains discovery, health, and next steps.
// Boundary IN: extension output bundle composition.
// Boundary OUT: daemon payload computation, owned by API and extension runtime suites.
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/spf13/cobra"
)

func TestExtensionDurationFormatting(t *testing.T) {
	t.Parallel()

	t.Run("Should format uptime without overflowing time.Duration", func(t *testing.T) {
		t.Parallel()

		if got, want := formatExtensionUptime(math.MaxInt64), "2562047788015215h 30m"; got != want {
			t.Fatalf("formatExtensionUptime(MaxInt64) = %q, want %q", got, want)
		}
	})

	t.Run("Should normalize non-positive duration values", func(t *testing.T) {
		t.Parallel()

		if got := formatExtensionUptime(-1); got != "" {
			t.Fatalf("formatExtensionUptime(-1) = %q, want empty", got)
		}
		if got, want := formatExtensionBackoff(-1), "0s"; got != want {
			t.Fatalf("formatExtensionBackoff(-1) = %q, want %q", got, want)
		}
	})

	t.Run("Should preserve the exact backoff duration ceiling", func(t *testing.T) {
		t.Parallel()

		maximum := int64(math.MaxInt64) / int64(time.Millisecond)
		wantMaximum := (time.Duration(maximum) * time.Millisecond).String()
		if got := formatExtensionBackoff(maximum); got != wantMaximum {
			t.Fatalf("formatExtensionBackoff(maximum) = %q, want %q", got, wantMaximum)
		}
		wantOverflow := fmt.Sprintf("%dms", maximum+1)
		if got := formatExtensionBackoff(maximum + 1); got != wantOverflow {
			t.Fatalf("formatExtensionBackoff(maximum+1) = %q, want %q", got, wantOverflow)
		}
	})

	t.Run("Should keep oversized backoff truthful", func(t *testing.T) {
		t.Parallel()

		if got, want := formatExtensionBackoff(math.MaxInt64), "9223372036854775807ms"; got != want {
			t.Fatalf("formatExtensionBackoff(MaxInt64) = %q, want %q", got, want)
		}
	})
}

func TestExtensionSingleRecordBundlesEmitJSONL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		bundle outputBundle
	}{
		{name: "Should emit status as one JSON object", bundle: extensionBundle(&ExtensionRecord{Name: "alpha"})},
		{
			name:   "Should emit remove as one JSON object",
			bundle: extensionRemoveBundle(extensionRemoveItem{Name: "alpha", Status: "removed"}),
		},
		{
			name: "Should emit provenance as one JSON object",
			bundle: extensionProvenanceBundle(ExtensionProvenanceRecord{
				Slug: "acme/alpha", InstalledFrom: "github",
			}),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			output, err := renderExtensionOutput(t, test.bundle, OutputJSONL)
			if err != nil {
				t.Fatalf("writeCommandOutput(jsonl) error = %v", err)
			}
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) != 1 {
				t.Fatalf("jsonl lines = %d, want 1; output=%q", len(lines), output)
			}
			var decoded map[string]any
			if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(jsonl) error = %v; output=%q", err, output)
			}
		})
	}
}

func TestExtensionLogsOutputCarriesTheStreamEpoch(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve the paired cursor in every structured format", func(t *testing.T) {
		t.Parallel()

		epoch := "epoch-alpha"
		snapshot := ExtensionLogsRecord{
			StreamEpoch: epoch,
			Logs: []ExtensionLogRecord{{
				Sequence:    7,
				StreamEpoch: epoch,
				Timestamp:   time.Date(2026, time.July, 20, 10, 0, 0, 0, time.UTC),
				Message:     "ready",
			}},
		}
		bundle := extensionLogsBundle(snapshot)

		jsonOutput, err := renderExtensionOutput(t, bundle, OutputJSON)
		if err != nil {
			t.Fatalf("writeCommandOutput(json) error = %v", err)
		}
		var decodedSnapshot ExtensionLogsRecord
		if err := json.Unmarshal([]byte(jsonOutput), &decodedSnapshot); err != nil {
			t.Fatalf("json.Unmarshal(extension logs snapshot) error = %v", err)
		}
		if !reflect.DeepEqual(decodedSnapshot, snapshot) {
			t.Fatalf("extension logs JSON = %#v, want %#v", decodedSnapshot, snapshot)
		}

		jsonlOutput, err := renderExtensionOutput(t, bundle, OutputJSONL)
		if err != nil {
			t.Fatalf("writeCommandOutput(jsonl) error = %v", err)
		}
		var decodedEntry ExtensionLogRecord
		if err := json.Unmarshal([]byte(strings.TrimSpace(jsonlOutput)), &decodedEntry); err != nil {
			t.Fatalf("json.Unmarshal(extension logs JSONL) error = %v", err)
		}
		if decodedEntry.StreamEpoch != epoch || decodedEntry.Sequence != 7 {
			t.Fatalf("extension logs JSONL = %#v, want epoch %q and sequence 7", decodedEntry, epoch)
		}

		toonOutput, err := renderExtensionOutput(t, bundle, OutputToon)
		if err != nil {
			t.Fatalf("writeCommandOutput(toon) error = %v", err)
		}
		for _, want := range []string{"stream_epoch", epoch, "7", "ready"} {
			if !strings.Contains(toonOutput, want) {
				t.Fatalf("extension logs TOON = %q, want %q", toonOutput, want)
			}
		}
	})
}

func TestExtensionLogResetOutputPreservesAnEmptyReplacement(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve an identified empty replacement in every structured format", func(t *testing.T) {
		t.Parallel()

		bundle := extensionLogResetBundle(ExtensionLogsRecord{
			Logs: []ExtensionLogRecord{}, StreamEpoch: "epoch-empty",
		})
		for _, format := range []OutputFormat{OutputJSON, OutputJSONL} {
			output, err := renderExtensionOutput(t, bundle, format)
			if err != nil {
				t.Fatalf("writeCommandOutput(%s) error = %v", format, err)
			}
			var decoded extensionLogResetRecord
			if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(%s reset) error = %v", format, err)
			}
			if decoded.Event != "extension_log_reset" ||
				decoded.StreamEpoch != "epoch-empty" ||
				len(decoded.Logs) != 0 {
				t.Fatalf("extension log reset %s = %#v, want identified empty reset", format, decoded)
			}
		}

		toonOutput, err := renderExtensionOutput(t, bundle, OutputToon)
		if err != nil {
			t.Fatalf("writeCommandOutput(toon reset) error = %v", err)
		}
		for _, want := range []string{"extension_log_reset", "epoch-empty", "log_count", "(empty)"} {
			if !strings.Contains(toonOutput, want) {
				t.Fatalf("extension log reset TOON = %q, want %q", toonOutput, want)
			}
		}
	})
}

func TestExtensionListShowsPassiveUpdate(t *testing.T) {
	t.Parallel()

	t.Run("Should show the remote version without an update flag", func(t *testing.T) {
		t.Parallel()

		bundle := extensionListBundle([]ExtensionRecord{{
			Name: "alpha", Version: "0.1.0", UpdateAvailable: true, RemoteVersion: "0.2.0",
		}})
		output, err := bundle.human()
		if err != nil {
			t.Fatalf("extensionListBundle().human() error = %v", err)
		}
		for _, want := range []string{"Update", "→ 0.2.0"} {
			if !strings.Contains(output, want) {
				t.Fatalf("extension list output = %q, want %q", output, want)
			}
		}
	})
}

func TestExtensionSearchShowsPassiveUpdate(t *testing.T) {
	t.Parallel()

	t.Run("Should show the remote version without an update flag", func(t *testing.T) {
		t.Parallel()

		bundle := extensionSearchBundle(ExtensionSearchRecord{Items: []extensionSearchItem{{
			Slug: "acme/alpha", Name: "alpha", Version: "0.2.0", UpdateAvailable: true,
		}}})
		output, err := bundle.human()
		if err != nil {
			t.Fatalf("extensionSearchBundle().human() error = %v", err)
		}
		for _, want := range []string{"Update", "→ 0.2.0"} {
			if !strings.Contains(output, want) {
				t.Fatalf("extension search output = %q, want %q", output, want)
			}
		}
	})
}

func TestExtensionStatusShowsFailureSummary(t *testing.T) {
	t.Parallel()

	t.Run("Should explain failure count and restart backoff", func(t *testing.T) {
		t.Parallel()

		bundle := extensionBundle(&ExtensionRecord{
			Name:                "alpha",
			Enabled:             true,
			State:               "errored",
			Health:              "unhealthy",
			ConsecutiveFailures: 4,
			RestartBackoffMS:    8000,
		})
		output, err := bundle.human()
		if err != nil {
			t.Fatalf("extensionBundle().human() error = %v", err)
		}
		for _, want := range []string{
			"Consecutive Failures:",
			"4",
			"Restart Backoff:",
			"8s",
			"Summary:",
			"crash-looping (4 failures, backoff 8s)",
		} {
			if !strings.Contains(output, want) {
				t.Fatalf("extension status output = %q, want %q", output, want)
			}
		}
	})
}

func TestExtensionAgentPluginOutputContract(t *testing.T) {
	t.Parallel()

	report := &extensionpkg.ValidationReport{
		Status: "valid", Format: string(extensionpkg.FormatAgentPlugin), Name: "acme.tools", Version: "1.2.0",
		WouldIngest: []contract.ExtensionValidationComponent{
			{Kind: "skill", Name: "deploy", Detail: "skills/deploy"},
			{Kind: "mcp_server", Name: "deployment-api", Transport: "streamable-http", Detail: "streamable-http"},
		},
		Issues: []contract.ValidationIssue{{
			Scope: "mcp:legacy-events", Message: "sse transport is not supported",
			Severity: contract.IssueSeverityWarn,
		}},
	}
	diagnostic := contract.DiagnosticItem{
		Code:     diagnosticcontract.CodeExtensionAgentPluginComponentSkipped,
		Severity: contract.SeverityWarn,
		Message:  "mcp server \"legacy-events\": sse transport is not supported",
		Evidence: map[string]any{"scope": "mcp:legacy-events", "component": "mcp_server"},
	}
	item := ExtensionRecord{
		Name: "acme.tools", Version: "1.2.0", Type: "resource", Format: string(extensionpkg.FormatAgentPlugin),
		Source: "user", Diagnostics: []contract.DiagnosticItem{diagnostic},
	}

	t.Run("Should render install format ingested skipped and portable next step", func(t *testing.T) {
		t.Parallel()

		human, err := extensionInstallSuccessBundle(&item, report).human()
		if err != nil {
			t.Fatalf("extensionInstallSuccessBundle().human() error = %v", err)
		}
		for _, want := range []string{
			"✓ install acme.tools", "Format: agent plugin", "Ingested", "skill", "deploy", "skills/deploy",
			"Skipped", "legacy-events", "sse transport is not supported",
			"next: compozy extension enable acme.tools", "Format:", "agent plugin",
		} {
			if !strings.Contains(human, want) {
				t.Fatalf("portable install output = %q, want %q", human, want)
			}
		}
		if strings.Index(human, "Ingested") > strings.Index(human, "next:") {
			t.Fatalf("portable install output orders report after next step: %q", human)
		}
	})

	t.Run("Should preserve native install human shape", func(t *testing.T) {
		t.Parallel()

		native := ExtensionRecord{Name: "native", Version: "1.0.0", Type: "resource", Format: "compozy"}
		human, err := extensionInstallSuccessBundle(&native, nil).human()
		if err != nil {
			t.Fatalf("extensionInstallSuccessBundle(native).human() error = %v", err)
		}
		want := renderHumanBlocks(
			"✓ install native",
			"next: compozy extension status native",
			extensionHumanDetail(&native, false),
		)
		if human != want || strings.Contains(human, "Format:") || strings.Contains(human, "Ingested") {
			t.Fatalf("native install output = %q, want legacy shape %q", human, want)
		}
	})

	t.Run("Should render dual manifest note exactly once", func(t *testing.T) {
		t.Parallel()

		dual := &extensionpkg.ValidationReport{DualManifest: true, Format: string(extensionpkg.FormatCompozy)}
		human, err := extensionInstallSuccessBundle(
			&ExtensionRecord{Name: "dual-target", Format: string(extensionpkg.FormatCompozy)}, dual,
		).human()
		if err != nil {
			t.Fatalf("extensionInstallSuccessBundle(dual).human() error = %v", err)
		}
		note := "note: directory carries both extension.toml and plugin.json; installed as a CompozyOS extension " +
			"(native manifest wins)"
		if strings.Count(human, note) != 1 {
			t.Fatalf("dual install output = %q, want one %q", human, note)
		}
	})

	t.Run("Should render install and validate in all four output modes", func(t *testing.T) {
		t.Parallel()

		bundles := []struct {
			name   string
			bundle outputBundle
		}{
			{name: "install", bundle: extensionInstallSuccessBundle(&item, report)},
			{name: "validate", bundle: extensionValidationBundle(report)},
		}
		for _, test := range bundles {
			t.Run("Should render "+test.name+" semantically", func(t *testing.T) {
				t.Parallel()

				for _, format := range []OutputFormat{OutputHuman, OutputJSON, OutputJSONL, OutputToon} {
					t.Run("Should render "+string(format)+" output", func(t *testing.T) {
						t.Parallel()

						output, err := renderExtensionOutput(t, test.bundle, format)
						if err != nil {
							t.Fatalf("writeCommandOutput(%s) error = %v", format, err)
						}
						if strings.TrimSpace(output) == "" {
							t.Fatalf("writeCommandOutput(%s) = empty", format)
						}
					})
				}
			})
		}
	})

	t.Run("Should render warn and fatal validation verdicts", func(t *testing.T) {
		t.Parallel()

		validHuman, err := extensionValidationBundle(report).human()
		if err != nil {
			t.Fatalf("extensionValidationBundle(valid).human() error = %v", err)
		}
		for _, want := range []string{"Status:", "valid", "Format:", "agent plugin", "WARN mcp:legacy-events"} {
			if !strings.Contains(validHuman, want) {
				t.Fatalf("portable validation output = %q, want %q", validHuman, want)
			}
		}
		if validationHasErrors(report.Issues) {
			t.Fatal("validationHasErrors(warn-only) = true, want false")
		}
		fatal := &extensionpkg.ValidationReport{
			Status: "invalid", Format: string(extensionpkg.FormatAgentPlugin),
			Issues: []contract.ValidationIssue{{
				Path: "plugin.json", Scope: "plugin.json", Message: "name is invalid",
				Severity: contract.IssueSeverityError,
			}},
		}
		fatalHuman, err := extensionValidationBundle(fatal).human()
		if err != nil {
			t.Fatalf("extensionValidationBundle(fatal).human() error = %v", err)
		}
		if !strings.Contains(fatalHuman, "Status:") || !strings.Contains(fatalHuman, "invalid") ||
			!strings.Contains(fatalHuman, "ERROR plugin.json") || !validationHasErrors(fatal.Issues) {
			t.Fatalf("fatal portable validation output = %q, want invalid ERROR verdict", fatalHuman)
		}
	})

	t.Run("Should summarize recorded degradation", func(t *testing.T) {
		t.Parallel()

		degraded := item
		degraded.Enabled = true
		degraded.DaemonRunning = true
		degraded.Health = "healthy"
		if got, want := extensionRuntimeSummary(&degraded), "running (1 component skipped)"; got != want {
			t.Fatalf("extensionRuntimeSummary() = %q, want %q", got, want)
		}
	})

	t.Run("Should render ingestion reports for every portable batch update", func(t *testing.T) {
		t.Parallel()

		secondReport := *report
		secondReport.Name = "beta.tools"
		items := []extensionUpdateItem{
			{Name: "acme.tools", Status: "updated", Path: "/extensions/acme.tools"},
			{Name: "beta.tools", Status: "updated", Path: "/extensions/beta.tools"},
		}
		portable := []extensionPortableUpdateOutput{
			{Update: items[0], Report: report, DataPath: "/data/acme.tools"},
			{Update: items[1], Report: &secondReport, DataPath: "/data/beta.tools"},
		}
		human, err := extensionUpdatesSuccessBundle(items, portable).human()
		if err != nil {
			t.Fatalf("extensionUpdatesSuccessBundle().human() error = %v", err)
		}
		for _, want := range []string{
			"✓ update acme.tools", "✓ update beta.tools", "/data/acme.tools", "/data/beta.tools",
			"next: compozy extension status acme.tools", "next: compozy extension status beta.tools",
		} {
			if !strings.Contains(human, want) {
				t.Fatalf("portable batch update output = %q, want %q", human, want)
			}
		}
		if strings.Count(human, "Format: agent plugin") != 2 {
			t.Fatalf("portable batch update output = %q, want two ingestion reports", human)
		}
	})
}

func TestExtensionRemoveOutputReportsDataCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should report the install and data paths removed", func(t *testing.T) {
		t.Parallel()

		human, err := extensionRemoveBundle(extensionRemoveItem{
			Name: "acme.tools", Path: "/home/alex/.compozy/extensions/acme.tools",
			DataPath: "/home/alex/.compozy/extension-data/acme.tools", Status: "removed",
		}).human()
		if err != nil {
			t.Fatalf("extensionRemoveBundle().human() error = %v", err)
		}
		if strings.Count(human, "removed:") != 2 {
			t.Fatalf("remove output = %q, want two removed lines", human)
		}
		toon, err := extensionRemoveBundle(extensionRemoveItem{
			Name: "acme.tools", Path: "/home/alex/.compozy/extensions/acme.tools",
			DataPath: "/home/alex/.compozy/extension-data/acme.tools", Status: "removed",
		}).toon()
		if err != nil {
			t.Fatalf("extensionRemoveBundle().toon() error = %v", err)
		}
		if !strings.Contains(toon, "data_path") ||
			!strings.Contains(toon, "/home/alex/.compozy/extension-data/acme.tools") {
			t.Fatalf("remove TOON = %q, want data cleanup path", toon)
		}
	})

	t.Run("Should report quarantined data as a successful warning", func(t *testing.T) {
		t.Parallel()

		quarantine := "/home/alex/.compozy/extension-data/acme.tools.removed-1755280000"
		human, err := extensionRemoveBundle(extensionRemoveItem{
			Name: "acme.tools", Path: "/home/alex/.compozy/extensions/acme.tools",
			DataPath: "/home/alex/.compozy/extension-data/acme.tools", QuarantinePath: quarantine, Status: "removed",
		}).human()
		if err != nil {
			t.Fatalf("extensionRemoveBundle(quarantined).human() error = %v", err)
		}
		for _, want := range []string{"✓ remove acme.tools", "warning: data directory could not be removed", quarantine,
			"a fresh install of acme.tools starts empty"} {
			if !strings.Contains(human, want) {
				t.Fatalf("quarantined remove output = %q, want %q", human, want)
			}
		}
	})
}

func TestExtensionSuccessBundlesNameNextStep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		verb string
		next string
	}{
		{verb: "dev", next: "compozy extension logs alpha --follow"},
		{verb: "install", next: "compozy extension status alpha"},
		{verb: "reload", next: "compozy extension logs alpha --follow"},
	}
	for _, test := range tests {
		t.Run("Should render "+test.verb+" confirmation", func(t *testing.T) {
			t.Parallel()

			bundle := extensionSuccessBundle(test.verb, &ExtensionRecord{Name: "alpha"})
			output, err := bundle.human()
			if err != nil {
				t.Fatalf("extensionSuccessBundle(%s).human() error = %v", test.verb, err)
			}
			for _, want := range []string{"✓ " + test.verb + " alpha", "next: " + test.next} {
				if !strings.Contains(output, want) {
					t.Fatalf("extension success output = %q, want %q", output, want)
				}
			}

			jsonOutput, err := renderExtensionOutput(t, bundle, OutputJSON)
			if err != nil {
				t.Fatalf("writeCommandOutput(json) error = %v", err)
			}
			var decoded ExtensionRecord
			if err := json.Unmarshal([]byte(jsonOutput), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(success) error = %v", err)
			}
			if decoded.Name != "alpha" {
				t.Fatalf("structured success name = %q, want alpha", decoded.Name)
			}
		})
	}
}

func TestExtensionEnableOutputUsesTheActionResult(t *testing.T) {
	t.Parallel()

	t.Run("Should enumerate started automation and preserve JSON exactly", func(t *testing.T) {
		t.Parallel()

		result := ExtensionEnableRecord{
			Extension: ExtensionRecord{
				Name:       "alpha",
				Enabled:    true,
				State:      "active",
				MissingEnv: []string{"API_KEY"},
			},
			AutomationStarted: []string{"alpha/daily", "alpha/on-task"},
		}
		bundle := extensionEnableBundle(&result)
		human, err := bundle.human()
		if err != nil {
			t.Fatalf("extensionEnableBundle().human() error = %v", err)
		}
		for _, want := range []string{
			"✓ Enabled alpha",
			"Automation started",
			"alpha/daily, alpha/on-task",
			"compozy extension secrets set alpha --env API_KEY",
		} {
			if !strings.Contains(human, want) {
				t.Fatalf("enable human output = %q, want %q", human, want)
			}
		}
		encoded, err := renderExtensionOutput(t, bundle, OutputJSON)
		if err != nil {
			t.Fatalf("writeCommandOutput(json) error = %v", err)
		}
		var decoded ExtensionEnableRecord
		if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(enable) error = %v", err)
		}
		if !reflect.DeepEqual(decoded, result) {
			t.Fatalf("enable JSON = %#v, want %#v", decoded, result)
		}
	})

	t.Run("Should omit the automation section for an empty kit", func(t *testing.T) {
		t.Parallel()

		human, err := extensionEnableBundle(&ExtensionEnableRecord{
			Extension: ExtensionRecord{Name: "empty", Enabled: true},
		}).human()
		if err != nil {
			t.Fatalf("extensionEnableBundle().human() error = %v", err)
		}
		if strings.Contains(human, "Automation started") {
			t.Fatalf("empty enable output = %q, want no automation section", human)
		}
	})
}

func TestExtensionInventoryPreviewAndSecretsOutputParity(t *testing.T) {
	t.Parallel()

	inventory := ExtensionInventoryRecord{
		Extension: "alpha", Enabled: true,
		Items: []ExtensionKitItemRecord{{Kind: "agent", ID: "agent/alpha/writer", Name: "writer", Live: true}},
	}
	secrets := ExtensionSecretsRecord{
		DeclaredEnv:  []string{"API_KEY"},
		BoundEnvKeys: []string{"API_KEY", "OLD_KEY"},
		Bindings: []contract.ExtensionSecretBindingPayload{
			{EnvName: "API_KEY", MCPServer: "deployment-api", HeaderName: "X-Deployment-Key"},
			{EnvName: "OLD_KEY", Stale: true},
		},
	}

	testCases := []struct {
		name       string
		bundle     outputBundle
		wantHuman  []string
		wantTOON   []string
		newDecoded func() any
		want       any
	}{
		{
			name:       "Should render inventory",
			bundle:     extensionInventoryBundle(inventory),
			wantHuman:  []string{"Extension inventory", "writer", "agent/alpha/writer", "true"},
			wantTOON:   []string{"extension_inventory", "items", "writer"},
			newDecoded: func() any { return &ExtensionInventoryRecord{} },
			want:       inventory,
		},
		{
			name:   "Should render presence-only secrets",
			bundle: extensionSecretsBundle(secrets),
			wantHuman: []string{
				"Extension Secrets", "API_KEY", "remote-header deployment-api:X-Deployment-Key",
				"OLD_KEY", "Stale Bindings",
			},
			wantTOON: []string{
				"extension_secrets", "stale_env_keys", "OLD_KEY", "bindings", "deployment-api", "X-Deployment-Key",
			},
			newDecoded: func() any { return &ExtensionSecretsRecord{} },
			want:       secrets,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			human, err := testCase.bundle.human()
			if err != nil {
				t.Fatalf("human output error = %v", err)
			}
			for _, want := range testCase.wantHuman {
				if !strings.Contains(human, want) {
					t.Fatalf("human output = %q, want %q", human, want)
				}
			}
			for _, format := range []OutputFormat{OutputJSON, OutputJSONL} {
				t.Run("Should preserve the payload in "+string(format)+" output", func(t *testing.T) {
					t.Parallel()

					encoded, err := renderExtensionOutput(t, testCase.bundle, format)
					if err != nil {
						t.Fatalf("writeCommandOutput(%s) error = %v", format, err)
					}
					decoded := testCase.newDecoded()
					if err := json.Unmarshal([]byte(strings.TrimSpace(encoded)), decoded); err != nil {
						t.Fatalf("json.Unmarshal(%s) error = %v; output=%q", format, err, encoded)
					}
					if !reflect.DeepEqual(reflect.ValueOf(decoded).Elem().Interface(), testCase.want) {
						t.Fatalf("%s decoded = %#v, want %#v", format, decoded, testCase.want)
					}
				})
			}
			toon, err := renderExtensionOutput(t, testCase.bundle, OutputToon)
			if err != nil {
				t.Fatalf("writeCommandOutput(toon) error = %v", err)
			}
			for _, want := range testCase.wantTOON {
				if !strings.Contains(toon, want) {
					t.Fatalf("toon output = %q, want %q", toon, want)
				}
			}
		})
	}
}

func renderExtensionOutput(t *testing.T, bundle outputBundle, format OutputFormat) (string, error) {
	t.Helper()

	command := &cobra.Command{Use: "test"}
	command.Flags().StringP(outputFlagName, "o", string(format), "")
	command.Flags().Bool(jsonFlagName, false, "")
	var output bytes.Buffer
	command.SetOut(&output)
	err := writeCommandOutput(command, bundle)
	return output.String(), err
}

// Suite: extension CLI output contracts
// Invariant: extension bundles preserve machine parity while human output explains discovery, health, and next steps.
// Boundary IN: extension output bundle composition.
// Boundary OUT: daemon payload computation, owned by API and extension runtime suites.
package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func TestExtensionSingleRecordBundlesEmitJSONL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		bundle outputBundle
	}{
		{name: "Should emit status as one JSON object", bundle: extensionBundle(ExtensionRecord{Name: "alpha"})},
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

		bundle := extensionBundle(ExtensionRecord{
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

			bundle := extensionSuccessBundle(test.verb, ExtensionRecord{Name: "alpha"})
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
		bundle := extensionEnableBundle(result)
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

		human, err := extensionEnableBundle(ExtensionEnableRecord{
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
	preview := ExtensionEnablePreviewRecord{
		Extension: "alpha",
		WouldPublish: []ExtensionKitItemRecord{{
			Kind: "automation.job", ID: "automation.job/alpha/daily", Name: "alpha/daily",
		}},
		AgentConflicts:              []string{"writer"},
		MissingEnv:                  []string{"API_KEY"},
		AutomationStarting:          []string{"alpha/daily"},
		NetworkRequirementDigest:    "digest-current",
		NetworkConfirmationRequired: true,
	}
	secrets := ExtensionSecretsRecord{
		DeclaredEnv:  []string{"API_KEY"},
		BoundEnvKeys: []string{"API_KEY", "OLD_KEY"},
		Bindings: []contract.ExtensionSecretBindingPayload{
			{EnvName: "API_KEY"},
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
			name:       "Should render preview",
			bundle:     extensionEnablePreviewBundle(preview),
			wantHuman:  []string{"Enable preview", "writer", "API_KEY", "alpha/daily", "digest-current"},
			wantTOON:   []string{"extension_preview", "would_publish", "digest-current"},
			newDecoded: func() any { return &ExtensionEnablePreviewRecord{} },
			want:       preview,
		},
		{
			name:       "Should render presence-only secrets",
			bundle:     extensionSecretsBundle(secrets),
			wantHuman:  []string{"Extension Secrets", "API_KEY", "OLD_KEY", "Stale Bindings"},
			wantTOON:   []string{"extension_secrets", "stale_env_keys", "OLD_KEY"},
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
			if strings.Contains(human+toon, "vault:") || strings.Contains(human+toon, "planted-secret") {
				t.Fatalf("output leaked secret material: human=%q toon=%q", human, toon)
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

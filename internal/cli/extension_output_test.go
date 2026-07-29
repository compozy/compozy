// Suite: extension CLI output contracts
// Invariant: extension bundles preserve machine parity while human output explains discovery, health, and next steps.
// Boundary IN: extension output bundle composition.
// Boundary OUT: daemon payload computation, owned by API and extension runtime suites.
package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

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

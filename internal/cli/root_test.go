// Suite: CLI execution error rendering
// Invariant: human diagnostics preserve the actionable message and suggested command carried by structured output.
// Boundary IN: root command execution error renderer.
// Boundary OUT: diagnostic authoring, owned by the originating domain packages.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

func TestRenderHumanExecutionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      error
		contains []string
	}{
		{
			name: "Should render policy remediation and configuration command",
			err: extensionpkg.ValidateUnverifiedSideLoad(
				"acme/blocked",
				"local_path",
				false,
				true,
			),
			contains: []string{
				"error: Unverified extension install is blocked",
				"This side-load is disabled by extensions marketplace policy.",
				"try: compozy config set extensions.trust.allow_unverified true",
			},
		},
		{
			name: "Should render daemon remediation command",
			err: newDaemonUnavailableError(
				"/tmp/compozy.sock",
				"GET",
				"/api/extensions",
				errors.New("connection refused"),
			),
			contains: []string{
				"error: Daemon unavailable",
				"CompozyOS daemon is not reachable at /tmp/compozy.sock",
				"try: compozy daemon start",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			if exitCode := writeExecutionError(&stderr, nil, test.err); exitCode == 0 {
				t.Fatal("writeExecutionError() exit code = 0, want failure")
			}
			for _, want := range test.contains {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("writeExecutionError() = %q, want %q", stderr.String(), want)
				}
			}
		})
	}
}

func TestRenderExtensionOperationExecutionError(t *testing.T) {
	t.Parallel()

	payload := contract.ExtensionOperationErrorPayload{
		Error:         "network confirmation required",
		Code:          "extension_network_confirmation_required",
		CurrentDigest: "digest-current",
		Diagnostic: &contract.DiagnosticItem{
			ID:               "extension.network_confirmation_required",
			Code:             "extension_network_confirmation_required",
			Category:         "extension",
			Title:            "Network confirmation is required",
			Message:          "Confirm the current extension digest.",
			Severity:         "error",
			DataFreshness:    "live",
			SuggestedCommand: "compozy extension enable alpha --confirm-network-requirement digest-current",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(payload) error = %v", err)
	}
	operationErr := readAPIErrorBody(http.StatusConflict, "409 Conflict", body)

	t.Run("Should render the digest and retry command for humans", func(t *testing.T) {
		t.Parallel()

		var stderr bytes.Buffer
		if exitCode := writeExecutionError(&stderr, nil, operationErr); exitCode == 0 {
			t.Fatal("writeExecutionError() exit code = 0, want failure")
		}
		for _, want := range []string{
			"Network confirmation is required",
			"Confirm the current extension digest.",
			"compozy extension enable alpha --confirm-network-requirement digest-current",
		} {
			if !strings.Contains(stderr.String(), want) {
				t.Fatalf("human error = %q, want %q", stderr.String(), want)
			}
		}
	})

	t.Run("Should preserve extension fields in structured output", func(t *testing.T) {
		t.Parallel()

		for _, format := range []string{"json", "jsonl", "toon"} {
			t.Run("Should preserve the exact error shape in "+format+" output", func(t *testing.T) {
				t.Parallel()

				var stderr bytes.Buffer
				args := []string{"extension", "enable", "alpha", "-o", format}
				if exitCode := writeExecutionError(&stderr, args, operationErr); exitCode == 0 {
					t.Fatal("writeExecutionError() exit code = 0, want failure")
				}
				switch format {
				case "json":
					var decoded contract.ExtensionOperationErrorPayload
					if err := json.Unmarshal(stderr.Bytes(), &decoded); err != nil {
						t.Fatalf("json.Unmarshal(json error) error = %v; output=%q", err, stderr.String())
					}
					if !reflect.DeepEqual(decoded, payload) {
						t.Fatalf("json error = %#v, want %#v", decoded, payload)
					}
				case "jsonl":
					var decoded struct {
						Type  string                                  `json:"type"`
						Error contract.ExtensionOperationErrorPayload `json:"error"`
					}
					if err := json.Unmarshal(stderr.Bytes(), &decoded); err != nil {
						t.Fatalf("json.Unmarshal(jsonl error) error = %v; output=%q", err, stderr.String())
					}
					if decoded.Type != clientErrorKey || !reflect.DeepEqual(decoded.Error, payload) {
						t.Fatalf("jsonl error = %#v, want type=%q payload=%#v", decoded, clientErrorKey, payload)
					}
				case "toon":
					want := "error{code,current_digest,agents,env_name,declared_env,message}:\n" +
						"  extension_network_confirmation_required,digest-current,\"\",\"\",\"\"," +
						"network confirmation required"
					if strings.TrimSpace(stderr.String()) != want {
						t.Fatalf("TOON error = %q, want exact named fields %q", stderr.String(), want)
					}
				default:
					t.Fatalf("unexpected structured format %q", format)
				}
			})
		}
	})
}

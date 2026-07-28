package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactCollectorManifestContainsOnlyCapturedSurfaces(t *testing.T) {
	t.Parallel()

	collector := NewArtifactCollector(t)
	if err := collector.CaptureJSON(ArtifactKindTranscript, []map[string]string{{"role": "user"}}); err != nil {
		t.Fatalf("CaptureJSON(transcript) error = %v", err)
	}
	if err := collector.CaptureJSON(ArtifactKindEvents, []map[string]string{{"type": "agent_message"}}); err != nil {
		t.Fatalf("CaptureJSON(events) error = %v", err)
	}

	manifest, err := collector.WriteManifest()
	if err != nil {
		t.Fatalf("WriteManifest() error = %v", err)
	}

	if got, want := len(manifest.Artifacts), 2; got != want {
		t.Fatalf("len(manifest.Artifacts) = %d, want %d", got, want)
	}
	if got, want := manifest.Artifacts[0].Path, "events.json"; got != want {
		t.Fatalf("manifest.Artifacts[0].Path = %q, want %q", got, want)
	}
	if got, want := manifest.Artifacts[1].Path, "transcript.json"; got != want {
		t.Fatalf("manifest.Artifacts[1].Path = %q, want %q", got, want)
	}

	manifestBytes, err := os.ReadFile(collector.ManifestPath())
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", collector.ManifestPath(), err)
	}

	var persisted ArtifactManifest
	if err := json.Unmarshal(manifestBytes, &persisted); err != nil {
		t.Fatalf("json.Unmarshal(manifest) error = %v", err)
	}
	if got, want := len(persisted.Artifacts), 2; got != want {
		t.Fatalf("len(persisted.Artifacts) = %d, want %d", got, want)
	}
}

func TestArtifactCollectorCaptureFilesUsesStableDirectoryPath(t *testing.T) {
	t.Parallel()

	collector := NewArtifactCollector(t)
	first := filepath.Join(t.TempDir(), "shot-1.png")
	second := filepath.Join(t.TempDir(), "shot-2.png")
	if err := os.WriteFile(first, []byte("one"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", first, err)
	}
	if err := os.WriteFile(second, []byte("two"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", second, err)
	}

	if err := collector.CaptureFiles(ArtifactKindBrowserScreenshots, []string{first, second}, "image/png"); err != nil {
		t.Fatalf("CaptureFiles(browser_screenshots) error = %v", err)
	}

	manifest := collector.Manifest()
	if got, want := len(manifest.Artifacts), 1; got != want {
		t.Fatalf("len(manifest.Artifacts) = %d, want %d", got, want)
	}
	if got, want := manifest.Artifacts[0].Path, "browser_screenshots"; got != want {
		t.Fatalf("manifest.Artifacts[0].Path = %q, want %q", got, want)
	}

	screenshotDir, ok := collector.ArtifactPath(ArtifactKindBrowserScreenshots)
	if !ok {
		t.Fatal("ArtifactPath(browser_screenshots) = missing, want present")
	}
	for _, name := range []string{"shot-1.png", "shot-2.png"} {
		if _, err := os.Stat(filepath.Join(screenshotDir, name)); err != nil {
			t.Fatalf("os.Stat(%q) error = %v", filepath.Join(screenshotDir, name), err)
		}
	}
}

func TestArtifactCollectorCaptureTextAndFile(t *testing.T) {
	t.Parallel()

	collector := NewArtifactCollector(t)
	if err := collector.CaptureText(ArtifactKindBrowserConsole, "console output"); err != nil {
		t.Fatalf("CaptureText(browser_console) error = %v", err)
	}

	sourcePath := filepath.Join(t.TempDir(), "trace.zip")
	if err := os.WriteFile(sourcePath, []byte("trace"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", sourcePath, err)
	}
	if err := collector.CaptureFile(ArtifactKindBrowserTrace, sourcePath, "application/zip"); err != nil {
		t.Fatalf("CaptureFile(browser_trace) error = %v", err)
	}

	if _, ok := collector.ArtifactPath(ArtifactKindBrowserTrace); !ok {
		t.Fatal("ArtifactPath(browser_trace) = missing, want present")
	}
}

func TestArtifactCollectorCaptureNamedJSONUsesStableDirectoryArtifact(t *testing.T) {
	t.Parallel()

	collector := NewArtifactCollector(t)
	firstPath, err := collector.CaptureNamedJSON(
		ArtifactKindTransportOutputs,
		"runtime status",
		TransportOutputArtifact{
			Transport: "cli",
			Command:   []string{"status", "-o", "json"},
			Stdout:    `{"status":"running"}`,
		},
	)
	if err != nil {
		t.Fatalf("CaptureNamedJSON(transport_outputs) error = %v", err)
	}
	secondPath, err := collector.CaptureNamedJSON(
		ArtifactKindTransportOutputs,
		"http status",
		TransportOutputArtifact{
			Transport:  "http",
			Method:     "GET",
			URL:        "/api/status",
			StatusCode: 200,
		},
	)
	if err != nil {
		t.Fatalf("CaptureNamedJSON(transport_outputs second) error = %v", err)
	}

	manifest := collector.Manifest()
	if got, want := len(manifest.Artifacts), 1; got != want {
		t.Fatalf("len(manifest.Artifacts) = %d, want %d", got, want)
	}
	if got, want := manifest.Artifacts[0].Path, "transport_outputs"; got != want {
		t.Fatalf("manifest.Artifacts[0].Path = %q, want %q", got, want)
	}
	if got, want := filepath.Base(firstPath), "runtime-status.json"; got != want {
		t.Fatalf("filepath.Base(firstPath) = %q, want %q", got, want)
	}
	if got, want := filepath.Base(secondPath), "http-status.json"; got != want {
		t.Fatalf("filepath.Base(secondPath) = %q, want %q", got, want)
	}
	for _, path := range []string{firstPath, secondPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("os.Stat(%q) error = %v", path, err)
		}
	}
}

func TestArtifactCollectorCaptureNamedTextUsesStableDirectoryArtifact(t *testing.T) {
	t.Parallel()

	collector := NewArtifactCollector(t)
	textPath, err := collector.CaptureNamedText(
		ArtifactKindTransportOutputs,
		"cli stderr",
		"permission denied",
	)
	if err != nil {
		t.Fatalf("CaptureNamedText(transport_outputs) error = %v", err)
	}

	manifest := collector.Manifest()
	if got, want := len(manifest.Artifacts), 1; got != want {
		t.Fatalf("len(manifest.Artifacts) = %d, want %d", got, want)
	}
	if got, want := manifest.Artifacts[0].Path, "transport_outputs"; got != want {
		t.Fatalf("manifest.Artifacts[0].Path = %q, want %q", got, want)
	}
	if got, want := filepath.Base(textPath), "cli-stderr.txt"; got != want {
		t.Fatalf("filepath.Base(textPath) = %q, want %q", got, want)
	}

	content, err := os.ReadFile(textPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", textPath, err)
	}
	if got, want := string(content), "permission denied"; got != want {
		t.Fatalf("string(content) = %q, want %q", got, want)
	}
}

func TestArtifactCollectorCaptureCombinedAndToolHostArtifactsSeparately(t *testing.T) {
	t.Parallel()

	collector := NewArtifactCollector(t)
	if err := collector.CaptureJSON(
		ArtifactKindCombinedFlow,
		CombinedFlowArtifact{
			Scenario:          "automation-task-resume-network",
			SessionID:         "sess-1",
			Channel:           "ops-nightly",
			AutomationRunID:   "run-1",
			TaskID:            "task-1",
			TaskRunID:         "task-run-1",
			NetworkMessageIDs: []string{"msg-1"},
			SideEffectPaths:   []string{"/workspace/toolhost/resume.txt"},
		},
	); err != nil {
		t.Fatalf("CaptureJSON(combined_flow) error = %v", err)
	}
	if err := collector.CaptureJSON(
		ArtifactKindToolHostDiagnostics,
		ToolHostDiagnosticsArtifact{
			SessionID: "sess-1",
			Operations: []ToolHostOperationDiagnostic{{
				Operation:        "create_terminal",
				Outcome:          ToolHostOutcomeAllowed,
				SideEffectPath:   "/workspace/toolhost/resume.txt",
				SideEffectExists: true,
			}},
		},
	); err != nil {
		t.Fatalf("CaptureJSON(tool_host_diagnostics) error = %v", err)
	}

	manifest := collector.Manifest()
	if got, want := len(manifest.Artifacts), 2; got != want {
		t.Fatalf("len(manifest.Artifacts) = %d, want %d", got, want)
	}

	combinedPath, ok := collector.ArtifactPath(ArtifactKindCombinedFlow)
	if !ok {
		t.Fatal("ArtifactPath(combined_flow) = missing, want present")
	}
	toolHostPath, ok := collector.ArtifactPath(ArtifactKindToolHostDiagnostics)
	if !ok {
		t.Fatal("ArtifactPath(tool_host_diagnostics) = missing, want present")
	}
	if combinedPath == toolHostPath {
		t.Fatalf("combinedPath = %q, want distinct tool-host artifact path", combinedPath)
	}
}

func TestArtifactCollectorTargetPathStaysWithinRoot(t *testing.T) {
	t.Parallel()

	collector := NewArtifactCollector(t)

	targetPath, err := collector.targetPath(artifactSpec{relativePath: "nested/trace.json"})
	if err != nil {
		t.Fatalf("targetPath(valid) error = %v", err)
	}
	if got, want := filepath.Dir(targetPath), filepath.Join(collector.RootDir(), "nested"); got != want {
		t.Fatalf("filepath.Dir(targetPath) = %q, want %q", got, want)
	}

	if _, err := collector.targetPath(artifactSpec{relativePath: "../escape.json"}); err == nil {
		t.Fatal("targetPath(escape) error = nil, want non-nil")
	}
	if got, ok := collector.ArtifactPath(ArtifactKind("missing")); ok || got != "" {
		t.Fatalf("ArtifactPath(missing) = (%q, %t), want (\"\", false)", got, ok)
	}
}

func TestToolHostDiagnosticsHelpersDistinguishAllowedAndBlockedOutcomes(t *testing.T) {
	t.Parallel()

	diagnostics := ToolHostDiagnosticsArtifact{
		SessionID: "sess-1",
		Operations: []ToolHostOperationDiagnostic{
			{
				Operation:        "write_text_file",
				Path:             "toolhost/allowed.txt",
				Outcome:          ToolHostOutcomeAllowed,
				SideEffectPath:   "/workspace/toolhost/allowed.txt",
				SideEffectExists: true,
			},
			{
				Operation:        "create_terminal",
				Path:             "toolhost/blocked.txt",
				Outcome:          ToolHostOutcomeBlocked,
				Error:            "acp: permission denied: create_terminal blocked by approve-reads",
				SideEffectPath:   "/workspace/toolhost/blocked.txt",
				SideEffectExists: false,
			},
		},
	}

	allowed, ok := diagnostics.Allowed("write_text_file")
	if !ok {
		t.Fatal("Allowed(write_text_file) = missing, want present")
	}
	if !allowed.SideEffectExists {
		t.Fatalf("allowed.SideEffectExists = %v, want true", allowed.SideEffectExists)
	}

	blocked, ok := diagnostics.Blocked("create_terminal")
	if !ok {
		t.Fatal("Blocked(create_terminal) = missing, want present")
	}
	if blocked.Error == "" {
		t.Fatal("blocked.Error = empty, want failure detail")
	}

	if _, ok := diagnostics.Allowed("create_terminal"); ok {
		t.Fatal("Allowed(create_terminal) = present, want blocked-only classification")
	}
	if _, ok := diagnostics.Blocked("write_text_file"); ok {
		t.Fatal("Blocked(write_text_file) = present, want allowed-only classification")
	}
}

package extensions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	runtimeevents "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
	sdkextension "github.com/compozy/compozy/sdk/extension"
)

var (
	sdkExtensionBuildOnce sync.Once
	sdkExtensionBinary    string
	sdkExtensionBuildErr  error
)

func TestManagerStartRunsSDKExtensionLifecycleOverRealStdIO(t *testing.T) {
	binary := buildSDKExtensionBinary(t)
	harness := newManagerHarness(t, managerHarnessSpec{
		Name:         "sdk-ext",
		Binary:       binary,
		Capabilities: []Capability{CapabilityPromptMutate, CapabilityTasksRead},
		Hooks:        []HookDeclaration{{Event: HookPromptPostBuild}},
		Workflow:     "demo",
	})
	defer harness.Close(t)

	writeTaskFixture(t, harness.manager.workspaceRoot, "demo", 1, "pending", "Demo task", "backend", "body")

	if err := harness.manager.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	loaded := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionLoaded)
	ready := awaitEvent(t, harness.events, runtimeevents.EventKindExtensionReady)

	if loaded.Kind != runtimeevents.EventKindExtensionLoaded {
		t.Fatalf("loaded event kind = %q, want %q", loaded.Kind, runtimeevents.EventKindExtensionLoaded)
	}

	var readyPayload kinds.ExtensionReadyPayload
	decodeEventPayload(t, ready, &readyPayload)
	if readyPayload.Extension != "sdk-ext" {
		t.Fatalf("ready payload extension = %q, want %q", readyPayload.Extension, "sdk-ext")
	}

	result, err := harness.manager.DispatchMutable(
		context.Background(),
		HookPromptPostBuild,
		sdkextension.PromptPostBuildPayload{
			RunID:      "run-001",
			PromptText: "hello",
			BatchParams: sdkextension.BatchParams{
				Name: "demo",
			},
		},
	)
	if err != nil {
		t.Fatalf("DispatchMutable() error = %v", err)
	}

	updated, ok := result.(sdkextension.PromptPostBuildPayload)
	if !ok {
		t.Fatalf("result type = %T, want %T", result, sdkextension.PromptPostBuildPayload{})
	}
	if updated.PromptText != "hello\npatched-by-sdk" {
		t.Fatalf("patched prompt_text = %q, want %q", updated.PromptText, "hello\npatched-by-sdk")
	}

	records := waitForRecords(t, harness.recordPath, 2)
	listRecord := findRecord(t, records, "host_tasks_list")
	if got := listRecord.Payload["workflow"]; got != "demo" {
		t.Fatalf("host_tasks_list workflow = %#v, want %q", got, "demo")
	}
	if got := listRecord.Payload["count"]; got != float64(1) {
		t.Fatalf("host_tasks_list count = %#v, want 1", got)
	}

	hookRecord := findRecord(t, records, "execute_hook")
	if got := hookRecord.Payload["prompt_text"]; got != "hello" {
		t.Fatalf("execute_hook prompt_text = %#v, want %q", got, "hello")
	}
	if got := hookRecord.Payload["extension"]; got != "sdk-ext" {
		t.Fatalf("execute_hook extension = %#v, want %q", got, "sdk-ext")
	}

	if err := harness.manager.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	records = waitForRecords(t, harness.recordPath, 3)
	shutdownRecord := findRecord(t, records, "shutdown")
	if got := shutdownRecord.Payload["reason"]; got != shutdownReasonRunCompleted {
		t.Fatalf("shutdown reason = %#v, want %q", got, shutdownReasonRunCompleted)
	}
}

func buildSDKExtensionBinary(t *testing.T) string {
	t.Helper()

	sdkExtensionBuildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "compozy-sdk-extension-*")
		if err != nil {
			sdkExtensionBuildErr = err
			return
		}

		binary := filepath.Join(dir, "sdk-extension")
		cmd := exec.CommandContext(context.Background(), "go", "build", "-o", binary, "./testdata/sdk_extension")
		cmd.Dir = "."
		output, err := cmd.CombinedOutput()
		if err != nil {
			sdkExtensionBuildErr = fmt.Errorf("go build sdk extension: %w: %s", err, output)
			return
		}
		sdkExtensionBinary = binary
	})

	if sdkExtensionBuildErr != nil {
		t.Fatal(sdkExtensionBuildErr)
	}
	return sdkExtensionBinary
}

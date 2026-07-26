package bridgesdk

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func TestAdapterMarkersPreserveHarnessContract(t *testing.T) {
	t.Run("Should write shared marker payloads and retain secondary reporting failures", func(t *testing.T) {
		dir := t.TempDir()
		deliveryPath := filepath.Join(dir, "delivery.jsonl")
		crashPath := filepath.Join(dir, "crash.json")
		handshakePath := filepath.Join(dir, "handshake.json")
		ownershipPath := filepath.Join(dir, "ownership.json")
		statePath := filepath.Join(dir, "state.jsonl")
		ingestPath := filepath.Join(dir, "ingest.jsonl")
		startsPath := filepath.Join(dir, "starts.log")
		shutdownPath := filepath.Join(dir, "shutdown.log")
		t.Setenv(AdapterDeliveryPathEnv, deliveryPath)
		t.Setenv(AdapterCrashOncePathEnv, crashPath)
		t.Setenv(AdapterHandshakePathEnv, handshakePath)
		t.Setenv(AdapterOwnershipPathEnv, ownershipPath)
		t.Setenv(AdapterStatePathEnv, statePath)
		t.Setenv(AdapterIngestPathEnv, ingestPath)
		t.Setenv(AdapterStartsPathEnv, startsPath)
		t.Setenv(AdapterShutdownPathEnv, shutdownPath)

		var stderr strings.Builder
		markers := NewAdapterMarkers("slack", &stderr)
		markers.RecordStart(41)
		markers.RecordListen("127.0.0.1:1234")
		markers.RecordInitialize(&InitializeMarker{})
		markers.RecordOwnership(OwnershipMarker{Listed: []bridgepkg.BridgeInstance{{ID: "brg-1"}}})
		markers.RecordState(StateMarker{BridgeInstanceID: "brg-1", Status: bridgepkg.BridgeStatusReady})
		markers.RecordIngest(IngestMarker{Envelope: bridgepkg.InboundMessageEnvelope{BridgeInstanceID: "brg-1"}})
		markers.RecordShutdown(41)
		markers.RecordDelivery(DeliveryMarker{
			PID: 42,
			Request: bridgepkg.DeliveryRequest{Event: bridgepkg.DeliveryEvent{
				DeliveryID: "dlv-1",
			}},
			Error: "provider unavailable",
		})
		if !markers.ShouldCrashOnce() {
			t.Fatal("ShouldCrashOnce() = false before marker exists")
		}
		markers.RecordCrash(map[string]any{"crashed": true})
		if markers.ShouldCrashOnce() {
			t.Fatal("ShouldCrashOnce() = true after marker exists")
		}

		payload, err := os.ReadFile(deliveryPath)
		if err != nil {
			t.Fatalf("ReadFile(delivery) error = %v", err)
		}
		var delivery DeliveryMarker
		if err := json.Unmarshal(payload, &delivery); err != nil {
			t.Fatalf("Unmarshal(delivery) error = %v", err)
		}
		if delivery.PID != 42 || delivery.Request.Event.DeliveryID != "dlv-1" ||
			delivery.Error != "provider unavailable" {
			t.Fatalf("delivery marker = %#v", delivery)
		}
		if markers.Error() != nil {
			t.Fatalf("markers.Error() = %v", markers.Error())
		}
		for _, path := range []string{handshakePath, ownershipPath, statePath, ingestPath, startsPath, shutdownPath} {
			if _, err := os.Stat(path); err != nil {
				t.Fatalf("os.Stat(%q) error = %v", path, err)
			}
		}
		starts, err := os.ReadFile(startsPath)
		if err != nil {
			t.Fatalf("ReadFile(starts) error = %v", err)
		}
		if got := string(starts); !strings.Contains(got, "pid=41") ||
			!strings.Contains(got, "listen=127.0.0.1:1234") {
			t.Fatalf("starts marker = %q", got)
		}
		markers.ReportError("test report", errors.New("boom"))
		if got := stderr.String(); !strings.Contains(got, "slack: test report: boom") {
			t.Fatalf("stderr = %q, want shared provider prefix", got)
		}
	})
}

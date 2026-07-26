package extensiontest

import (
	"testing"
	"time"

	observepkg "github.com/compozy/agh/internal/observe"

	aghtestutil "github.com/compozy/agh/internal/testutil"
)

// AppendInboundUpdate appends one fake platform update line for the adapter to ingest.
func (h *Harness) AppendInboundUpdate(t testing.TB, update any) {
	t.Helper()
	AppendInboundUpdateMarker(t, h.Markers, update)
}

// WaitForHandshake waits until the adapter writes its initialize marker.
func (h *Harness) WaitForHandshake(t testing.TB, timeout time.Duration) HandshakeRecord {
	t.Helper()
	return WaitForHandshakeMarker(t, h.Markers, timeout)
}

// WaitForStates waits until the state marker file satisfies the predicate.
func (h *Harness) WaitForStates(t testing.TB, timeout time.Duration, predicate func([]StateRecord) bool) []StateRecord {
	t.Helper()
	return WaitForStateMarkers(t, h.Markers, timeout, predicate)
}

// WaitForDeliveries waits until the delivery marker file satisfies the predicate.
func (h *Harness) WaitForDeliveries(
	t testing.TB,
	timeout time.Duration,
	predicate func([]DeliveryRecord) bool,
) []DeliveryRecord {
	t.Helper()
	return WaitForDeliveryMarkers(t, h.Markers, timeout, predicate)
}

// WaitForIngests waits until the ingest marker file satisfies the predicate.
func (h *Harness) WaitForIngests(
	t testing.TB,
	timeout time.Duration,
	predicate func([]IngestRecord) bool,
) []IngestRecord {
	t.Helper()
	return WaitForIngestMarkers(t, h.Markers, timeout, predicate)
}

// ObserveHealth returns the current observer health surface.
func (h *Harness) ObserveHealth(t testing.TB) observepkg.Health {
	t.Helper()
	health, err := h.Observer.Health(aghtestutil.Context(t))
	if err != nil {
		t.Fatalf("observer.Health() error = %v", err)
	}
	return health
}

// QueryBridgeHealth returns the current per-instance bridge health rows.
func (h *Harness) QueryBridgeHealth(t testing.TB) []observepkg.BridgeInstanceHealth {
	t.Helper()
	rows, err := h.Observer.QueryBridgeHealth(aghtestutil.Context(t))
	if err != nil {
		t.Fatalf("observer.QueryBridgeHealth() error = %v", err)
	}
	return rows
}

// Report reads the current collected adapter evidence into one reusable report.
func (h *Harness) Report(t testing.TB) ConformanceReport {
	t.Helper()
	return ReportFromMarkers(t, h.Markers)
}

// AppendInboundUpdateMarker appends one fake platform update line for the
// reference adapter marker contract.
func AppendInboundUpdateMarker(t testing.TB, markers MarkerPaths, update any) {
	t.Helper()
	appendJSONLine(t, markers.Updates, update)
}

// WaitForHandshakeMarker waits until the adapter writes its initialize marker.
func WaitForHandshakeMarker(t testing.TB, markers MarkerPaths, timeout time.Duration) HandshakeRecord {
	t.Helper()
	var record HandshakeRecord
	waitForCondition(t, timeout, "adapter handshake", func() bool {
		loaded, err := readJSONFile[HandshakeRecord](markers.Handshake)
		if err != nil {
			return false
		}
		record = loaded
		return true
	})
	return record
}

// WaitForStateMarkers waits until the state marker file satisfies the predicate.
func WaitForStateMarkers(
	t testing.TB,
	markers MarkerPaths,
	timeout time.Duration,
	predicate func([]StateRecord) bool,
) []StateRecord {
	t.Helper()
	return waitForJSONLinesCondition(t, markers.State, timeout, "adapter state markers", predicate)
}

// WaitForDeliveryMarkers waits until the delivery marker file satisfies the predicate.
func WaitForDeliveryMarkers(
	t testing.TB,
	markers MarkerPaths,
	timeout time.Duration,
	predicate func([]DeliveryRecord) bool,
) []DeliveryRecord {
	t.Helper()
	return waitForJSONLinesCondition(t, markers.Delivery, timeout, "adapter delivery markers", predicate)
}

// WaitForIngestMarkers waits until the ingest marker file satisfies the predicate.
func WaitForIngestMarkers(
	t testing.TB,
	markers MarkerPaths,
	timeout time.Duration,
	predicate func([]IngestRecord) bool,
) []IngestRecord {
	t.Helper()
	return waitForJSONLinesCondition(t, markers.Ingest, timeout, "adapter ingest markers", predicate)
}

// ReportFromMarkers reads the collected adapter evidence from one marker set.
func ReportFromMarkers(t testing.TB, markers MarkerPaths) ConformanceReport {
	t.Helper()
	report := ConformanceReport{}
	if handshake, err := readJSONFile[HandshakeRecord](markers.Handshake); err == nil {
		report.Handshake = &handshake
	}
	if ownership, err := readJSONFile[OwnershipRecord](markers.Ownership); err == nil {
		report.Ownership = &ownership
	}
	if states, err := readJSONLinesFile[StateRecord](markers.State); err == nil {
		report.States = states
	}
	if deliveries, err := readJSONLinesFile[DeliveryRecord](markers.Delivery); err == nil {
		report.Deliveries = deliveries
	}
	if ingests, err := readJSONLinesFile[IngestRecord](markers.Ingest); err == nil {
		report.Ingests = ingests
	}
	return report
}

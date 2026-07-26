package extensiontest

import (
	"encoding/json"
	"strings"
	"testing"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

func recordHostStateTransition(
	t testing.TB,
	path string,
	params json.RawMessage,
	result any,
	callErr error,
) {
	t.Helper()

	record := StateRecord{}

	var request bridgecontract.BridgesInstancesReportStateParams
	if err := json.Unmarshal(params, &request); err == nil {
		record.BridgeInstanceID = strings.TrimSpace(request.BridgeInstanceID)
		record.Status = bridgeStatusDomain(request.Status.Normalize())
		record.Instance = bridgepkg.BridgeInstance{
			ID:          record.BridgeInstanceID,
			Status:      bridgeStatusDomain(request.Status.Normalize()),
			Degradation: bridgeDegradationDomain(request.Degradation),
		}
	}

	switch typed := result.(type) {
	case *bridgepkg.BridgeInstance:
		if typed != nil {
			record.Instance = copyBridgeInstance(*typed)
		}
	case bridgepkg.BridgeInstance:
		record.Instance = copyBridgeInstance(typed)
	case *bridgecontract.BridgeInstance:
		if typed != nil {
			record.Instance = bridgeInstanceDomain(*typed)
		}
	case bridgecontract.BridgeInstance:
		record.Instance = bridgeInstanceDomain(typed)
	}

	if record.BridgeInstanceID == "" {
		record.BridgeInstanceID = strings.TrimSpace(record.Instance.ID)
	}
	if record.Status == "" {
		record.Status = record.Instance.Status.Normalize()
	}
	if callErr != nil {
		record.Error = callErr.Error()
	}

	appendJSONLine(t, path, record)
}

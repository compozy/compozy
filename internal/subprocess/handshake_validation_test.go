package subprocess

import (
	"strings"
	"testing"
	"time"

	bridges "github.com/compozy/agh/internal/bridges/contract"
)

func TestInitializeBridgeRuntimeValidateContract(t *testing.T) {
	t.Parallel()

	t.Run("Should reject whitespace equivalent managed bridge instance IDs", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 15, 12, 20, 0, 0, time.UTC)
		runtime := InitializeBridgeRuntime{
			RuntimeVersion: InitializeBridgeRuntimeVersion2,
			Purpose:        BridgeRuntimePurposeService,
			Provider:       "telegram-reference",
			Platform:       "telegram",
			ManagedInstances: []InitializeBridgeManagedInstance{
				managedBridgeInstanceContract("brg-dup", now),
				managedBridgeInstanceContract(" brg-dup ", now),
			},
		}

		err := runtime.Validate()
		if err == nil {
			t.Fatal("Validate() error = nil, want duplicated managed instance error")
		}
		if !strings.Contains(err.Error(), "duplicated") {
			t.Fatalf("Validate() error = %v, want duplicated managed instance error", err)
		}
	})
}

func managedBridgeInstanceContract(id string, now time.Time) InitializeBridgeManagedInstance {
	return InitializeBridgeManagedInstance{
		Instance: bridges.BridgeInstance{
			ID:            id,
			Scope:         bridges.ScopeGlobal,
			Platform:      "telegram",
			ExtensionName: "telegram-reference",
			DisplayName:   "Telegram",
			Enabled:       true,
			Status:        bridges.BridgeStatusReady,
			RoutingPolicy: bridges.RoutingPolicy{IncludePeer: true},
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}
}

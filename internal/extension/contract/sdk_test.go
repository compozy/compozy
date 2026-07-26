package contract

import (
	"testing"

	"github.com/compozy/agh/internal/hooks"
)

func TestHookContractsResolveDescriptors(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve all hook descriptors", func(t *testing.T) {
		t.Parallel()

		contracts, err := BuildHookContracts()
		if err != nil {
			t.Fatalf("BuildHookContracts() error = %v", err)
		}
		if got, want := len(contracts), len(hooks.AllEventDescriptors()); got != want {
			t.Fatalf("len(BuildHookContracts()) = %d, want %d", got, want)
		}

		for idx, descriptor := range hooks.AllEventDescriptors() {
			if contracts[idx].Event != descriptor.Event {
				t.Fatalf("HookContracts()[%d].Event = %q, want %q", idx, contracts[idx].Event, descriptor.Event)
			}
			if contracts[idx].Payload.Name != descriptor.PayloadSchema {
				t.Fatalf(
					"HookContracts()[%d].Payload.Name = %q, want %q",
					idx,
					contracts[idx].Payload.Name,
					descriptor.PayloadSchema,
				)
			}
			if contracts[idx].Patch.Name != descriptor.PatchSchema {
				t.Fatalf(
					"HookContracts()[%d].Patch.Name = %q, want %q",
					idx,
					contracts[idx].Patch.Name,
					descriptor.PatchSchema,
				)
			}
		}
	})

	t.Run("Should resolve representative hook payload and patch names", func(t *testing.T) {
		t.Parallel()

		contracts := HookContracts()
		byEvent := make(map[hooks.HookEvent]HookContractSpec, len(contracts))
		for _, contract := range contracts {
			byEvent[contract.Event] = contract
		}
		for _, tc := range []struct {
			event   hooks.HookEvent
			payload string
			patch   string
		}{
			{
				event:   hooks.HookCoordinatorPreSpawn,
				payload: "CoordinatorPreSpawnPayload",
				patch:   "CoordinatorSpawnPatch",
			},
			{
				event:   hooks.HookTaskStatusChanged,
				payload: "TaskStatusChangedPayload",
				patch:   "TaskObservationPatch",
			},
			{
				event:   hooks.HookTaskRunPreClaim,
				payload: "TaskRunPreClaimPayload",
				patch:   "TaskRunPreClaimPatch",
			},
			{
				event:   hooks.HookLoopGenerationPre,
				payload: "LoopGenerationPrePayload",
				patch:   "LoopGenerationPrePatch",
			},
			{
				event:   hooks.HookSpawnPreCreate,
				payload: "SpawnPreCreatePayload",
				patch:   "SpawnCreatePatch",
			},
			{
				event:   hooks.HookNetworkMessagePersisted,
				payload: "NetworkMessagePersistedPayload",
				patch:   "NetworkObservationPatch",
			},
			{
				event:   hooks.HookNetworkWorkClosed,
				payload: "NetworkWorkClosedPayload",
				patch:   "NetworkObservationPatch",
			},
		} {
			contract, ok := byEvent[tc.event]
			if !ok {
				t.Fatalf("HookContracts() missing %q", tc.event)
			}
			if contract.Payload.Name != tc.payload || contract.Patch.Name != tc.patch {
				t.Fatalf(
					"%s contract payload/patch = %q/%q, want %q/%q",
					tc.event,
					contract.Payload.Name,
					contract.Patch.Name,
					tc.payload,
					tc.patch,
				)
			}
		}
	})
}

func TestSDKRootTypesDefensiveCopy(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate returned root type slice mutations", func(t *testing.T) {
		t.Parallel()

		types := SDKRootTypes()
		if len(types) == 0 {
			t.Fatal("SDKRootTypes() returned no types")
		}

		original := types[0].Name
		types[0].Name = "MutatedRootType"

		next := SDKRootTypes()
		if next[0].Name != original {
			t.Fatalf("SDKRootTypes()[0].Name = %q after mutation, want %q", next[0].Name, original)
		}
	})
}

func TestNamedHookTypeErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should reject unknown hook contract type", func(t *testing.T) {
		t.Parallel()

		_, err := namedHookType("DefinitelyMissingHookType")
		if err == nil {
			t.Fatal("namedHookType() error = nil, want error")
		}
		if got, want := err.Error(), `unknown hook contract type "DefinitelyMissingHookType"`; got != want {
			t.Fatalf("namedHookType() error = %q, want %q", got, want)
		}
	})
}

func TestHookContractsCompatibilityWrapper(t *testing.T) {
	t.Parallel()

	t.Run("Should match error-returning hook contracts", func(t *testing.T) {
		t.Parallel()

		contracts, err := BuildHookContracts()
		if err != nil {
			t.Fatalf("BuildHookContracts() error = %v", err)
		}
		wrapped := HookContracts()
		if len(wrapped) != len(contracts) {
			t.Fatalf("len(HookContracts()) = %d, want %d", len(wrapped), len(contracts))
		}
		for idx := range contracts {
			if wrapped[idx].Event != contracts[idx].Event ||
				wrapped[idx].Payload.Name != contracts[idx].Payload.Name ||
				wrapped[idx].Patch.Name != contracts[idx].Patch.Name {
				t.Fatalf("HookContracts()[%d] = %#v, want %#v", idx, wrapped[idx], contracts[idx])
			}
		}
	})
}

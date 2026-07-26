package contract

import (
	"reflect"
	"testing"

	apicontract "github.com/compozy/agh/internal/api/contract"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
	extensionprotocol "github.com/compozy/agh/internal/extensionprotocol"
)

func TestHostAPIMethodSpecsFollowProtocolWireOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should follow protocol wire order", func(t *testing.T) {
		t.Parallel()

		specs := HostAPIMethodSpecs()
		wantOrder := extensionprotocol.AllHostAPIMethods()
		if len(specs) != len(wantOrder) {
			t.Fatalf("len(HostAPIMethodSpecs()) = %d, want %d", len(specs), len(wantOrder))
		}

		for idx := range wantOrder {
			if specs[idx].Method != wantOrder[idx] {
				t.Fatalf("HostAPIMethodSpecs()[%d].Method = %q, want %q", idx, specs[idx].Method, wantOrder[idx])
			}
		}
	})

	t.Run("Should root bridge method named types in the bridge wire contract", func(t *testing.T) {
		t.Parallel()

		type expectedTypes struct {
			paramsName string
			paramsType reflect.Type
			resultName string
			resultType reflect.Type
		}
		want := map[HostAPIMethod]expectedTypes{
			extensionprotocol.HostAPIMethodBridgesInstancesList: {
				paramsName: "EmptyResult", paramsType: reflect.TypeFor[EmptyResult](),
				resultName: "BridgeInstance", resultType: reflect.TypeFor[[]bridgecontract.BridgeInstance](),
			},
			extensionprotocol.HostAPIMethodBridgesInstancesGet: {
				paramsName: "BridgeInstanceTargetParams",
				paramsType: reflect.TypeFor[bridgecontract.BridgeInstanceTargetParams](),
				resultName: "BridgeInstance", resultType: reflect.TypeFor[bridgecontract.BridgeInstance](),
			},
			extensionprotocol.HostAPIMethodBridgesInstancesReportState: {
				paramsName: "BridgesInstancesReportStateParams",
				paramsType: reflect.TypeFor[bridgecontract.BridgesInstancesReportStateParams](),
				resultName: "BridgeInstance", resultType: reflect.TypeFor[bridgecontract.BridgeInstance](),
			},
			extensionprotocol.HostAPIMethodBridgesMessagesIngest: {
				paramsName: "InboundMessageEnvelope",
				paramsType: reflect.TypeFor[bridgecontract.InboundMessageEnvelope](),
				resultName: "BridgesMessagesIngestResult",
				resultType: reflect.TypeFor[bridgecontract.BridgesMessagesIngestResult](),
			},
		}

		for _, spec := range HostAPIMethodSpecs() {
			expected, ok := want[spec.Method]
			if !ok {
				continue
			}
			if spec.Params.Name != expected.paramsName || reflect.TypeOf(spec.Params.Value) != expected.paramsType {
				t.Fatalf(
					"HostAPIMethodSpecs()[%q].Params = (%q, %T), want (%q, %v)",
					spec.Method, spec.Params.Name, spec.Params.Value, expected.paramsName, expected.paramsType,
				)
			}
			if spec.Result.Name != expected.resultName || reflect.TypeOf(spec.Result.Value) != expected.resultType {
				t.Fatalf(
					"HostAPIMethodSpecs()[%q].Result = (%q, %T), want (%q, %v)",
					spec.Method, spec.Result.Name, spec.Result.Value, expected.resultName, expected.resultType,
				)
			}
			delete(want, spec.Method)
		}
		if len(want) != 0 {
			t.Fatalf("HostAPIMethodSpecs() missing bridge methods: %#v", want)
		}
	})
}

func TestHostAPIMethodSpecsDefensiveCopy(t *testing.T) {
	t.Parallel()

	t.Run("Should isolate returned spec slice mutations", func(t *testing.T) {
		t.Parallel()

		specs := HostAPIMethodSpecs()
		if len(specs) == 0 {
			t.Fatal("HostAPIMethodSpecs() returned no specs")
		}

		original := specs[0].Method
		specs[0].Method = HostAPIMethod("mutated")

		next := HostAPIMethodSpecs()
		if next[0].Method != original {
			t.Fatalf("HostAPIMethodSpecs()[0].Method = %q after mutation, want %q", next[0].Method, original)
		}
	})
}

func TestHostAPIMethodSpecsPagedCollectionResults(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the pagination envelopes returned by collection handlers", func(t *testing.T) {
		t.Parallel()

		want := map[HostAPIMethod]NamedType{
			HostAPIMethodAutomationJobs: {
				Name:  "AutomationJobsResult",
				Value: AutomationJobsResult{},
			},
			HostAPIMethodAutomationTriggers: {
				Name:  "AutomationTriggersResult",
				Value: AutomationTriggersResult{},
			},
			HostAPIMethodTasks: {
				Name:  "TasksResponse",
				Value: apicontract.TasksResponse{},
			},
		}

		for _, spec := range HostAPIMethodSpecs() {
			wantResult, ok := want[spec.Method]
			if !ok {
				continue
			}
			if spec.Result.Name != wantResult.Name {
				t.Fatalf(
					"HostAPIMethodSpecs()[%q].Result.Name = %q, want %q",
					spec.Method,
					spec.Result.Name,
					wantResult.Name,
				)
			}
			if got, expected := reflect.TypeOf(spec.Result.Value), reflect.TypeOf(wantResult.Value); got != expected {
				t.Fatalf("HostAPIMethodSpecs()[%q].Result.Value type = %v, want %v", spec.Method, got, expected)
			}
			delete(want, spec.Method)
		}

		if len(want) != 0 {
			t.Fatalf("HostAPIMethodSpecs() missing paged methods: %#v", want)
		}
	})
}

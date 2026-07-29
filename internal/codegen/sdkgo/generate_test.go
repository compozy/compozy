package sdkgo

import (
	"bytes"
	"slices"
	"testing"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/sdk/go/contracts"
)

func TestGenerate(t *testing.T) {
	t.Parallel()

	t.Run("Should match contract counts and emit describe and conformance contracts", func(t *testing.T) {
		t.Parallel()

		result, err := Generate()
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		second, err := Generate()
		if err != nil {
			t.Fatalf("Generate() second call error = %v", err)
		}
		if !slices.Equal(result.FileNames(), second.FileNames()) {
			t.Fatalf("Generate() file names changed: %v != %v", result.FileNames(), second.FileNames())
		}
		for _, name := range result.FileNames() {
			if !bytes.Equal(result.Files[name], second.Files[name]) {
				t.Fatalf("Generate() output %s changed between calls", name)
			}
		}
		hooks, err := extensioncontract.BuildHookContracts()
		if err != nil {
			t.Fatalf("BuildHookContracts() error = %v", err)
		}
		if got, want := result.HostMethodCount, len(extensioncontract.HostAPIMethodSpecs()); got != want {
			t.Fatalf("HostMethodCount = %d, want %d", got, want)
		}
		if got, want := result.HookEventCount, len(hooks); got != want {
			t.Fatalf("HookEventCount = %d, want %d", got, want)
		}

		var generated []byte
		for _, name := range result.FileNames() {
			generated = append(generated, result.Files[name]...)
		}
		for _, symbol := range [][]byte{
			[]byte("type DescribePayload struct"),
			[]byte("Tools            []ExtensionToolRuntimeDescriptor"),
			[]byte("CommandGroups    []ExtensionCommandGroupSpec"),
			[]byte("type ProvideConformanceFixture struct"),
			[]byte("func PublicProvideConformanceFixtures()"),
		} {
			if !bytes.Contains(generated, symbol) {
				t.Fatalf("Generate() output missing %q", symbol)
			}
		}
	})

	t.Run("Should expose generated required methods for bridge and model provides", func(t *testing.T) {
		t.Parallel()

		bridgeMethods := contracts.RequiredMethods("bridge.adapter")
		for _, method := range []string{"bridges/deliver", "bridges/targets/snapshot"} {
			if !slices.Contains(bridgeMethods, method) {
				t.Fatalf("RequiredMethods(bridge.adapter) = %v, want %q", bridgeMethods, method)
			}
		}
		if got := contracts.RequiredMethods("model.source"); !slices.Contains(got, "models/list") {
			t.Fatalf("RequiredMethods(model.source) = %v, want models/list", got)
		}
		if got := contracts.RequiredMethods("unknown.provide"); got != nil {
			t.Fatalf("RequiredMethods(unknown.provide) = %v, want nil", got)
		}
	})
}

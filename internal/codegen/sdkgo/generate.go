package sdkgo

import (
	"fmt"
	"maps"
	"sort"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

// Result is the complete deterministic Go SDK contract tree.
type Result struct {
	Files           map[string][]byte
	HostMethodCount int
	HookEventCount  int
}

// Generate renders the public Go SDK contracts from the daemon contract authority.
func Generate() (Result, error) {
	hostSpecs := extensioncontract.HostAPIMethodSpecs()
	hookSpecs, err := extensioncontract.BuildHookContracts()
	if err != nil {
		return Result{}, fmt.Errorf("build hook contracts: %w", err)
	}
	methodNames := make([]string, 0, len(hostSpecs))
	for _, spec := range hostSpecs {
		methodNames = append(methodNames, string(spec.Method))
	}
	if err := assertUniqueIdentifiers(methodNames); err != nil {
		return Result{}, err
	}
	eventNames := make([]string, 0, len(hookSpecs))
	for _, spec := range hookSpecs {
		eventNames = append(eventNames, string(spec.Event))
	}
	if err := assertUniqueIdentifiers(eventNames); err != nil {
		return Result{}, err
	}

	generator := newTypeGenerator()
	for _, root := range extensioncontract.SDKRootTypes() {
		if err := generator.addNamed(root); err != nil {
			return Result{}, err
		}
	}
	for _, spec := range hostSpecs {
		if err := generator.addNamed(spec.Params); err != nil {
			return Result{}, err
		}
		if err := generator.addNamed(spec.Result); err != nil {
			return Result{}, err
		}
	}
	for _, spec := range hookSpecs {
		if err := generator.addNamed(spec.Payload); err != nil {
			return Result{}, err
		}
		if err := generator.addNamed(spec.Patch); err != nil {
			return Result{}, err
		}
	}

	typeFiles, err := generator.renderFiles()
	if err != nil {
		return Result{}, err
	}
	files := make(map[string][]byte, len(typeFiles)+5)
	maps.Copy(files, typeFiles)
	renderers := map[string]func() ([]byte, error){
		"host_methods_gen.go":   func() ([]byte, error) { return renderHostMethods(hostSpecs) },
		"host_contracts_gen.go": func() ([]byte, error) { return renderHostContracts(hostSpecs) },
		"hook_events_gen.go":    func() ([]byte, error) { return renderHookEvents(hookSpecs) },
		"hook_contracts_gen.go": func() ([]byte, error) { return renderHookContracts(hookSpecs) },
		"capabilities_gen.go": func() ([]byte, error) {
			return renderCapabilityContracts(extensioncontract.ProvideCapabilityContracts())
		},
	}
	for name, render := range renderers {
		content, renderErr := render()
		if renderErr != nil {
			return Result{}, fmt.Errorf("render generated %s: %w", name, renderErr)
		}
		files[name] = content
	}

	return Result{
		Files:           files,
		HostMethodCount: len(hostSpecs),
		HookEventCount:  len(hookSpecs),
	}, nil
}

// FileNames returns result paths in deterministic order.
func (r Result) FileNames() []string {
	names := make([]string, 0, len(r.Files))
	for name := range r.Files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

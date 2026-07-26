package contract

import (
	"fmt"

	"github.com/compozy/agh/internal/hooks"
)

// BuildHookContracts returns the canonical hook payload/patch registry in event order.
func BuildHookContracts() ([]HookContractSpec, error) {
	descriptors := hooks.AllEventDescriptors()
	specs := make([]HookContractSpec, 0, len(descriptors))
	for _, descriptor := range descriptors {
		payload, err := namedHookType(descriptor.PayloadSchema)
		if err != nil {
			return nil, fmt.Errorf("hook contract %q payload schema: %w", descriptor.Event, err)
		}
		patch, err := namedHookType(descriptor.PatchSchema)
		if err != nil {
			return nil, fmt.Errorf("hook contract %q patch schema: %w", descriptor.Event, err)
		}
		specs = append(specs, HookContractSpec{
			Event:   descriptor.Event,
			Payload: payload,
			Patch:   patch,
		})
	}
	return specs, nil
}

// HookContracts returns the canonical hook payload/patch registry in event order.
func HookContracts() []HookContractSpec {
	specs, err := BuildHookContracts()
	if err != nil {
		panic(err)
	}
	return specs
}

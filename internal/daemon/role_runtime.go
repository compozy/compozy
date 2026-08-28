package daemon

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

type resolvedRoleRuntime struct {
	speed      speedpkg.Speed
	acpOptions []compozyconfig.ACPOptionSelection
}

func (r *ResolvedRole) speedValue() speedpkg.Speed {
	if r == nil || r.runtime == nil {
		return ""
	}
	return r.runtime.speed
}

func (r *ResolvedRole) acpOptionsValue() []compozyconfig.ACPOptionSelection {
	if r == nil || r.runtime == nil {
		return nil
	}
	return r.runtime.acpOptions
}

func (r *ResolvedRole) setRuntime(
	speed speedpkg.Speed,
	options []compozyconfig.ACPOptionSelection,
) {
	normalizedOptions := compozyconfig.CloneACPOptionSelections(options)
	if speed == "" && len(normalizedOptions) == 0 {
		r.runtime = nil
		return
	}
	r.runtime = &resolvedRoleRuntime{speed: speed, acpOptions: normalizedOptions}
}

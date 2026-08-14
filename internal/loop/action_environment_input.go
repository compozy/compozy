package loop

import "github.com/compozy/compozy/internal/loop/dsl"

// EnvironmentValue returns the action environment without exposing optional storage details.
func (in ActionExecutionInput) EnvironmentValue() dsl.EnvironmentSpec {
	return environmentSpecValue(in.Environment)
}

// EnvironmentValue returns the binding environment without exposing optional storage details.
func (r ActionSessionBindRequest) EnvironmentValue() dsl.EnvironmentSpec {
	return environmentSpecValue(r.Environment)
}

func environmentSpecValue(spec *dsl.EnvironmentSpec) dsl.EnvironmentSpec {
	if spec == nil {
		return dsl.EnvironmentSpec{}
	}
	return *spec
}

func cloneEnvironmentSpec(spec dsl.EnvironmentSpec) *dsl.EnvironmentSpec {
	cloned := spec
	return &cloned
}

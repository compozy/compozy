package spec

func cloneOperationSpecs(specs []OperationSpec) []OperationSpec {
	if len(specs) == 0 {
		return nil
	}

	cloned := make([]OperationSpec, len(specs))
	for index, spec := range specs {
		cloned[index] = cloneOperationSpec(spec)
	}
	return cloned
}

func cloneOperationSpec(spec OperationSpec) OperationSpec {
	spec.Tags = append([]string(nil), spec.Tags...)
	spec.Transports = append([]Transport(nil), spec.Transports...)
	spec.Parameters = cloneParameterSpecs(spec.Parameters)
	spec.RequestBody = cloneSpecValue(spec.RequestBody)
	spec.Responses = cloneResponseSpecs(spec.Responses)
	return spec
}

func cloneParameterSpecs(specs []ParameterSpec) []ParameterSpec {
	if len(specs) == 0 {
		return nil
	}

	cloned := make([]ParameterSpec, len(specs))
	for index, spec := range specs {
		cloned[index] = spec
		cloned[index].Enum = append([]string(nil), spec.Enum...)
	}
	return cloned
}

func cloneResponseSpecs(specs []ResponseSpec) []ResponseSpec {
	if len(specs) == 0 {
		return nil
	}

	cloned := make([]ResponseSpec, len(specs))
	for index, spec := range specs {
		cloned[index] = spec
		cloned[index].Body = cloneSpecValue(spec.Body)
	}
	return cloned
}

func cloneSpecValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, item := range typed {
			cloned[key] = cloneSpecValue(item)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneSpecValue(item)
		}
		return cloned
	default:
		return value
	}
}

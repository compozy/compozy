package common

import (
	"fmt"
	"maps"

	"dario.cat/mergo"
)

type (
	Input  map[string]any
	Output map[string]any
)

func merge(dst, src map[string]any, kind string) (map[string]any, error) {
	result := make(map[string]any)
	maps.Copy(result, dst)
	if err := mergo.Merge(&result, src, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
		return nil, fmt.Errorf("failed to merge %s: %w", kind, err)
	}
	return result, nil
}

func (i *Input) Merge(other Input) (Input, error) {
	if i == nil {
		return other, nil
	}
	return merge(*i, other, "input")
}

func (o *Output) Merge(other Output) (Output, error) {
	if o == nil {
		return other, nil
	}
	return merge(*o, other, "output")
}

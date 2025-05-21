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

// -----------------------------------------------------------------------------
// Input
// -----------------------------------------------------------------------------

func (i *Input) Merge(other Input) (Input, error) {
	if i == nil {
		return other, nil
	}
	return merge(*i, other, "input")
}

func (i *Input) Prop(key string) any {
	if i == nil {
		return nil
	}
	return (*i)[key]
}

func (i *Input) Set(key string, value any) {
	if i == nil {
		return
	}
	(*i)[key] = value
}

func (i *Input) ToProtoBufMap() (map[string]any, error) {
	return DefaultToProtoMap(*i)
}

// -----------------------------------------------------------------------------
// Output
// -----------------------------------------------------------------------------

func (o *Output) Merge(other Output) (Output, error) {
	if o == nil {
		return other, nil
	}
	return merge(*o, other, "output")
}

func (o *Output) Prop(key string) any {
	if o == nil {
		return nil
	}
	return (*o)[key]
}

func (o *Output) Set(key string, value any) {
	if o == nil {
		return
	}
	(*o)[key] = value
}

func (o *Output) ToProtoBufMap() (map[string]any, error) {
	return DefaultToProtoMap(*o)
}

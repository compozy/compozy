package core

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

func NewInput(m map[string]any) Input {
	if m == nil {
		return make(Input)
	}
	return Input(m)
}

func (i *Input) Merge(other *Input) (*Input, error) {
	if i == nil {
		return other, nil
	}
	result, err := merge(*i, *other, "input")
	if err != nil {
		return nil, err
	}
	newInput := Input(result)
	return &newInput, nil
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

func (i *Input) AsMap() map[string]any {
	if i == nil {
		return nil
	}
	result := make(map[string]any)
	maps.Copy(result, *i)
	return result
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

func (o *Output) AsMap() map[string]any {
	if o == nil {
		return nil
	}
	result := make(map[string]any)
	maps.Copy(result, *o)
	return result
}

// DeepCopy creates a deep copy of Input
func (i *Input) Clone() (*Input, error) {
	return DeepCopy(i)
}

// DeepCopy creates a deep copy of Output
func (o *Output) Clone() (*Output, error) {
	return DeepCopy(o)
}

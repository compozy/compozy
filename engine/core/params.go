package core

type (
	Input  map[string]any
	Output map[string]any
)

// OutputRootKey is the canonical map key used to store non-object JSON values
// (arrays, scalars) inside a Output map. When an action returns structured
// output whose schema root is not "object", the orchestrator wraps the value
// under this key so downstream consumers can still access it in a consistent way.
const OutputRootKey = "__value__"

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
	var source Input
	if other != nil {
		source = *other
	}
	result, err := Merge(*i, source, "input")
	if err != nil {
		return nil, err
	}
	*i = result
	return i, nil
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
	return CloneMap(map[string]any(*i))
}

// -----------------------------------------------------------------------------
// Output
// -----------------------------------------------------------------------------

func (o *Output) Merge(other Output) (Output, error) {
	if o == nil {
		return other, nil
	}
	result, err := Merge(*o, other, "output")
	if err != nil {
		return nil, err
	}
	*o = result
	return *o, nil
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
	return CloneMap(map[string]any(*o))
}

// DeepCopy creates a deep copy of Input
func (i *Input) Clone() (*Input, error) {
	return DeepCopy(i)
}

// DeepCopy creates a deep copy of Output
func (o *Output) Clone() (*Output, error) {
	return DeepCopy(o)
}

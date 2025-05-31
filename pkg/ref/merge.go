package ref

import (
	"reflect"

	"dario.cat/mergo"
	"github.com/pkg/errors"
)

// ApplyMergeMode merges refValue with inlineValue according to r.Mode.
func (r *Ref) ApplyMergeMode(refValue, inlineValue any) (any, error) {
	switch r.Mode {
	case ModeReplace:
		return refValue, nil
	case ModeAppend:
		refSlice, refOK := toSlice(refValue)
		inlineSlice, inlineOK := toSlice(inlineValue)
		if !refOK || !inlineOK {
			return nil, errors.New("append mode requires both values to be arrays")
		}
		return append(inlineSlice, refSlice...), nil
	case ModeMerge, "":
		refMap, refOK := toMap(refValue)
		inlineMap, inlineOK := toMap(inlineValue)
		if !refOK || !inlineOK {
			return refValue, nil
		}
		result := make(map[string]any)
		if err := mergo.Map(&result, inlineMap, mergo.WithAppendSlice); err != nil {
			return nil, err
		}
		if err := mergo.Map(&result, refMap, mergo.WithOverride, mergo.WithAppendSlice); err != nil {
			return nil, err
		}
		return result, nil
	default:
		return nil, errors.Errorf("unknown merge mode: %s", r.Mode)
	}
}

func toMap(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	if ok {
		return m, true
	}
	if v == nil {
		return nil, false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Struct {
		out := make(map[string]any)
		for i := 0; i < rv.NumField(); i++ {
			f := rv.Type().Field(i)
			if !f.IsExported() {
				continue
			}
			out[f.Name] = rv.Field(i).Interface()
		}
		return out, true
	}
	return nil, false
}

func toSlice(v any) ([]any, bool) {
	s, ok := v.([]any)
	if ok {
		return s, true
	}
	if v == nil {
		return nil, false
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice {
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = rv.Index(i).Interface()
		}
		return out, true
	}
	return nil, false
}

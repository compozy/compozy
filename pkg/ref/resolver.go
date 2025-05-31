package ref

import (
	"context"
	"reflect"
)

// Meta provides optional file context for resolution.
type Meta struct {
	FilePath string
	RefPath  string
}

// Resolver resolves references within structs, maps, or slices.
type Resolver struct {
	ProjectRoot string
	Mode        Mode
}

// NewResolver creates a new Resolver with the given project root.
func NewResolver(projectRoot string, opts ...func(*Resolver)) *Resolver {
	r := &Resolver{ProjectRoot: projectRoot, Mode: ModeMerge}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Resolve resolves all references found within v. The value can be a struct pointer,
// map[string]any, or []any.
func (r *Resolver) Resolve(ctx context.Context, v any, meta ...Meta) (any, error) {
	wr := &WithRef{}
	file := ""
	if len(meta) > 0 {
		file = meta[0].FilePath
	}
	wr.SetRefMetadata(file, r.ProjectRoot)

	switch t := v.(type) {
	case map[string]any:
		return wr.ResolveMapReference(ctx, t, t)
	case []any:
		result := make([]any, len(t))
		for i, item := range t {
			resolved, err := r.Resolve(ctx, item, meta...)
			if err != nil {
				return nil, err
			}
			result[i] = resolved
		}
		return result, nil
	default:
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr && rv.Elem().Kind() == reflect.Struct {
			err := wr.ResolveReferences(ctx, v, nil)
			return v, err
		}
		return v, nil
	}
}

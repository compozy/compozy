package tasks

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var BuiltinTypes = []string{
	"frontend",
	"backend",
	"docs",
	"test",
	"infra",
	"refactor",
	"chore",
	"bugfix",
}

var taskTypeSlugPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)

type TypeRegistry struct {
	values []string
	index  map[string]struct{}
}

func NewRegistry(configured []string) (*TypeRegistry, error) {
	resolved := configured
	if configured == nil {
		resolved = slices.Clone(BuiltinTypes)
	} else if len(configured) == 0 {
		return nil, errors.New("task type list cannot be empty")
	}

	values := make([]string, 0, len(resolved))
	index := make(map[string]struct{}, len(resolved))
	for _, raw := range resolved {
		slug := strings.TrimSpace(raw)
		if !taskTypeSlugPattern.MatchString(slug) {
			return nil, fmt.Errorf("task type %q must match %s", raw, taskTypeSlugPattern.String())
		}
		if _, exists := index[slug]; exists {
			return nil, fmt.Errorf("duplicate task type %q", slug)
		}
		index[slug] = struct{}{}
		values = append(values, slug)
	}

	slices.Sort(values)
	return &TypeRegistry{
		values: values,
		index:  index,
	}, nil
}

func (r *TypeRegistry) IsAllowed(slug string) bool {
	if r == nil {
		return false
	}
	_, ok := r.index[slug]
	return ok
}

func (r *TypeRegistry) Values() []string {
	if r == nil {
		return nil
	}
	return slices.Clone(r.values)
}

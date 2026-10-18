package daemon

import (
	"strings"

	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/httpapi"
	"github.com/compozy/compozy/internal/api/spec"
)

var runtimeOpenAPIDocument = core.NewOpenAPIDocument(spec.Render)

func appAccessPolicy() httpapi.AppAccessPolicy {
	return buildAppAccessPolicy(spec.Operations())
}

func buildAppAccessPolicy(operations []spec.OperationSpec) httpapi.AppAccessPolicy {
	decisions := make(map[string]bool)
	for _, operation := range operations {
		key := operation.Method + " " + appAccessRoutePath(operation.Path)
		allowed := operation.AppAccess == spec.AppAccessAllowed
		if previous, exists := decisions[key]; exists {
			allowed = allowed && previous
		}
		decisions[key] = allowed
	}
	return func(method, route string) bool {
		if route == "" {
			return false
		}
		route = appAccessRoutePath(route)
		prefix, _, wildcard := strings.Cut(route, "*")
		if !wildcard {
			return decisions[method+" "+route]
		}
		matched := false
		for key, allowed := range decisions {
			if strings.HasPrefix(key, method+" "+prefix) {
				matched = true
				if !allowed {
					return false
				}
			}
		}
		return matched
	}
}

func appAccessRoutePath(path string) string {
	parts := strings.Split(path, "/")
	for index, part := range parts {
		if strings.HasPrefix(part, ":") || strings.HasPrefix(part, "{") {
			parts[index] = ":"
		}
	}
	return strings.Join(parts, "/")
}

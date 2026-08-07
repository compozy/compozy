package spec

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var operationPathParameterPattern = regexp.MustCompile(`\{([^{}]+)\}`)

func validateOperationRegistry(operations []OperationSpec) error {
	if len(operations) == 0 {
		return errors.New("operation registry is empty")
	}
	operationIDs := make(map[string]struct{}, len(operations))
	routeKeys := make(map[string]struct{}, len(operations))
	for index, operation := range operations {
		if err := validateOperationRegistryEntry(operation, operationIDs, routeKeys); err != nil {
			return fmt.Errorf("operation registry entry %d: %w", index, err)
		}
	}
	return nil
}

func validateOperationRegistryEntry(
	operation OperationSpec,
	operationIDs map[string]struct{},
	routeKeys map[string]struct{},
) error {
	if !supportedOperationMethod(operation.Method) {
		return fmt.Errorf("unsupported method %q", operation.Method)
	}
	if !strings.HasPrefix(operation.Path, "/api/") {
		return fmt.Errorf("path %q must be an absolute /api path", operation.Path)
	}
	operationID := strings.TrimSpace(operation.OperationID)
	if operationID == "" {
		return fmt.Errorf("%s %s has an empty operation ID", operation.Method, operation.Path)
	}
	if _, exists := operationIDs[operationID]; exists {
		return fmt.Errorf("duplicate operation ID %q", operationID)
	}
	operationIDs[operationID] = struct{}{}

	routeKey := operation.Method + " " + operation.Path
	if _, exists := routeKeys[routeKey]; exists {
		return fmt.Errorf("duplicate route %s", routeKey)
	}
	routeKeys[routeKey] = struct{}{}

	if err := validateOperationTransports(operation); err != nil {
		return err
	}
	if err := validateOperationPathParameters(operation); err != nil {
		return err
	}
	if len(operation.Responses) == 0 {
		return fmt.Errorf("%s has no responses", routeKey)
	}
	return nil
}

func supportedOperationMethod(method string) bool {
	switch method {
	case httpMethodDelete, httpMethodGet, httpMethodPatch, httpMethodPost, httpMethodPut:
		return true
	default:
		return false
	}
}

func validateOperationTransports(operation OperationSpec) error {
	if len(operation.Transports) == 0 {
		return fmt.Errorf("%s %s has no transports", operation.Method, operation.Path)
	}
	seen := make(map[Transport]struct{}, len(operation.Transports))
	for _, transport := range operation.Transports {
		switch transport {
		case TransportHTTP, TransportUDS:
		default:
			return fmt.Errorf("%s %s has unknown transport %q", operation.Method, operation.Path, transport)
		}
		if _, exists := seen[transport]; exists {
			return fmt.Errorf("%s %s repeats transport %q", operation.Method, operation.Path, transport)
		}
		seen[transport] = struct{}{}
	}
	return nil
}

func validateOperationPathParameters(operation OperationSpec) error {
	templateNames, err := operationPathParameterNames(operation.Path)
	if err != nil {
		return err
	}
	declared := make(map[string]int)
	for _, parameter := range operation.Parameters {
		if parameter.In != specParameterInPath {
			continue
		}
		if !parameter.Required {
			return fmt.Errorf(
				"%s %s path parameter %q must be required",
				operation.Method,
				operation.Path,
				parameter.Name,
			)
		}
		declared[parameter.Name]++
	}
	for name, count := range templateNames {
		if count != 1 {
			return fmt.Errorf("%s %s repeats path template parameter %q", operation.Method, operation.Path, name)
		}
		if declared[name] != 1 {
			return fmt.Errorf(
				"%s %s path template parameter %q requires exactly one declaration",
				operation.Method,
				operation.Path,
				name,
			)
		}
	}
	for name, count := range declared {
		if count != 1 {
			return fmt.Errorf("%s %s repeats path parameter declaration %q", operation.Method, operation.Path, name)
		}
		if templateNames[name] != 1 {
			return fmt.Errorf(
				"%s %s declares path parameter %q absent from the template",
				operation.Method,
				operation.Path,
				name,
			)
		}
	}
	return nil
}

func operationPathParameterNames(path string) (map[string]int, error) {
	names := make(map[string]int)
	for _, match := range operationPathParameterPattern.FindAllStringSubmatch(path, -1) {
		name := strings.TrimSpace(match[1])
		if name == "" {
			return nil, fmt.Errorf("path %q contains an empty template parameter", path)
		}
		names[name]++
	}
	withoutParameters := operationPathParameterPattern.ReplaceAllString(path, "")
	if strings.ContainsAny(withoutParameters, "{}") {
		return nil, fmt.Errorf("path %q contains malformed template parameters", path)
	}
	return names, nil
}

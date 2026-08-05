package hostapi

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"slices"
	"strings"
)

//go:embed catalog.json
var catalogJSON []byte

var familyOrder = []string{"core", "automation", "tasks", "network", "resources", "bridges", "clarify"}

type namedType struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type method struct {
	Constant       string    `json:"constant"`
	Wire           string    `json:"wire"`
	Family         string    `json:"family"`
	Params         namedType `json:"params"`
	Result         namedType `json:"result"`
	OptionalParams bool      `json:"optional_params,omitempty"`
}

func loadCatalog() ([]method, error) {
	var methods []method
	if err := json.Unmarshal(catalogJSON, &methods); err != nil {
		return nil, fmt.Errorf("decode Host API codegen catalog: %w", err)
	}
	if err := validateCatalog(methods); err != nil {
		return nil, err
	}
	return methods, nil
}

func validateCatalog(methods []method) error {
	if len(methods) == 0 {
		return errors.New("host API codegen catalog is empty")
	}
	constants := make(map[string]struct{}, len(methods))
	wires := make(map[string]struct{}, len(methods))
	lastFamily := 0
	for index, entry := range methods {
		if err := validateMethod(entry); err != nil {
			return fmt.Errorf("host API codegen catalog entry %d: %w", index, err)
		}
		if _, exists := constants[entry.Constant]; exists {
			return fmt.Errorf("host API codegen catalog constant %q is duplicated", entry.Constant)
		}
		constants[entry.Constant] = struct{}{}
		if _, exists := wires[entry.Wire]; exists {
			return fmt.Errorf("host API codegen catalog wire method %q is duplicated", entry.Wire)
		}
		wires[entry.Wire] = struct{}{}
		familyIndex := slices.Index(familyOrder, entry.Family)
		if familyIndex < lastFamily {
			return fmt.Errorf("host API codegen catalog family %q is out of order", entry.Family)
		}
		lastFamily = familyIndex
	}
	return nil
}

func validateMethod(entry method) error {
	if !token.IsIdentifier(entry.Constant) || !strings.HasPrefix(entry.Constant, "HostAPIMethod") {
		return fmt.Errorf("constant %q is not a Host API identifier", entry.Constant)
	}
	if strings.TrimSpace(entry.Wire) == "" || strings.ContainsAny(entry.Wire, " \t\r\n") {
		return fmt.Errorf("wire method %q is invalid", entry.Wire)
	}
	if !slices.Contains(familyOrder, entry.Family) {
		return fmt.Errorf("family %q is invalid", entry.Family)
	}
	if err := validateNamedType("params", entry.Params); err != nil {
		return err
	}
	return validateNamedType("result", entry.Result)
}

func validateNamedType(label string, named namedType) error {
	if strings.TrimSpace(named.Name) == "" {
		return fmt.Errorf("%s name is required", label)
	}
	if _, err := parser.ParseExpr(named.Value); err != nil {
		return fmt.Errorf("%s Go value %q: %w", label, named.Value, err)
	}
	return nil
}

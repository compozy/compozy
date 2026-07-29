package extensionpkg

import (
	"fmt"
	"slices"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

// ConsentArea is one operator-facing permission summary derived from Host API methods.
type ConsentArea struct {
	Area   string `json:"area"`
	Access string `json:"access"`
}

// DeriveConsentAreas derives a deterministic consent summary from the closed permission set.
func DeriveConsentAreas(methods []string) ([]ConsentArea, error) {
	areas := make(map[ConsentArea]struct{})
	for idx, method := range methods {
		trimmed := strings.TrimSpace(method)
		contract, ok := extensioncontract.PermissionContractForMethod(trimmed)
		if !ok || contract.Area == "" || contract.Access == "" {
			return nil, &ManifestValidationError{
				Field:   fmt.Sprintf("permissions.requires[%d]", idx),
				Value:   method,
				Message: unknownHostAPIPermissionMessage,
			}
		}
		areas[ConsentArea{Area: contract.Area, Access: contract.Access}] = struct{}{}
	}

	result := make([]ConsentArea, 0, len(areas))
	for area := range areas {
		result = append(result, area)
	}
	slices.SortFunc(result, func(left, right ConsentArea) int {
		if byArea := strings.Compare(left.Area, right.Area); byArea != 0 {
			return byArea
		}
		return strings.Compare(left.Access, right.Access)
	})
	return result, nil
}

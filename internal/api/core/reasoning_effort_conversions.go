package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func reasoningEffortsFromStrings(values []string) []contract.ReasoningEffort {
	if values == nil {
		return nil
	}
	efforts := make([]contract.ReasoningEffort, 0, len(values))
	for _, value := range values {
		efforts = append(efforts, contract.ReasoningEffort(strings.TrimSpace(value)))
	}
	return efforts
}

func reasoningEffortsToStrings(values []contract.ReasoningEffort) []string {
	if values == nil {
		return nil
	}
	efforts := make([]string, 0, len(values))
	for _, value := range values {
		efforts = append(efforts, strings.TrimSpace(string(value)))
	}
	return efforts
}

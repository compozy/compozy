package globaldb

import automation "github.com/compozy/agh/internal/automation/model"

type automationCatalogCandidate struct {
	ID     string
	Source automation.JobSource
	Name   string
}

func appendAutomationCatalogPredicate(where string, predicate string) string {
	if where == "" {
		return " WHERE " + predicate
	}
	return where + " AND " + predicate
}

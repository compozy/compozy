package globaldb

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/compozy/compozy/internal/loop/gate"
)

type goalVerdictDiagnostics struct {
	blockingJSON []byte
	criteriaJSON []byte
	warningsJSON []byte
}

func marshalGoalVerdictDiagnostics(verdict *gate.Verdict) (goalVerdictDiagnostics, error) {
	blocking := []gate.BlockingIssue{}
	criteria := []gate.CriterionResult{}
	warnings := []gate.DiagnosticWarning{}
	if verdict != nil {
		blocking = append(blocking, verdict.BlockingIssues...)
		criteria = append(criteria, verdict.Criteria...)
		warnings = append(warnings, verdict.Warnings...)
	}

	blockingJSON, err := marshalGoalDiagnosticField("blocking issues", blocking)
	if err != nil {
		return goalVerdictDiagnostics{}, err
	}
	criteriaJSON, err := marshalGoalDiagnosticField("criteria", criteria)
	if err != nil {
		return goalVerdictDiagnostics{}, err
	}
	warningsJSON, err := marshalGoalDiagnosticField("warnings", warnings)
	if err != nil {
		return goalVerdictDiagnostics{}, err
	}
	return goalVerdictDiagnostics{
		blockingJSON: blockingJSON,
		criteriaJSON: criteriaJSON,
		warningsJSON: warningsJSON,
	}, nil
}

func marshalGoalDiagnosticField(name string, value any) ([]byte, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("store: encode Goal verdict %s: %w", name, err)
	}
	return data, nil
}

func goalVerdictDiagnosticsEqual(left, right goalVerdictDiagnostics) bool {
	return bytes.Equal(left.blockingJSON, right.blockingJSON) &&
		bytes.Equal(left.criteriaJSON, right.criteriaJSON) &&
		bytes.Equal(left.warningsJSON, right.warningsJSON)
}

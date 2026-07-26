package daemon

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/session"
)

func buildSessionGoalDefinition(
	contract session.GoalObjectiveContract,
	agent string,
	judgeModel string,
	maxTurns int,
) dsl.Definition {
	judgeModel = strings.TrimSpace(judgeModel)
	var modelDefaults *dsl.ModelDefaults
	if judgeModel != "" {
		modelDefaults = &dsl.ModelDefaults{Judge: judgeModel}
	}
	criterion := dsl.GateCriterion{
		ID:     "objective_satisfied",
		Type:   dsl.CriterionAgentJudge,
		Agent:  strings.TrimSpace(agent),
		Rubric: sessionGoalRubric(contract),
	}
	outputSchema := dsl.Schema{
		"type": "object",
		"properties": map[string]any{
			daemonStatusField: map[string]any{
				"type": "string",
				"enum": []string{goalSnapshotStatusComplete, goalSnapshotStatusBlocked},
			},
		},
		"required":             []string{daemonStatusField},
		"additionalProperties": true,
	}
	return dsl.Definition{
		APIVersion:  dsl.APIVersion,
		Kind:        dsl.KindLoop,
		Meta:        dsl.Meta{Name: loop.InlineGoalLoopName, Version: 1},
		Concurrency: dsl.ConcurrencyAllow,
		Inputs:      map[string]dsl.Input{},
		Contract: dsl.Contract{
			Goal:             contract.Objective,
			DefinitionOfDone: "The objective is satisfied against the stated verification and constraints.",
			ModelDefaults:    modelDefaults,
			Constraints:      append([]string(nil), contract.Constraints...),
			Verification:     []dsl.GateCriterion{},
			TerminalStates: []dsl.TerminalState{
				dsl.TerminalDone,
				dsl.TerminalBlocked,
				dsl.TerminalFailed,
				dsl.TerminalExhausted,
				dsl.TerminalStalled,
			},
			IterationCap: 1,
			NoProgress: dsl.NoProgress{
				Window:     1,
				HashFields: []string{"nodes.goal.output.status"},
			},
			Budget: dsl.Budget{OnExceeded: dsl.BudgetExceededHalt},
		},
		Graph: dsl.Graph{
			Nodes: []dsl.Node{{
				ID:      loopManagedGoalKind,
				Class:   dsl.NodeClassAction,
				Kind:    string(dsl.ActionGoal),
				Session: &dsl.SessionSpec{Mode: dsl.SessionModeContinuous},
				Params: dsl.NodeParams{
					daemonAgentField: agent, "objective": contract.Objective,
					"judge": []dsl.GateCriterion{criterion}, "max_turns": maxTurns,
					"on_exhausted": dsl.GoalOnExhaustedHalt, "output_schema": outputSchema,
				},
			}},
			Edges: []dsl.Edge{},
		},
		DefinitionExtensionState: &dsl.DefinitionExtensionState{Start: []dsl.StartBinding{}},
	}
}

func sessionGoalRubric(contract session.GoalObjectiveContract) string {
	sections := []string{"Objective:\n" + strings.TrimSpace(contract.Objective)}
	if len(contract.Verify) > 0 {
		sections = append(sections, "Verification:\n"+goalRubricList(contract.Verify))
	}
	if len(contract.Constraints) > 0 {
		sections = append(sections, "Constraints:\n"+goalRubricList(contract.Constraints))
	}
	return strings.Join(sections, "\n\n")
}

func goalRubricList(values []string) string {
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, fmt.Sprintf("- %s", strings.TrimSpace(value)))
	}
	return strings.Join(lines, "\n")
}

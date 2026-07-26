package loop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/task"
)

type goalControlCandidate struct {
	key      generationOutputKey
	terminal task.CoordinatorTerminal
}

func selectGoalControlTerminal(candidates []goalControlCandidate) *task.CoordinatorTerminal {
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].key.nodeID == candidates[j].key.nodeID {
			return candidates[i].key.itemIndex < candidates[j].key.itemIndex
		}
		return candidates[i].key.nodeID < candidates[j].key.nodeID
	})
	selected := candidates[0].terminal
	return &selected
}

func resolveGoalActionControl(
	parent Run,
	output GenerationOutput,
	run task.Run,
) (GenerationOutput, *task.CoordinatorTerminal, error) {
	wasAwaitingGoal := output.Status == generationOutputAwaitingGoal
	control, err := DecodeActionControlResult(run.Result)
	if err != nil {
		return GenerationOutput{}, nil, err
	}
	if control.Disposition == ActionDispositionNeedsApproval {
		expectedGate := syntheticGoalGateID(output, control.Cause)
		if control.GateID != expectedGate {
			return GenerationOutput{}, nil, invalidActionControlError("gate_id", string(control.GateID))
		}
	}

	terminal := goalControlTerminal(control)
	switch control.Disposition {
	case ActionDispositionSucceeded:
		output.Status = generationOutputSucceeded
		output.OutputRef = ""
		return output, nil, nil
	case ActionDispositionBlocked, ActionDispositionExhausted:
		output.Status = generationOutputFailed
		output.OutputRef = string(control.Cause)
	case ActionDispositionPaused, ActionDispositionNeedsApproval:
		output.Status = generationOutputAwaitingGoal
		output.OutputRef = ""
	}
	if wasAwaitingGoal && goalControlAlreadySettled(parent.Status, control.Disposition) {
		return output, nil, nil
	}
	return output, &terminal, nil
}

// DecodeActionControlResult validates one persisted neutral task completion for Loop ownership.
func DecodeActionControlResult(raw json.RawMessage) (ActionControl, error) {
	var envelope task.CoordinatorControlResult
	if err := decodeStrictJSON(raw, &envelope); err != nil {
		return ActionControl{}, invalidActionControlError("coordinator_control", err.Error())
	}
	if err := envelope.Validate("coordinator_control"); err != nil {
		return ActionControl{}, invalidActionControlError("coordinator_control", err.Error())
	}
	if envelope.Kind != coordinatorControlKindLoopAction {
		return ActionControl{}, invalidActionControlError("kind", envelope.Kind)
	}
	var control ActionControl
	if err := decodeStrictJSON(envelope.Payload, &control); err != nil {
		return ActionControl{}, invalidActionControlError("payload", err.Error())
	}
	control.GoalStatus = strings.TrimSpace(control.GoalStatus)
	control.CheckpointID = strings.TrimSpace(control.CheckpointID)
	control.GateID = dsl.NodeID(strings.TrimSpace(string(control.GateID)))
	control.Cause = ReasonCode(strings.TrimSpace(string(control.Cause)))
	if err := control.Validate(); err != nil {
		return ActionControl{}, err
	}
	return control, nil
}

func decodeStrictJSON(raw json.RawMessage, target any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return fmt.Errorf("payload is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("payload contains multiple JSON values")
		}
		return fmt.Errorf("decode trailing payload: %w", err)
	}
	return nil
}

func syntheticGoalGateID(output GenerationOutput, cause ReasonCode) dsl.NodeID {
	return SyntheticGoalGateID(dsl.NodeID(output.NodeID), output.Generation, output.ItemIndex, cause)
}

// SyntheticGoalGateID returns the exact Goal approval gate correlation identity.
func SyntheticGoalGateID(nodeID dsl.NodeID, generation int, itemIndex int, cause ReasonCode) dsl.NodeID {
	return dsl.NodeID(fmt.Sprintf(
		"goal:%s:%d:%d:%s",
		strings.TrimSpace(string(nodeID)),
		generation,
		itemIndex,
		strings.TrimSpace(string(cause)),
	))
}

func goalControlTerminal(control ActionControl) task.CoordinatorTerminal {
	terminal := task.CoordinatorTerminal{
		Cause:      string(control.Cause),
		ReasonCode: string(control.Cause),
	}
	switch control.Disposition {
	case ActionDispositionBlocked:
		terminal.Status = string(StatusBlocked)
	case ActionDispositionExhausted:
		terminal.Status = string(StatusExhausted)
	case ActionDispositionPaused:
		terminal.Status = string(StatusPaused)
	case ActionDispositionNeedsApproval:
		terminal.Status = string(StatusNeedsApproval)
		terminal.GateID = string(control.GateID)
	}
	return terminal
}

func goalControlAlreadySettled(status Status, disposition ActionDisposition) bool {
	switch disposition {
	case ActionDispositionPaused:
		return status == StatusPaused
	case ActionDispositionNeedsApproval:
		return status == StatusNeedsApproval
	default:
		return false
	}
}

package globaldb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/goal"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

// AdvanceBindingCreationFailure atomically consumes one proven-no-create Goal attempt.
func (g *GoalRepo) AdvanceBindingCreationFailure(
	ctx context.Context,
	req goal.AdvanceBindingCreationFailureRequest,
) error {
	if err := g.checkReady(ctx, "advance Goal binding creation failure"); err != nil {
		return err
	}
	if err := validateAdvanceBindingCreationFailure(req); err != nil {
		return err
	}
	if req.FailedAt.IsZero() {
		req.FailedAt = g.now().UTC()
	}
	if req.SuccessorCreatedAt.IsZero() {
		req.SuccessorCreatedAt = req.FailedAt
	}
	return g.withTaskImmediateTransaction(ctx, "advance Goal binding creation failure", func(
		exec taskSQLExecutor,
	) error {
		if err := validateBindingRunWorkspace(ctx, exec, req.Key); err != nil {
			return err
		}
		binding, err := getSessionBindingAttemptWithExecutor(ctx, exec, req.Key, req.ExpectedBindingEpoch)
		if err != nil {
			return err
		}
		if !bindingMatchesFailedRetryRequest(binding, req) {
			return goalControlStaleError("binding creation failure source identity changed")
		}
		if binding.State == goal.BindingStateFailed {
			return validateCommittedBindingCreationFailure(ctx, exec, req, binding)
		}
		if binding.State != goal.BindingStateCreating {
			return goalControlStaleError("binding creation failure no longer owns a creating attempt")
		}
		if err := persistGoalBindingRetryWitness(ctx, exec, req); err != nil {
			return err
		}
		if err := failGoalBindingCreationAttempt(ctx, exec, req); err != nil {
			return err
		}
		if req.PrepareSuccessor {
			if err := insertGoalBindingCreationSuccessor(ctx, exec, req); err != nil {
				return err
			}
		}
		return advanceGoalCheckpointBindingAttempt(ctx, exec, req, binding)
	})
}

type goalBindingRetryWitness struct {
	WorkspaceID                    string `json:"workspace_id"`
	LoopRunID                      string `json:"loop_run_id"`
	Handle                         string `json:"handle"`
	CheckpointGeneration           int    `json:"checkpoint_generation"`
	CheckpointNodeID               string `json:"checkpoint_node_id"`
	CheckpointItemIndex            int    `json:"checkpoint_item_index"`
	ExpectedControlEpoch           int64  `json:"expected_control_epoch"`
	ExpectedCheckpointPhase        string `json:"expected_checkpoint_phase"`
	ExpectedTaskRunID              string `json:"expected_task_run_id"`
	ExpectedQueueEntryID           string `json:"expected_queue_entry_id"`
	ExpectedPromptID               string `json:"expected_prompt_id"`
	ExpectedCheckpointBindingEpoch int64  `json:"expected_checkpoint_binding_epoch"`
	ExpectedCheckpointSessionID    string `json:"expected_checkpoint_session_id"`
	ExpectedCheckpointHandle       string `json:"expected_checkpoint_handle"`
	ExpectedPromptAttempt          int    `json:"expected_prompt_attempt"`
	ExpectedBindingEpoch           int64  `json:"expected_binding_epoch"`
	ExpectedBindingAttemptID       string `json:"expected_binding_attempt_id"`
	ExpectedBindingSessionID       string `json:"expected_binding_session_id"`
	ExpectedBindingProfileRef      string `json:"expected_binding_profile_ref"`
	ExpectedBindingPolicyDigest    string `json:"expected_binding_policy_digest"`
	ExpectedBindingCreationDigest  string `json:"expected_binding_creation_digest"`
	FailureCode                    string `json:"failure_code"`
	PrepareSuccessor               bool   `json:"prepare_successor"`
	SuccessorBindingEpoch          int64  `json:"successor_binding_epoch"`
	SuccessorBindingAttemptID      string `json:"successor_binding_attempt_id"`
	SuccessorSessionID             string `json:"successor_session_id"`
	CreationProfileRef             string `json:"creation_profile_ref"`
	PolicySpecDigest               string `json:"policy_spec_digest"`
	SuccessorCreationDigest        string `json:"successor_creation_digest"`
}

func bindingRetryRequestDigest(req goal.AdvanceBindingCreationFailureRequest) (string, error) {
	witness := goalBindingRetryWitness{
		WorkspaceID:                    strings.TrimSpace(string(req.Key.WorkspaceID)),
		LoopRunID:                      strings.TrimSpace(string(req.Key.LoopRunID)),
		Handle:                         strings.TrimSpace(req.Key.Handle),
		CheckpointGeneration:           req.CheckpointKey.Generation,
		CheckpointNodeID:               strings.TrimSpace(string(req.CheckpointKey.NodeID)),
		CheckpointItemIndex:            req.CheckpointKey.ItemIndex,
		ExpectedControlEpoch:           req.ExpectedControlEpoch,
		ExpectedCheckpointPhase:        strings.TrimSpace(req.ExpectedCheckpointPhase),
		ExpectedTaskRunID:              strings.TrimSpace(req.ExpectedTaskRunID),
		ExpectedQueueEntryID:           strings.TrimSpace(req.ExpectedQueueEntryID),
		ExpectedPromptID:               strings.TrimSpace(req.ExpectedPromptID),
		ExpectedCheckpointBindingEpoch: req.ExpectedCheckpointBindingEpoch,
		ExpectedCheckpointSessionID:    strings.TrimSpace(req.ExpectedCheckpointSessionID),
		ExpectedCheckpointHandle:       strings.TrimSpace(req.ExpectedCheckpointHandle),
		ExpectedPromptAttempt:          req.ExpectedPromptAttempt,
		ExpectedBindingEpoch:           req.ExpectedBindingEpoch,
		ExpectedBindingAttemptID:       strings.TrimSpace(req.ExpectedBindingAttemptID),
		ExpectedBindingSessionID:       strings.TrimSpace(req.ExpectedBindingSessionID),
		ExpectedBindingProfileRef:      strings.TrimSpace(req.ExpectedBindingProfileRef),
		ExpectedBindingPolicyDigest:    strings.TrimSpace(req.ExpectedBindingPolicyDigest),
		ExpectedBindingCreationDigest:  strings.TrimSpace(req.ExpectedBindingCreationDigest),
		FailureCode:                    strings.TrimSpace(req.FailureCode),
		PrepareSuccessor:               req.PrepareSuccessor,
		SuccessorBindingEpoch:          req.SuccessorBindingEpoch,
		SuccessorBindingAttemptID:      strings.TrimSpace(req.SuccessorBindingAttemptID),
		SuccessorSessionID:             strings.TrimSpace(req.SuccessorSessionID),
		CreationProfileRef:             strings.TrimSpace(req.CreationProfileRef),
		PolicySpecDigest:               strings.TrimSpace(req.PolicySpecDigest),
		SuccessorCreationDigest:        strings.TrimSpace(req.SuccessorCreationDigest),
	}
	payload, err := json.Marshal(witness)
	if err != nil {
		return "", fmt.Errorf("store: encode Goal binding retry witness: %w", err)
	}
	return fmt.Sprintf("%x", sha256.Sum256(payload)), nil
}

func persistGoalBindingRetryWitness(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AdvanceBindingCreationFailureRequest,
) error {
	digest, err := bindingRetryRequestDigest(req)
	if err != nil {
		return err
	}
	if err := sqlcgen.New(exec).InsertGoalBindingRetryWitness(ctx, sqlcgen.InsertGoalBindingRetryWitnessParams{
		LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle, FailedBindingEpoch: req.ExpectedBindingEpoch,
		RequestDigest: digest, CreatedAt: store.FormatTimestamp(req.FailedAt),
	}); err != nil {
		return fmt.Errorf("store: persist Goal binding retry witness: %w", err)
	}
	return nil
}

func validateGoalBindingRetryWitness(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AdvanceBindingCreationFailureRequest,
) error {
	want, err := bindingRetryRequestDigest(req)
	if err != nil {
		return err
	}
	stored, err := sqlcgen.New(exec).
		GetGoalBindingRetryWitnessDigest(ctx, sqlcgen.GetGoalBindingRetryWitnessDigestParams{
			LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle, FailedBindingEpoch: req.ExpectedBindingEpoch,
		})
	if errors.Is(err, sql.ErrNoRows) {
		return goalControlStaleError("binding creation retry witness is missing")
	}
	if err != nil {
		return fmt.Errorf("store: load Goal binding retry witness: %w", err)
	}
	if stored != want {
		return goalControlStaleError("binding creation retry witness changed")
	}
	return nil
}

func validateAdvanceBindingCreationFailure(req goal.AdvanceBindingCreationFailureRequest) error {
	if err := req.Key.Validate(); err != nil {
		return err
	}
	if err := req.CheckpointKey.Validate(); err != nil {
		return err
	}
	if err := validateBindingFailureCheckpointOwner(req); err != nil {
		return err
	}
	return validateBindingFailureSuccessor(req)
}

func validateBindingFailureCheckpointOwner(req goal.AdvanceBindingCreationFailureRequest) error {
	checkpointBindingEmpty := strings.TrimSpace(req.ExpectedCheckpointSessionID) == "" &&
		strings.TrimSpace(req.ExpectedCheckpointHandle) == ""
	checkpointBindingComplete := strings.TrimSpace(req.ExpectedCheckpointSessionID) != "" &&
		strings.TrimSpace(req.ExpectedCheckpointHandle) != ""
	if req.Key.WorkspaceID != req.CheckpointKey.WorkspaceID || req.Key.LoopRunID != req.CheckpointKey.LoopRunID ||
		req.ExpectedControlEpoch < 1 || req.ExpectedPromptAttempt < 0 || req.ExpectedBindingEpoch < 1 ||
		!goalCheckpointPhaseValid(strings.TrimSpace(req.ExpectedCheckpointPhase)) ||
		strings.TrimSpace(req.ExpectedTaskRunID) == "" || req.ExpectedCheckpointBindingEpoch < 0 ||
		(req.ExpectedCheckpointBindingEpoch == 0 && !checkpointBindingEmpty) ||
		(req.ExpectedCheckpointBindingEpoch > 0 && !checkpointBindingComplete) ||
		!failedBindingRetryIdentityComplete(req) || strings.TrimSpace(req.FailureCode) == "" {
		return fmt.Errorf("%w: Goal binding retry identity is incomplete", looppkg.ErrValidation)
	}
	return nil
}

func failedBindingRetryIdentityComplete(req goal.AdvanceBindingCreationFailureRequest) bool {
	return strings.TrimSpace(req.ExpectedBindingAttemptID) != "" &&
		strings.TrimSpace(req.ExpectedBindingSessionID) != "" &&
		strings.TrimSpace(req.ExpectedBindingProfileRef) != "" &&
		strings.TrimSpace(req.ExpectedBindingPolicyDigest) != "" &&
		strings.TrimSpace(req.ExpectedBindingCreationDigest) != ""
}

func bindingMatchesFailedRetryRequest(
	binding goal.SessionBinding,
	req goal.AdvanceBindingCreationFailureRequest,
) bool {
	return binding.BindingEpoch == req.ExpectedBindingEpoch &&
		binding.BindingAttemptID == strings.TrimSpace(req.ExpectedBindingAttemptID) &&
		binding.SessionID == strings.TrimSpace(req.ExpectedBindingSessionID) &&
		binding.CreationProfileRef == strings.TrimSpace(req.ExpectedBindingProfileRef) &&
		binding.PolicySpecDigest == strings.TrimSpace(req.ExpectedBindingPolicyDigest) &&
		binding.CreationDigest == strings.TrimSpace(req.ExpectedBindingCreationDigest)
}

func validateBindingFailureSuccessor(req goal.AdvanceBindingCreationFailureRequest) error {
	if !req.PrepareSuccessor {
		if req.SuccessorBindingEpoch != 0 || strings.TrimSpace(req.SuccessorBindingAttemptID) != "" ||
			strings.TrimSpace(req.SuccessorSessionID) != "" || strings.TrimSpace(req.SuccessorCreationDigest) != "" {
			return fmt.Errorf("%w: exhausted Goal binding retry cannot prepare a successor", looppkg.ErrValidation)
		}
		return nil
	}
	if req.SuccessorBindingEpoch != req.ExpectedBindingEpoch+1 ||
		strings.TrimSpace(req.SuccessorBindingAttemptID) == "" || strings.TrimSpace(req.SuccessorSessionID) == "" ||
		strings.TrimSpace(req.CreationProfileRef) == "" || strings.TrimSpace(req.PolicySpecDigest) == "" ||
		strings.TrimSpace(req.SuccessorCreationDigest) == "" {
		return fmt.Errorf("%w: Goal binding retry successor identity is incomplete", looppkg.ErrValidation)
	}
	return nil
}

func failGoalBindingCreationAttempt(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AdvanceBindingCreationFailureRequest,
) error {
	affected, err := sqlcgen.New(exec).FailGoalBindingCreationAttempt(ctx, sqlcgen.FailGoalBindingCreationAttemptParams{
		FailureCode: goalNullableString(req.FailureCode), FailedAt: store.FormatTimestamp(req.FailedAt),
		LoopRunID: string(req.Key.LoopRunID), Handle: req.Key.Handle, BindingEpoch: req.ExpectedBindingEpoch,
	})
	if err != nil {
		return fmt.Errorf("store: fail Goal-owned binding creation attempt: %w", err)
	}
	return requireGoalAffectedCount(affected, "fail Goal-owned binding creation attempt")
}

func insertGoalBindingCreationSuccessor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AdvanceBindingCreationFailureRequest,
) error {
	err := sqlcgen.New(exec).InsertGoalSessionBindingAttempt(ctx, sqlcgen.InsertGoalSessionBindingAttemptParams{
		LoopRunID:    string(req.Key.LoopRunID),
		Handle:       req.Key.Handle,
		BindingEpoch: req.SuccessorBindingEpoch,
		BindingAttemptID: strings.TrimSpace(
			req.SuccessorBindingAttemptID,
		),
		SessionID:          strings.TrimSpace(req.SuccessorSessionID),
		WorkspaceID:        string(req.Key.WorkspaceID),
		CreationProfileRef: strings.TrimSpace(req.CreationProfileRef),
		PolicySpecDigest: strings.TrimSpace(
			req.PolicySpecDigest,
		),
		CreationDigest: strings.TrimSpace(req.SuccessorCreationDigest),
		CreatedAt:      store.FormatTimestamp(req.SuccessorCreatedAt),
	})
	if err != nil {
		return fmt.Errorf("store: prepare Goal-owned binding retry successor: %w", err)
	}
	return nil
}

func advanceGoalCheckpointBindingAttempt(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AdvanceBindingCreationFailureRequest,
	failed goal.SessionBinding,
) error {
	bindingEpoch := failed.BindingEpoch
	sessionID := failed.SessionID
	if req.PrepareSuccessor {
		bindingEpoch = req.SuccessorBindingEpoch
		sessionID = strings.TrimSpace(req.SuccessorSessionID)
	}
	affected, err := sqlcgen.New(exec).
		AdvanceGoalCheckpointBindingAttempt(ctx, sqlcgen.AdvanceGoalCheckpointBindingAttemptParams{
			BindingHandle: goalNullableString(req.Key.Handle), BindingEpoch: goalNullableInt64(&bindingEpoch),
			SessionID: goalNullableString(sessionID), UpdatedAt: store.FormatTimestamp(req.FailedAt),
			LoopRunID:  string(req.CheckpointKey.LoopRunID),
			Generation: int64(req.CheckpointKey.Generation), NodeID: string(req.CheckpointKey.NodeID),
			ItemIndex: int64(req.CheckpointKey.ItemIndex), ControlEpoch: req.ExpectedControlEpoch,
			Phase: strings.TrimSpace(req.ExpectedCheckpointPhase), PromptAttempt: int64(req.ExpectedPromptAttempt),
			TaskRunID:             goalRequiredSQLString(req.ExpectedTaskRunID),
			QueueEntryID:          goalRequiredSQLString(req.ExpectedQueueEntryID),
			PromptID:              goalRequiredSQLString(req.ExpectedPromptID),
			ExpectedBindingEpoch:  goalNullableInt64(&req.ExpectedCheckpointBindingEpoch),
			ExpectedSessionID:     goalRequiredSQLString(req.ExpectedCheckpointSessionID),
			ExpectedBindingHandle: goalRequiredSQLString(req.ExpectedCheckpointHandle),
		})
	if err != nil {
		return fmt.Errorf("store: advance Goal checkpoint binding attempt: %w", err)
	}
	return requireGoalAffectedCount(affected, "advance Goal checkpoint binding attempt")
}

func validateCommittedBindingCreationFailure(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AdvanceBindingCreationFailureRequest,
	binding goal.SessionBinding,
) error {
	if err := validateGoalBindingRetryWitness(ctx, exec, req); err != nil {
		return err
	}
	if binding.FailureCode != strings.TrimSpace(req.FailureCode) {
		return goalControlStaleError("binding creation failure code changed")
	}
	checkpoint, err := loadGoalCheckpointWithExecutor(ctx, exec, req.CheckpointKey)
	if err != nil {
		return err
	}
	wantEpoch, err := validateCommittedBindingSuccessor(ctx, exec, req)
	if err != nil {
		return err
	}
	return validateCommittedBindingCheckpoint(req, binding, checkpoint, wantEpoch)
}

func validateCommittedBindingSuccessor(
	ctx context.Context,
	exec taskSQLExecutor,
	req goal.AdvanceBindingCreationFailureRequest,
) (int64, error) {
	if !req.PrepareSuccessor {
		return req.ExpectedBindingEpoch, nil
	}
	successor, err := getSessionBindingAttemptWithExecutor(ctx, exec, req.Key, req.SuccessorBindingEpoch)
	if err != nil {
		return 0, err
	}
	if successor.BindingAttemptID != strings.TrimSpace(req.SuccessorBindingAttemptID) ||
		successor.SessionID != strings.TrimSpace(req.SuccessorSessionID) ||
		successor.CreationProfileRef != strings.TrimSpace(req.CreationProfileRef) ||
		successor.PolicySpecDigest != strings.TrimSpace(req.PolicySpecDigest) ||
		successor.CreationDigest != strings.TrimSpace(req.SuccessorCreationDigest) ||
		successor.Ownership != goal.BindingOwnershipRunOwned || successor.State != goal.BindingStateCreating {
		return 0, goalControlStaleError("binding creation retry successor changed")
	}
	return req.SuccessorBindingEpoch, nil
}

func validateCommittedBindingCheckpoint(
	req goal.AdvanceBindingCreationFailureRequest,
	binding goal.SessionBinding,
	checkpoint goal.Checkpoint,
	wantEpoch int64,
) error {
	if checkpoint.ControlEpoch != req.ExpectedControlEpoch ||
		checkpoint.Phase != strings.TrimSpace(req.ExpectedCheckpointPhase) ||
		checkpoint.TaskRunID != strings.TrimSpace(req.ExpectedTaskRunID) ||
		checkpoint.QueueEntryID != strings.TrimSpace(req.ExpectedQueueEntryID) ||
		checkpoint.PromptID != strings.TrimSpace(req.ExpectedPromptID) ||
		checkpoint.PromptAttempt != req.ExpectedPromptAttempt+1 || checkpoint.BindingEpoch != wantEpoch ||
		checkpoint.BindingHandle != req.Key.Handle {
		return goalControlStaleError("committed binding creation failure checkpoint changed")
	}
	wantSessionID := binding.SessionID
	if req.PrepareSuccessor {
		wantSessionID = strings.TrimSpace(req.SuccessorSessionID)
	}
	if checkpoint.SessionID != wantSessionID {
		return goalControlStaleError("committed binding creation failure session changed")
	}
	return nil
}

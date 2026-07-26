package globaldb

import (
	"database/sql"
	"encoding/json"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
)

func loopRunInsertParams(
	run looppkg.Run,
	inputsJSON []byte,
	metadataJSON []byte,
) (sqlcgen.InsertLoopRunParams, error) {
	network, err := encodeParticipationSnapshot(string(run.WorkspaceID), run.NetworkSpecSnapshot())
	if err != nil {
		return sqlcgen.InsertLoopRunParams{}, err
	}
	return sqlcgen.InsertLoopRunParams{
		ID:                      string(run.ID),
		WorkspaceID:             string(run.WorkspaceID),
		LoopName:                run.LoopName,
		Status:                  string(run.Status),
		Generation:              int64(run.Generation),
		ReattemptStrategy:       string(run.ReattemptStrategy),
		CreatedAt:               store.FormatTimestamp(run.CreatedAt),
		StartedAt:               store.FormatTimestamp(run.StartedAt),
		LastProgressAt:          run.LastProgressAt,
		DefinitionVersion:       int64(run.DefinitionVersion),
		DefinitionDigest:        run.DefinitionDigest,
		ActiveGateID:            string(run.ActiveGateID),
		ActiveHumanCriteriaJson: string(run.ActiveHumanCriteria),
		BudgetApprovalSeq:       int64(run.BudgetApprovalSeq),
		StartMetadataJson:       string(metadataJSON),
		IterationCap:            int64(run.IterationCap),
		BudgetTokens:            int64(run.BudgetTokens),
		BudgetWallSec:           int64(run.BudgetWallSec),
		BudgetOnExceeded:        string(run.BudgetOnExceeded),
		TokensUsed:              run.TokensUsed,
		ParentLoopRunID:         nullString(string(run.ParentLoopRunID)),
		PauseRequested:          int64(boolToInt(run.PauseRequested)),
		InputsJson:              string(inputsJSON),
		StartedByKind:           string(run.StartedBy.Kind),
		StartedByRef:            run.StartedBy.Ref,
		StartedOriginKind:       string(run.StartedOrigin.Kind),
		StartedOriginRef:        run.StartedOrigin.Ref,
		GoalContextNudgeRatio:   run.GoalContextNudgeRatio,
		OriginKind:              string(run.Origin.Kind),
		OriginSessionID: nullString(
			run.Origin.SessionID,
		),
		OriginCreationProfileRef: nullString(run.Origin.CreationProfileRef),
		OriginPolicySpecDigest: nullString(
			run.Origin.PolicySpecDigest,
		),
		OriginCreationDigest: nullString(run.Origin.CreationDigest),
		NetworkSpecJson:      network.JSON,
		NetworkMode:          network.Mode,
		NetworkChannel:       network.Channel,
		NetworkSource:        network.Source,
	}, nil
}

func loopRunFromGenerated(row *sqlcgen.LoopRun) (looppkg.Run, error) {
	controlRequested := sql.NullString{}
	if row.ControlRequestedAt.Valid {
		controlRequested = sql.NullString{String: store.FormatTimestamp(row.ControlRequestedAt.Time), Valid: true}
	}
	values := loopRunScanValues{
		run: looppkg.Run{
			LoopName: row.LoopName, Generation: int(row.Generation), DefinitionVersion: int(row.DefinitionVersion),
			DefinitionDigest: row.DefinitionDigest, ActiveGateID: looppkg.NodeID(row.ActiveGateID),
			BudgetApprovalSeq: int(row.BudgetApprovalSeq),
			BudgetTokens:      int(row.BudgetTokens), BudgetWallSec: int(row.BudgetWallSec), TokensUsed: row.TokensUsed,
			IterationCap: int(row.IterationCap), GoalContextNudgeRatio: row.GoalContextNudgeRatio,
		},
		runID: row.ID, workspaceID: row.WorkspaceID, status: row.Status, reattempt: row.ReattemptStrategy,
		budgetOnExceeded: row.BudgetOnExceeded, createdAtRaw: row.CreatedAt,
		startedAtRaw: row.StartedAt, lastProgressAtRaw: store.FormatTimestamp(row.LastProgressAt),
		activeHumanRaw: row.ActiveHumanCriteriaJson, startMetadataRaw: row.StartMetadataJson,
		parentID: row.ParentLoopRunID, pauseRequested: int(row.PauseRequested),
		controlActorKind: row.ControlActorKind, controlActorID: row.ControlActorID,
		controlRequested: controlRequested, inputsRaw: row.InputsJson,
		startedByKind: row.StartedByKind, startedByRef: row.StartedByRef,
		startedOriginKind: row.StartedOriginKind, startedOriginRef: row.StartedOriginRef,
		originKind: row.OriginKind, originSessionID: row.OriginSessionID,
		originProfileRef: row.OriginCreationProfileRef, originPolicy: row.OriginPolicySpecDigest,
		originCreation:  row.OriginCreationDigest,
		networkSpecJSON: row.NetworkSpecJson, networkMode: row.NetworkMode,
		networkChannel: row.NetworkChannel, networkSource: row.NetworkSource,
	}
	return values.toRun()
}

func loopRunEventFromGenerated(row sqlcgen.LoopRunEvent) looppkg.RunEvent {
	return looppkg.RunEvent{
		ID: row.ID, LoopRunID: looppkg.RunID(row.LoopRunID), WorkspaceID: looppkg.WorkspaceID(row.WorkspaceID),
		Seq: row.Seq, Kind: row.Kind, Payload: json.RawMessage(row.PayloadJson), At: row.At.UTC(),
	}
}

func generationOutputFromGenerated(row sqlcgen.ListLoopGenerationOutputsRow) looppkg.GenerationOutput {
	output := looppkg.GenerationOutput{
		Generation: int(row.Generation), NodeID: row.NodeID,
		ItemIndex: int(row.ItemIndex), Status: row.Status,
	}
	if row.OutputRef.Valid {
		output.OutputRef = row.OutputRef.String
	}
	if row.TaskRunID.Valid {
		output.TaskRunID = row.TaskRunID.String
	}
	if row.ChildLoopRunID.Valid {
		output.ChildLoopRunID = row.ChildLoopRunID.String
	}
	return output
}

func loopConfigFromGenerated(row sqlcgen.GetLoopConfigRow) looppkg.LoopConfig {
	return loopConfigScanValues{
		humanGateEnabled: int(row.HumanGateEnabled), reattempt: row.ReattemptStrategy,
		enabledChecks: row.EnabledChecksJson, iterationCap: row.IterationCap,
		budgetTokens: row.BudgetTokens, budgetWallSec: row.BudgetWallSec,
		budgetOnExceeded: row.BudgetOnExceeded, noProgressWindow: row.NoProgressWindow,
		fanOutWidth: row.FanOutWidth, gateMaxRevisions: row.GateMaxRevisions,
		modelDefaultWorker: row.ModelDefaultWorker, modelDefaultJudge: row.ModelDefaultJudge,
	}.toConfig()
}

func loopConfigPatchParams(
	insert sqlcgen.InsertLoopConfigIfMissingParams,
	patch loopConfigPatchFlags,
) sqlcgen.PatchLoopConfigParams {
	return sqlcgen.PatchLoopConfigParams{
		PatchHumanGate: int64(boolToInt(patch.HumanGate)), HumanGateEnabled: insert.HumanGateEnabled,
		PatchReattempt: loopPatchStringFlag(patch.Reattempt), ReattemptStrategy: insert.ReattemptStrategy,
		PatchEnabledChecks: map[bool]string{false: "0", true: "1"}[patch.EnabledChecks],
		EnabledChecksJson:  insert.EnabledChecksJson, PatchIterationCap: loopPatchIntFlag(patch.IterationCap),
		IterationCap: insert.IterationCap, PatchBudgetTokens: loopPatchIntFlag(patch.BudgetTokens),
		BudgetTokens: insert.BudgetTokens, PatchBudgetWallSec: loopPatchIntFlag(patch.BudgetWallSec),
		BudgetWallSec: insert.BudgetWallSec, PatchBudgetOnExceeded: loopPatchStringFlag(patch.BudgetOnExceeded),
		BudgetOnExceeded: insert.BudgetOnExceeded, PatchNoProgressWindow: loopPatchIntFlag(patch.NoProgressWindow),
		NoProgressWindow: insert.NoProgressWindow, PatchFanOutWidth: loopPatchIntFlag(patch.FanOutWidth),
		FanOutWidth: insert.FanOutWidth, PatchGateMaxRevisions: loopPatchIntFlag(patch.GateMaxRevisions),
		GateMaxRevisions: insert.GateMaxRevisions, PatchModelWorker: loopPatchStringFlag(patch.ModelWorker),
		ModelDefaultWorker: insert.ModelDefaultWorker, PatchModelJudge: loopPatchStringFlag(patch.ModelJudge),
		ModelDefaultJudge: insert.ModelDefaultJudge, WorkspaceID: insert.WorkspaceID, LoopName: insert.LoopName,
	}
}

func nullableLoopTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value.UTC(), Valid: !value.IsZero()}
}

func loopPatchIntFlag(value bool) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(boolToInt(value)), Valid: true}
}

func loopPatchStringFlag(value bool) sql.NullString {
	if value {
		return sql.NullString{String: "1", Valid: true}
	}
	return sql.NullString{String: "0", Valid: true}
}

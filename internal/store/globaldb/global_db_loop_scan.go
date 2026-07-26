package globaldb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
)

type loopRunScanner interface {
	Scan(dest ...any) error
}

type loopRunScanValues struct {
	run               looppkg.Run
	runID             string
	workspaceID       string
	status            string
	reattempt         string
	budgetOnExceeded  string
	createdAtRaw      string
	startedAtRaw      string
	lastProgressAtRaw string
	activeHumanRaw    string
	startMetadataRaw  string
	parentID          sql.NullString
	pauseRequested    int
	controlActorKind  sql.NullString
	controlActorID    sql.NullString
	controlRequested  sql.NullString
	inputsRaw         string
	startedByKind     string
	startedByRef      string
	startedOriginKind string
	startedOriginRef  string
	originKind        string
	originSessionID   sql.NullString
	originProfileRef  sql.NullString
	originPolicy      sql.NullString
	originCreation    sql.NullString
	networkSpecJSON   string
	networkMode       string
	networkChannel    sql.NullString
	networkSource     string
}

func scanLoopRun(row loopRunScanner) (looppkg.Run, error) {
	var values loopRunScanValues
	if err := values.scan(row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return looppkg.Run{}, looppkg.ErrRunNotFound
		}
		return looppkg.Run{}, fmt.Errorf("store: scan loop run: %w", err)
	}
	return values.toRun()
}

func (v *loopRunScanValues) scan(row loopRunScanner) error {
	return row.Scan(
		&v.runID,
		&v.workspaceID,
		&v.run.LoopName,
		&v.status,
		&v.run.Generation,
		&v.reattempt,
		&v.createdAtRaw,
		&v.startedAtRaw,
		&v.lastProgressAtRaw,
		&v.run.DefinitionVersion,
		&v.run.DefinitionDigest,
		&v.run.ActiveGateID,
		&v.activeHumanRaw,
		&v.run.BudgetApprovalSeq,
		&v.startMetadataRaw,
		&v.run.BudgetTokens,
		&v.run.BudgetWallSec,
		&v.budgetOnExceeded,
		&v.run.TokensUsed,
		&v.parentID,
		&v.pauseRequested,
		&v.controlActorKind,
		&v.controlActorID,
		&v.controlRequested,
		&v.inputsRaw,
		&v.run.IterationCap,
		&v.startedByKind,
		&v.startedByRef,
		&v.startedOriginKind,
		&v.startedOriginRef,
		&v.run.GoalContextNudgeRatio,
		&v.originKind,
		&v.originSessionID,
		&v.originProfileRef,
		&v.originPolicy,
		&v.originCreation,
		&v.networkSpecJSON,
		&v.networkMode,
		&v.networkChannel,
		&v.networkSource,
	)
}

func (v *loopRunScanValues) toRun() (looppkg.Run, error) {
	run := v.run
	run.ID = looppkg.RunID(v.runID)
	run.WorkspaceID = looppkg.WorkspaceID(v.workspaceID)
	run.Status = looppkg.Status(v.status)
	if !run.Status.Valid() {
		return looppkg.Run{}, fmt.Errorf(
			"%w: loop run %q status is invalid: %q",
			looppkg.ErrValidation,
			run.ID,
			v.status,
		)
	}
	run.ReattemptStrategy = looppkg.ReattemptStrategy(v.reattempt)
	run.BudgetOnExceeded = dsl.BudgetExceeded(v.budgetOnExceeded)
	if err := applyLoopRunScanTimestamps(&run, v.createdAtRaw, v.startedAtRaw, v.lastProgressAtRaw); err != nil {
		return looppkg.Run{}, err
	}
	if v.parentID.Valid {
		run.ParentLoopRunID = looppkg.RunID(v.parentID.String)
	}
	run.PauseRequested = v.pauseRequested != 0
	if err := applyLoopRunControlScan(&run, v); err != nil {
		return looppkg.Run{}, err
	}
	run.StartedBy = taskpkg.ActorIdentity{
		Kind: taskpkg.ActorKind(strings.TrimSpace(v.startedByKind)),
		Ref:  strings.TrimSpace(v.startedByRef),
	}
	run.StartedOrigin = taskpkg.Origin{
		Kind: taskpkg.OriginKind(strings.TrimSpace(v.startedOriginKind)),
		Ref:  strings.TrimSpace(v.startedOriginRef),
	}
	origin := looppkg.RunOrigin{
		Kind:               looppkg.RunOriginKind(strings.TrimSpace(v.originKind)),
		SessionID:          strings.TrimSpace(v.originSessionID.String),
		CreationProfileRef: strings.TrimSpace(v.originProfileRef.String),
		PolicySpecDigest:   strings.TrimSpace(v.originPolicy.String),
		CreationDigest:     strings.TrimSpace(v.originCreation.String),
	}.Normalize()
	if err := origin.Validate(); err != nil {
		return looppkg.Run{}, err
	}
	run.Origin = &origin
	if err := json.Unmarshal([]byte(v.inputsRaw), &run.Inputs); err != nil {
		return looppkg.Run{}, fmt.Errorf("store: decode loop run inputs: %w", err)
	}
	if run.Inputs == nil {
		run.Inputs = map[string]any{}
	}
	run.ActiveHumanCriteria = json.RawMessage(v.activeHumanRaw)
	if len(run.ActiveHumanCriteria) == 0 {
		run.ActiveHumanCriteria = json.RawMessage(`[]`)
	}
	if !json.Valid(run.ActiveHumanCriteria) {
		return looppkg.Run{}, fmt.Errorf("store: decode loop run active human criteria: %w", looppkg.ErrValidation)
	}
	if err := json.Unmarshal([]byte(v.startMetadataRaw), &run.StartMetadata); err != nil {
		return looppkg.Run{}, fmt.Errorf("store: decode loop run start metadata: %w", err)
	}
	if run.StartMetadata == nil {
		run.StartMetadata = map[string]any{}
	}
	networkSpec, err := decodeParticipationSnapshot(
		string(run.WorkspaceID),
		v.networkSpecJSON,
		v.networkMode,
		v.networkChannel,
		v.networkSource,
	)
	if err != nil {
		return looppkg.Run{}, err
	}
	run.SetNetworkSpec(networkSpec)
	return run, nil
}

func applyLoopRunScanTimestamps(
	run *looppkg.Run,
	createdAtRaw string,
	startedAtRaw string,
	lastProgressAtRaw string,
) error {
	createdAt, err := parseLoopRunTimestamp(createdAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse loop run created_at: %w", err)
	}
	run.CreatedAt = createdAt
	startedAt, err := parseLoopRunTimestamp(startedAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse loop run started_at: %w", err)
	}
	run.StartedAt = startedAt
	lastProgressAt, err := parseLoopRunTimestamp(lastProgressAtRaw)
	if err != nil {
		return fmt.Errorf("store: parse loop run last_progress_at: %w", err)
	}
	run.LastProgressAt = lastProgressAt
	return nil
}

type loopConfigScanValues struct {
	humanGateEnabled   int
	reattempt          sql.NullString
	enabledChecks      string
	iterationCap       sql.NullInt64
	budgetTokens       sql.NullInt64
	budgetWallSec      sql.NullInt64
	budgetOnExceeded   sql.NullString
	noProgressWindow   sql.NullInt64
	fanOutWidth        sql.NullInt64
	gateMaxRevisions   sql.NullInt64
	modelDefaultWorker sql.NullString
	modelDefaultJudge  sql.NullString
}

func (v loopConfigScanValues) toConfig() looppkg.LoopConfig {
	cfg := looppkg.LoopConfig{
		HumanGateEnabled: new(v.humanGateEnabled != 0),
		EnabledChecks:    json.RawMessage(v.enabledChecks),
	}
	if v.reattempt.Valid {
		value := looppkg.ReattemptStrategy(v.reattempt.String)
		cfg.ReattemptStrategy = &value
	}
	if v.iterationCap.Valid {
		cfg.IterationCap = new(int(v.iterationCap.Int64))
	}
	if v.budgetTokens.Valid {
		cfg.BudgetTokens = new(int(v.budgetTokens.Int64))
	}
	if v.budgetWallSec.Valid {
		cfg.BudgetWallSec = new(int(v.budgetWallSec.Int64))
	}
	if v.budgetOnExceeded.Valid {
		value := dsl.BudgetExceeded(v.budgetOnExceeded.String)
		cfg.BudgetOnExceeded = &value
	}
	if v.noProgressWindow.Valid {
		cfg.NoProgressWindow = new(int(v.noProgressWindow.Int64))
	}
	if v.fanOutWidth.Valid {
		cfg.FanOutWidth = new(int(v.fanOutWidth.Int64))
	}
	if v.gateMaxRevisions.Valid {
		cfg.GateMaxRevisions = new(int(v.gateMaxRevisions.Int64))
	}
	if v.modelDefaultWorker.Valid || v.modelDefaultJudge.Valid {
		cfg.ModelDefaults = &looppkg.ModelDefaults{}
		if v.modelDefaultWorker.Valid {
			cfg.ModelDefaults.Worker = new(v.modelDefaultWorker.String)
		}
		if v.modelDefaultJudge.Valid {
			cfg.ModelDefaults.Judge = new(v.modelDefaultJudge.String)
		}
	}
	return cfg
}

func parseLoopRunTimestamp(value string) (time.Time, error) {
	parsed, err := store.ParseTimestamp(value)
	if err == nil {
		return parsed, nil
	}
	rfc3339, parseErr := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if parseErr != nil {
		return time.Time{}, err
	}
	return rfc3339.UTC(), nil
}

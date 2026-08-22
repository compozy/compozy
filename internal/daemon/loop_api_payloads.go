package daemon

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
)

func loopDefinitionPayload(
	spec looppkg.ResourceSpec,
	def dsl.Definition,
) (contract.LoopDefinitionPayload, error) {
	document, err := loopDefinitionDocument(def)
	if err != nil {
		return contract.LoopDefinitionPayload{}, err
	}
	return contract.LoopDefinitionPayload{
		Name:        spec.Name,
		Version:     spec.Version,
		Description: spec.Description,
		Source:      contract.LoopSource(spec.Source.Normalize()),
		Catalog:     loopCatalogResourcePayload(spec.Catalog),
		Definition:  document,
	}, nil
}

func loopCatalogEntryPayload(
	spec looppkg.ResourceSpec,
	def dsl.Definition,
	summary looppkg.CatalogRunSummary,
) (contract.LoopCatalogEntryPayload, error) {
	aggregate := contract.LoopCatalogAggregatePayload{
		Runs:      summary.Aggregate30d.Runs,
		Succeeded: summary.Aggregate30d.Succeeded,
		Failed:    summary.Aggregate30d.Failed,
	}
	document, err := loopDefinitionDocument(def)
	if err != nil {
		return contract.LoopCatalogEntryPayload{}, err
	}
	var inputs map[string]contract.LoopInput
	if err := transcodeLoopAPI(document.Inputs, &inputs); err != nil {
		return contract.LoopCatalogEntryPayload{}, fmt.Errorf("daemon: encode Loop catalog inputs: %w", err)
	}
	var start []contract.LoopStartBinding
	if err := transcodeLoopAPI(document.Start, &start); err != nil {
		return contract.LoopCatalogEntryPayload{}, fmt.Errorf("daemon: encode Loop catalog starts: %w", err)
	}
	var loopContract contract.LoopContract
	if err := transcodeLoopAPI(document.Contract, &loopContract); err != nil {
		return contract.LoopCatalogEntryPayload{}, fmt.Errorf("daemon: encode Loop catalog contract: %w", err)
	}
	var lastRun *contract.LoopCatalogLastRunPayload
	if summary.LastRun != nil {
		lastRun = &contract.LoopCatalogLastRunPayload{
			ID:             string(summary.LastRun.ID),
			Status:         contract.LoopRunStatus(summary.LastRun.Status),
			BestGeneration: cloneOptional(summary.LastRun.BestGeneration),
			BestScore:      cloneOptional(summary.LastRun.BestScore),
			CreatedAt:      summary.LastRun.CreatedAt,
		}
	}
	return contract.LoopCatalogEntryPayload{
		Name:          spec.Name,
		Version:       spec.Version,
		Description:   spec.Description,
		Source:        contract.LoopSource(spec.Source.Normalize()),
		Catalog:       loopCatalogResourcePayload(spec.Catalog),
		Inputs:        inputs,
		Start:         start,
		Contract:      loopContract,
		LastRun:       lastRun,
		Aggregate30d:  aggregate,
		SuccessRate30: loopSuccessRate(aggregate),
	}, nil
}

func loopCatalogResourcePayload(spec looppkg.CatalogResourceSpec) contract.LoopCatalogResourceSpec {
	return contract.LoopCatalogResourceSpec{
		UseWhen:  spec.UseWhen,
		Keywords: append([]string(nil), spec.Keywords...),
		Category: spec.Category,
	}
}

func loopSuccessRate(aggregate contract.LoopCatalogAggregatePayload) float64 {
	if aggregate.Runs == 0 {
		return 0
	}
	return float64(aggregate.Succeeded) / float64(aggregate.Runs)
}

func loopRunPayload(run looppkg.Run) (contract.LoopRunPayload, error) {
	startMetadata, err := cloneLoopAPIMap(run.StartMetadata)
	if err != nil {
		return contract.LoopRunPayload{}, err
	}
	inputs, err := cloneLoopAPIMap(run.Inputs)
	if err != nil {
		return contract.LoopRunPayload{}, err
	}
	payload := contract.LoopRunPayload{
		ID:                           string(run.ID),
		ProfileID:                    run.ProfileID,
		WorkspaceID:                  string(run.WorkspaceID),
		LoopName:                     run.LoopName,
		Status:                       contract.LoopRunStatus(run.Status),
		CompletionState:              contract.LoopCompletionState(run.CompletionStateSnapshot()),
		Generation:                   int64(run.Generation),
		ReattemptStrategy:            contract.LoopReattemptStrategy(run.ReattemptStrategy),
		CreatedAt:                    run.CreatedAt,
		StartedAt:                    run.StartedAt,
		LastProgressAt:               run.LastProgressAt,
		StartedByKind:                string(run.StartedBy.Kind),
		StartedByRef:                 run.StartedBy.Ref,
		StartedOriginKind:            string(run.StartedOrigin.Kind),
		StartedOriginRef:             run.StartedOrigin.Ref,
		DefinitionVersion:            run.DefinitionVersion,
		DefinitionDigest:             run.DefinitionDigest,
		ActiveGateID:                 string(run.ActiveGateID),
		BudgetApprovalSeq:            run.BudgetApprovalSeq,
		StartMetadata:                startMetadata,
		IterationCap:                 run.IterationCap,
		BudgetTokens:                 run.BudgetTokens,
		BudgetWallSec:                run.BudgetWallSec,
		BudgetOnExceeded:             contract.LoopBudgetExceeded(run.BudgetOnExceeded),
		TokensUsed:                   run.TokensUsed,
		ParentLoopRunID:              string(run.ParentLoopRunID),
		PauseRequested:               run.PauseRequested,
		Inputs:                       inputs,
		ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
	}
	payload.BestGeneration = cloneOptional(run.BestGeneration)
	payload.BestScore = cloneOptional(run.BestScore)
	forks := run.ForksSnapshot()
	payload.Forks = make([]contract.LoopForkRef, 0, len(forks))
	for _, fork := range forks {
		payload.Forks = append(payload.Forks, contract.LoopForkRef{
			RunID: string(fork.RunID), Generation: fork.Generation,
		})
	}
	if forkedFrom := run.ForkedFromSnapshot(); forkedFrom != nil {
		payload.ForkedFrom = &contract.LoopForkRef{
			RunID: string(forkedFrom.RunID), Generation: forkedFrom.Generation,
		}
	}
	return payload, nil
}

func cloneOptional[T any](value *T) *T {
	if value == nil {
		return nil
	}
	return new(*value)
}

func loopRunsAggregate(runs []looppkg.Run) contract.LoopRunsAggregatePayload {
	var aggregate contract.LoopRunsAggregatePayload
	for _, run := range runs {
		aggregate.Total++
		if run.Status.Live() {
			aggregate.Live++
		}
		if run.Status.Terminal() {
			aggregate.Terminal++
		}
		switch run.Status {
		case looppkg.StatusDone:
			aggregate.Succeeded++
		case looppkg.StatusFailed, looppkg.StatusBlocked, looppkg.StatusExhausted, looppkg.StatusStalled:
			aggregate.Failed++
		default:
		}
	}
	return aggregate
}

func loopRunEventPayload(event looppkg.RunEvent) contract.LoopRunEventPayload {
	return contract.LoopRunEventPayload{
		ID:          event.ID,
		LoopRunID:   string(event.LoopRunID),
		WorkspaceID: string(event.WorkspaceID),
		Seq:         event.Seq,
		Kind:        contract.LoopRunEventKind(event.Kind),
		Payload:     cloneRawMessage(event.Payload),
		At:          event.At,
	}
}

func loopLintErrorsPayload(errors []looppkg.LintError) []contract.LoopLintErrorPayload {
	payloads := make([]contract.LoopLintErrorPayload, 0, len(errors))
	for _, item := range errors {
		payloads = append(payloads, contract.LoopLintErrorPayload{
			NodeID:   string(item.NodeID),
			Path:     item.Path,
			Code:     item.Code,
			Message:  item.Message,
			Severity: contract.LoopLintSeverity(item.Severity),
		})
	}
	sort.SliceStable(payloads, func(left, right int) bool {
		if payloads[left].NodeID != payloads[right].NodeID {
			return payloads[left].NodeID < payloads[right].NodeID
		}
		if payloads[left].Path != payloads[right].Path {
			return payloads[left].Path < payloads[right].Path
		}
		return payloads[left].Code < payloads[right].Code
	})
	return payloads
}

func cloneLoopAPIMap(input map[string]any) (map[string]any, error) {
	if input == nil {
		return nil, nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("daemon: clone loop api map: %w", err)
	}
	var cloned map[string]any
	if err := json.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("daemon: clone loop api map: %w", err)
	}
	return cloned, nil
}

func cloneRawMessage(input json.RawMessage) json.RawMessage {
	if len(input) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), input...)
}

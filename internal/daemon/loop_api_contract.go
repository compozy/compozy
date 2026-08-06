package daemon

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
)

func loopDefinitionDocument(def dsl.Definition) (contract.LoopDefinitionDocument, error) {
	doc, err := contract.NewLoopDefinitionDocument(def)
	if err != nil {
		return contract.LoopDefinitionDocument{}, fmt.Errorf("daemon: encode loop definition DTO: %w", err)
	}
	return doc, nil
}

func loopDefinitionDocumentFromJSON(raw json.RawMessage) (contract.LoopDefinitionDocument, error) {
	var doc contract.LoopDefinitionDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		return contract.LoopDefinitionDocument{}, fmt.Errorf("daemon: decode loop definition snapshot DTO: %w", err)
	}
	return doc, nil
}

func loopDefinitionDomain(doc contract.LoopDefinitionDocument) (dsl.Definition, error) {
	var def dsl.Definition
	if err := doc.Decode(&def); err != nil {
		return dsl.Definition{}, fmt.Errorf("daemon: decode loop definition DTO: %w", err)
	}
	return def, nil
}

func loopConfigPayload(cfg *looppkg.LoopConfig) (*contract.LoopConfig, error) {
	if cfg == nil {
		return nil, nil
	}
	var out contract.LoopConfig
	if err := transcodeLoopAPI(*cfg, &out); err != nil {
		return nil, fmt.Errorf("daemon: encode loop config DTO: %w", err)
	}
	return &out, nil
}

func loopConfigDomain(cfg contract.LoopConfig) (looppkg.LoopConfig, error) {
	var out looppkg.LoopConfig
	if err := transcodeLoopAPI(cfg, &out); err != nil {
		return looppkg.LoopConfig{}, fmt.Errorf("daemon: decode loop config DTO: %w", err)
	}
	return out, nil
}

func loopEffectiveConfigPayload(cfg looppkg.EffectiveConfig) (contract.LoopEffectiveConfig, error) {
	var out contract.LoopEffectiveConfig
	if err := transcodeLoopAPI(cfg, &out); err != nil {
		return contract.LoopEffectiveConfig{}, fmt.Errorf("daemon: encode loop effective config DTO: %w", err)
	}
	return out, nil
}

func loopResolvedLifecyclePayload(
	cfg looppkg.ResolvedLifecycleConfig,
) contract.LoopResolvedLifecycleConfig {
	return contract.LoopResolvedLifecycleConfig{
		RetryMaxAttempts:       cfg.RetryMaxAttempts,
		RetryBackoffBase:       cfg.RetryBackoffBase.String(),
		RetryBackoffMax:        cfg.RetryBackoffMax.String(),
		RetryNonRetryable:      append([]string(nil), cfg.RetryNonRetryable...),
		LivenessSilenceWindow:  cfg.LivenessSilenceWindow.String(),
		ResumeDeathStreakLimit: cfg.ResumeDeathStreakLimit,
		PredicateCostLimit:     cfg.PredicateCostLimit,
		WaitAdmissionAttempts:  cfg.WaitAdmissionAttempts,
		WaitAdmissionInterval:  cfg.WaitAdmissionInterval.String(),
		AdmissionHorizon:       cfg.AdmissionHorizon.String(),
		Sources:                maps.Clone(cfg.Sources),
	}
}

func loopPlanNodesPayload(nodes []looppkg.PlanNodePreview) []contract.LoopPlanNodePreview {
	out := make([]contract.LoopPlanNodePreview, 0, len(nodes))
	for _, node := range nodes {
		deps := make([]string, 0, len(node.DependsOn))
		for _, dep := range node.DependsOn {
			deps = append(deps, string(dep))
		}
		out = append(out, contract.LoopPlanNodePreview{
			ID:        string(node.ID),
			Class:     contract.LoopNodeClass(node.Class),
			Kind:      node.Kind,
			DependsOn: deps,
		})
	}
	return out
}

func loopGenerationOutputsPayload(
	outputs []looppkg.GenerationOutput,
	attemptSets ...[]looppkg.NodeAttempt,
) []contract.LoopGenerationOutput {
	attempts := []looppkg.NodeAttempt{}
	if len(attemptSets) > 0 {
		attempts = attemptSets[0]
	}
	out := make([]contract.LoopGenerationOutput, 0, len(outputs))
	for _, output := range outputs {
		payload := contract.LoopGenerationOutput{
			Generation:      int64(output.Generation),
			NodeID:          output.NodeID,
			ItemIndex:       output.ItemIndex,
			Status:          output.Status,
			OutputRef:       output.OutputRef,
			TaskRunID:       output.TaskRunID,
			SessionID:       output.SessionID,
			ChildLoopRunID:  output.ChildLoopRunID,
			ResolvedRuntime: loopResolvedRuntimePayload(output.ResolvedRuntime),
			Attempt:         output.Attempt,
			NextAttemptAt:   cloneTimePointer(output.NextAttemptAt),
		}
		if attempt := loopOutputAttempt(output, attempts); attempt != nil {
			if attempt.FailureClass != nil {
				payload.FailureClass = string(*attempt.FailureClass)
			}
			payload.Disposition = string(attempt.Disposition)
		}
		out = append(out, payload)
	}
	return out
}

func loopOutputAttempt(output looppkg.GenerationOutput, attempts []looppkg.NodeAttempt) *looppkg.NodeAttempt {
	for index := range slices.Backward(attempts) {
		attempt := &attempts[index]
		if attempt.Generation == output.Generation && string(attempt.NodeID) == output.NodeID &&
			attempt.ItemIndex == output.ItemIndex && attempt.Attempt == output.Attempt {
			return attempt
		}
	}
	return nil
}

func loopResolvedRuntimePayload(runtime *looppkg.ResolvedRuntime) *contract.LoopResolvedRuntime {
	if runtime == nil {
		return nil
	}
	return &contract.LoopResolvedRuntime{
		Provider:  runtime.Runtime.Provider,
		Model:     runtime.Runtime.Model,
		Reasoning: runtime.Runtime.Reasoning,
		Source: contract.LoopRuntimeProvenance{
			Provider:  string(runtime.Source.Provider),
			Model:     string(runtime.Source.Model),
			Reasoning: string(runtime.Source.Reasoning),
		},
	}
}

func transcodeLoopAPI(value any, target any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

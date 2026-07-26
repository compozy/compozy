package daemon

import (
	"encoding/json"
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
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

func loopGenerationOutputsPayload(outputs []looppkg.GenerationOutput) []contract.LoopGenerationOutput {
	out := make([]contract.LoopGenerationOutput, 0, len(outputs))
	for _, output := range outputs {
		out = append(out, contract.LoopGenerationOutput{
			Generation:     output.Generation,
			NodeID:         output.NodeID,
			ItemIndex:      output.ItemIndex,
			Status:         output.Status,
			OutputRef:      output.OutputRef,
			TaskRunID:      output.TaskRunID,
			ChildLoopRunID: output.ChildLoopRunID,
		})
	}
	return out
}

func transcodeLoopAPI(value any, target any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

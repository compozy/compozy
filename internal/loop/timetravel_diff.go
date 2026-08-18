package loop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/compozy/compozy/internal/loop/gate"
)

const (
	diffInlinePayloadLimit = 16 * 1024
	diffKindGeneration     = "generation"
)

func (s *service) DiffRun(ctx context.Context, workspaceID WorkspaceID, query DiffQuery) (DiffResult, error) {
	reader, ok := s.store.(GenerationOutputReader)
	if !ok {
		return DiffResult{}, fmt.Errorf("%w: generation history is unavailable", ErrActionDependencyMissing)
	}
	base, err := s.store.GetLoopRun(ctx, workspaceID, query.RunID)
	if err != nil {
		return DiffResult{}, err
	}
	against := base
	kind := diffKindGeneration
	if query.AgainstRunID != "" {
		against, err = s.store.GetLoopRun(ctx, workspaceID, query.AgainstRunID)
		if err != nil {
			return DiffResult{}, err
		}
		kind = "run"
	}
	if base.LoopName != against.LoopName {
		return DiffResult{}, reasonError(ReasonCodeDiffCrossLoop, ErrDiffCrossLoop, map[string]string{
			"base_loop": base.LoopName, "against_loop": against.LoopName,
		})
	}
	if query.Generation < 0 || query.AgainstGeneration < 0 {
		return DiffResult{}, fmt.Errorf("%w: diff generations must be positive", ErrValidation)
	}
	baseGeneration, baseOutputs, err := loadDiffGenerationOutputs(
		ctx, reader, workspaceID, base, int(query.Generation),
	)
	if err != nil {
		return DiffResult{}, err
	}
	againstGeneration, againstOutputs, err := loadDiffGenerationOutputs(
		ctx, reader, workspaceID, against, int(query.AgainstGeneration),
	)
	if err != nil {
		return DiffResult{}, err
	}
	definitionDivergence := base.DefinitionDigest != against.DefinitionDigest
	nodes, err := diffGenerationOutputs(
		ctx, reader, workspaceID, base.ID, baseOutputs, against.ID, againstOutputs, definitionDivergence,
	)
	if err != nil {
		return DiffResult{}, err
	}
	if verdictReader, ok := s.store.(gate.VerdictReader); ok {
		verdictRows, verdictErr := diffGenerationVerdicts(
			ctx, verdictReader, workspaceID, base.ID, int64(baseGeneration), against.ID, int64(againstGeneration),
		)
		if verdictErr != nil {
			return DiffResult{}, verdictErr
		}
		nodes = append(nodes, verdictRows...)
	}
	if routeReader, ok := s.store.(RouteCauseReader); ok {
		routeRows, routeErr := diffGenerationRoutes(
			ctx, routeReader, workspaceID, base.ID, int64(baseGeneration), against.ID, int64(againstGeneration),
		)
		if routeErr != nil {
			return DiffResult{}, routeErr
		}
		nodes = append(nodes, routeRows...)
	}
	sortDiffNodeRows(nodes)
	result := DiffResult{
		Kind: kind,
		Base: DiffEndpoint{RunID: base.ID, Generation: int64(baseGeneration), Status: base.Status,
			AsOf: base.Status.Live()},
		Against: DiffEndpoint{RunID: against.ID, Generation: int64(againstGeneration), Status: against.Status,
			AsOf: against.Status.Live()},
		Inputs: []DiffInputRow{}, Nodes: nodes,
		DefinitionDivergence: definitionDivergence,
	}
	if kind == "run" {
		result.Inputs, err = diffRunInputs(base.Inputs, against.Inputs)
		if err != nil {
			return DiffResult{}, err
		}
		result.Terminal = &DiffTerminalRow{Base: base.Status, Against: against.Status}
	}
	return result, nil
}

func loadDiffGenerationOutputs(
	ctx context.Context,
	reader GenerationOutputReader,
	workspaceID WorkspaceID,
	run Run,
	requested int,
) (int, []GenerationOutput, error) {
	if requested > run.Generation {
		return 0, nil, fmt.Errorf("%w: generation %d is unavailable", ErrValidation, requested)
	}
	if requested > 0 {
		outputs, err := reader.ListGenerationOutputs(ctx, workspaceID, run.ID, requested)
		return requested, outputs, err
	}
	for generation := run.Generation; generation >= 1; generation-- {
		outputs, err := reader.ListGenerationOutputs(ctx, workspaceID, run.ID, generation)
		if err != nil {
			return 0, nil, err
		}
		if len(outputs) > 0 && generationOutputsSettled(outputs) {
			return generation, outputs, nil
		}
	}
	return 0, nil, fmt.Errorf("%w: no settled generation is available", ErrValidation)
}

func generationOutputsSettled(outputs []GenerationOutput) bool {
	for _, output := range outputs {
		if !generationOutputSettled(output.Status) {
			return false
		}
	}
	return true
}

type diffOutputKey struct {
	node string
	item int
}

type diffVerdictKey struct {
	gate string
	item int
}

func diffGenerationVerdicts(
	ctx context.Context,
	reader gate.VerdictReader,
	workspaceID WorkspaceID,
	baseRunID RunID,
	baseGeneration int64,
	againstRunID RunID,
	againstGeneration int64,
) ([]DiffNodeRow, error) {
	base, err := reader.ListGateVerdicts(ctx, string(workspaceID), string(baseRunID), baseGeneration)
	if err != nil {
		return nil, err
	}
	against, err := reader.ListGateVerdicts(ctx, string(workspaceID), string(againstRunID), againstGeneration)
	if err != nil {
		return nil, err
	}
	baseByKey := indexDiffVerdicts(base)
	againstByKey := indexDiffVerdicts(against)
	keys := unionDiffVerdictKeys(baseByKey, againstByKey)
	rows := make([]DiffNodeRow, 0, len(keys))
	for _, key := range keys {
		left, leftOK := baseByKey[key]
		right, rightOK := againstByKey[key]
		leftJSON, leftErr := marshalDiffVerdict(left, leftOK)
		if leftErr != nil {
			return nil, leftErr
		}
		rightJSON, rightErr := marshalDiffVerdict(right, rightOK)
		if rightErr != nil {
			return nil, rightErr
		}
		if bytes.Equal(leftJSON, rightJSON) {
			continue
		}
		row := DiffNodeRow{NodeID: key.gate, ItemIndex: key.item, Change: "verdict"}
		if leftOK {
			row.Base = summarizeDiffJSON(leftJSON)
		}
		if rightOK {
			row.Against = summarizeDiffJSON(rightJSON)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func indexDiffVerdicts(records []gate.VerdictRecord) map[diffVerdictKey]gate.VerdictRecord {
	indexed := make(map[diffVerdictKey]gate.VerdictRecord, len(records))
	for _, record := range records {
		indexed[diffVerdictKey{gate: record.GateID, item: record.ItemIndex}] = record
	}
	return indexed
}

func unionDiffVerdictKeys(
	base map[diffVerdictKey]gate.VerdictRecord,
	against map[diffVerdictKey]gate.VerdictRecord,
) []diffVerdictKey {
	seen := make(map[diffVerdictKey]struct{}, len(base)+len(against))
	keys := make([]diffVerdictKey, 0, len(base)+len(against))
	for key := range base {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range against {
		if _, exists := seen[key]; !exists {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].gate == keys[j].gate {
			return keys[i].item < keys[j].item
		}
		return keys[i].gate < keys[j].gate
	})
	return keys
}

func marshalDiffVerdict(record gate.VerdictRecord, exists bool) (json.RawMessage, error) {
	if !exists {
		return nil, nil
	}
	value := struct {
		Outcome        gate.VerdictOutcome `json:"outcome"`
		Score          *float64            `json:"score,omitempty"`
		BlockingIssues json.RawMessage     `json:"blocking_issues"`
		Criteria       json.RawMessage     `json:"criteria"`
	}{record.Outcome, record.Score, record.BlockingIssues, record.Criteria}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("loop: encode diff verdict %q: %w", record.GateID, err)
	}
	return raw, nil
}

func diffGenerationRoutes(
	ctx context.Context,
	reader RouteCauseReader,
	workspaceID WorkspaceID,
	baseRunID RunID,
	baseGeneration int64,
	againstRunID RunID,
	againstGeneration int64,
) ([]DiffNodeRow, error) {
	base, err := reader.ListRouteCauses(ctx, workspaceID, baseRunID, baseGeneration)
	if err != nil {
		return nil, err
	}
	against, err := reader.ListRouteCauses(ctx, workspaceID, againstRunID, againstGeneration)
	if err != nil {
		return nil, err
	}
	baseByKey := indexDiffRoutes(base)
	againstByKey := indexDiffRoutes(against)
	keys := make(map[diffOutputKey]struct{}, len(baseByKey)+len(againstByKey))
	for key := range baseByKey {
		keys[key] = struct{}{}
	}
	for key := range againstByKey {
		keys[key] = struct{}{}
	}
	rows := make([]DiffNodeRow, 0, len(keys))
	for key := range keys {
		left, leftOK := baseByKey[key]
		right, rightOK := againstByKey[key]
		leftJSON, leftErr := marshalDiffRoute(left, leftOK)
		if leftErr != nil {
			return nil, leftErr
		}
		rightJSON, rightErr := marshalDiffRoute(right, rightOK)
		if rightErr != nil {
			return nil, rightErr
		}
		if bytes.Equal(leftJSON, rightJSON) {
			continue
		}
		row := DiffNodeRow{NodeID: key.node, ItemIndex: key.item, Change: "changed"}
		if leftOK {
			row.Base = summarizeDiffJSON(leftJSON)
		}
		if rightOK {
			row.Against = summarizeDiffJSON(rightJSON)
			row.Cause = right.Cause
		} else if leftOK {
			row.Cause = left.Cause
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func indexDiffRoutes(routes []RouteCause) map[diffOutputKey]RouteCause {
	indexed := make(map[diffOutputKey]RouteCause, len(routes))
	for _, route := range routes {
		indexed[diffOutputKey{node: string(route.NodeID), item: route.ItemIndex}] = route
	}
	return indexed
}

func marshalDiffRoute(route RouteCause, exists bool) (json.RawMessage, error) {
	if !exists {
		return nil, nil
	}
	value := struct {
		Route       NodeID `json:"route"`
		Cause       string `json:"cause"`
		MatchedWhen string `json:"matched_when,omitempty"`
		Default     bool   `json:"default,omitempty"`
	}{route.Route, route.Cause, route.MatchedWhen, route.Default}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("loop: encode diff route %q: %w", route.NodeID, err)
	}
	return raw, nil
}

func sortDiffNodeRows(rows []DiffNodeRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].NodeID == rows[j].NodeID {
			if rows[i].ItemIndex == rows[j].ItemIndex {
				return rows[i].Change < rows[j].Change
			}
			return rows[i].ItemIndex < rows[j].ItemIndex
		}
		return rows[i].NodeID < rows[j].NodeID
	})
}

func indexDiffOutputs(outputs []GenerationOutput) map[diffOutputKey]GenerationOutput {
	indexed := make(map[diffOutputKey]GenerationOutput, len(outputs))
	for _, output := range outputs {
		indexed[diffOutputKey{node: output.NodeID, item: output.ItemIndex}] = output
	}
	return indexed
}

func diffOutputValue(
	ctx context.Context,
	reader GenerationOutputReader,
	workspaceID WorkspaceID,
	runID RunID,
	output GenerationOutput,
) (DiffValue, error) {
	if output.OutputRef == "" {
		return summarizeDiffJSON(json.RawMessage(fmt.Sprintf("%q", output.Status))), nil
	}
	payload, err := reader.GetGenerationOutputPayload(ctx, GenerationOutputPayloadKey{
		WorkspaceID: workspaceID, RunID: runID, Generation: output.Generation,
		NodeID: NodeID(output.NodeID), ItemIndex: output.ItemIndex, OutputRef: output.OutputRef,
	})
	if err != nil {
		return DiffValue{}, err
	}
	return summarizeDiffJSON(payload), nil
}

func diffRunInputs(base, against map[string]any) ([]DiffInputRow, error) {
	keys := make(map[string]struct{}, len(base)+len(against))
	for key := range base {
		keys[key] = struct{}{}
	}
	for key := range against {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	rows := make([]DiffInputRow, 0, len(ordered))
	for _, key := range ordered {
		left, err := json.Marshal(base[key])
		if err != nil {
			return nil, fmt.Errorf("loop: encode base input %q: %w", key, err)
		}
		right, err := json.Marshal(against[key])
		if err != nil {
			return nil, fmt.Errorf("loop: encode against input %q: %w", key, err)
		}
		if bytes.Equal(left, right) {
			continue
		}
		rows = append(rows, DiffInputRow{Key: key, Base: summarizeDiffJSON(left), Against: summarizeDiffJSON(right)})
	}
	return rows, nil
}

func summarizeDiffJSON(payload json.RawMessage) DiffValue {
	if len(payload) <= diffInlinePayloadLimit {
		return DiffValue{Inline: append(json.RawMessage(nil), payload...)}
	}
	sum := sha256.Sum256(payload)
	return DiffValue{Size: len(payload), Hash: "sha256:" + hex.EncodeToString(sum[:])}
}

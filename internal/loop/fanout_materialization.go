package loop

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
	"github.com/compozy/compozy/internal/task"
)

const fanOutMaterializationKind = "fan_out"

type fanOutMaterialization struct {
	Kind        string              `json:"kind"`
	Branches    int                 `json:"branches"`
	BatchSize   int                 `json:"batch_size"`
	MaxParallel int                 `json:"max_parallel"`
	Chunks      [][]fanOutCandidate `json:"chunks"`
}

type fanOutCandidate struct {
	Index int `json:"index"`
	Item  any `json:"item"`
}

type fanOutFilterEvaluation struct {
	Candidates  []fanOutCandidate
	Disposition *PredicateFailureDisposition
	Diagnostics []PredicateDiagnostic
}

func buildFanOutMaterialization(
	node dsl.Node,
	candidates []fanOutCandidate,
	defaultMaxParallel int,
) (fanOutMaterialization, *task.CoordinatorTerminal) {
	batchSize := node.BatchSize
	if batchSize <= 0 {
		batchSize = 1
	}
	maxParallel := node.MaxParallel
	if maxParallel <= 0 {
		maxParallel = defaultMaxParallel
	}
	if maxParallel <= 0 {
		maxParallel = 1
	}
	chunks := chunkFanOutCandidates(candidates, batchSize)
	if node.MaxFanOut > 0 && len(chunks) > node.MaxFanOut {
		return fanOutMaterialization{}, fanOutBoundTerminal()
	}
	return fanOutMaterialization{
		Kind:        fanOutMaterializationKind,
		Branches:    len(chunks),
		BatchSize:   batchSize,
		MaxParallel: maxParallel,
		Chunks:      chunks,
	}, nil
}

func chunkFanOutCandidates(candidates []fanOutCandidate, batchSize int) [][]fanOutCandidate {
	if len(candidates) == 0 {
		return [][]fanOutCandidate{}
	}
	branchCount := int(math.Ceil(float64(len(candidates)) / float64(batchSize)))
	chunks := make([][]fanOutCandidate, 0, branchCount)
	for start := 0; start < len(candidates); start += batchSize {
		end := min(start+batchSize, len(candidates))
		chunks = append(chunks, append([]fanOutCandidate(nil), candidates[start:end]...))
	}
	return chunks
}

func indexedFanOutCandidates(items []any) []fanOutCandidate {
	candidates := make([]fanOutCandidate, len(items))
	for index, item := range items {
		candidates[index] = fanOutCandidate{Index: index, Item: item}
	}
	return candidates
}

func fanOutBranchIndexes(materialization fanOutMaterialization) []int {
	indexes := make([]int, 0, len(materialization.Chunks))
	for _, chunk := range materialization.Chunks {
		if len(chunk) > 0 {
			indexes = append(indexes, chunk[0].Index)
		}
	}
	return indexes
}

func fanOutMaterializationRef(materialization fanOutMaterialization) (string, error) {
	data, err := json.Marshal(materialization)
	if err != nil {
		return "", fmt.Errorf("loop: marshal fan-out materialization: %w", err)
	}
	return string(data), nil
}

func parseFanOutMaterialization(ref string) (fanOutMaterialization, bool, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return fanOutMaterialization{}, false, nil
	}
	var materialization fanOutMaterialization
	if err := json.Unmarshal([]byte(trimmed), &materialization); err != nil {
		return fanOutMaterialization{}, false, fmt.Errorf(
			"%w: decode fan-out materialization: %w",
			ErrValidation,
			err,
		)
	}
	if materialization.Kind != fanOutMaterializationKind {
		return fanOutMaterialization{}, false, nil
	}
	return materialization, true, nil
}

func fanOutItem(
	outputs []GenerationOutput,
	fanOutID dsl.NodeID,
	itemIndex int,
) (any, bool, error) {
	for _, output := range outputs {
		if output.NodeID != string(fanOutID) || output.ItemIndex != 0 {
			continue
		}
		materialization, ok, err := parseFanOutMaterialization(generationOutputRuntimePayload(output))
		if err != nil || !ok {
			return nil, false, err
		}
		var chunk []fanOutCandidate
		for _, candidateChunk := range materialization.Chunks {
			if len(candidateChunk) > 0 && candidateChunk[0].Index == itemIndex {
				chunk = candidateChunk
				break
			}
		}
		if len(chunk) == 0 {
			return nil, false, nil
		}
		// batch_size: 1 scopes `.item` to the fanned element itself; larger batch
		// sizes scope `.item` to the chunk slice, even for a short final chunk.
		if materialization.BatchSize == 1 && len(chunk) == 1 {
			return chunk[0].Item, true, nil
		}
		items := make([]any, len(chunk))
		for index, candidate := range chunk {
			items[index] = candidate.Item
		}
		return items, true, nil
	}
	return nil, false, nil
}

func resolveFanOutCollection(
	resolved *ResolvedDefinition,
	node dsl.Node,
	namespace map[string]any,
) ([]any, error) {
	key := fmt.Sprintf("nodes.%s.collection", node.ID)
	if tmpl := resolved.Templates[key]; tmpl != nil && len(tmpl.References) == 1 &&
		isPureTemplateReference(node.Collection, tmpl.References[0]) {
		value, ok := valueAtPath(namespace, tmpl.References[0].Path)
		if !ok {
			return nil, fmt.Errorf(
				"%w: fan-out collection reference %q is unavailable: %w",
				ErrActionMaterialization,
				tmpl.References[0].Raw,
				ErrValidation,
			)
		}
		return collectionItems(value)
	}
	rendered, err := refs.RenderTemplateString(key, node.Collection, namespace)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrActionMaterialization, key, err)
	}
	return collectionItems(rendered)
}

func evaluateFanOutFilter(
	resolved *ResolvedDefinition,
	node dsl.Node,
	namespace map[string]any,
	items []any,
) (fanOutFilterEvaluation, error) {
	candidates := indexedFanOutCandidates(items)
	if strings.TrimSpace(node.Filter) == "" {
		return fanOutFilterEvaluation{Candidates: candidates}, nil
	}
	key := fmt.Sprintf("nodes.%s.filter", node.ID)
	condition := resolved.Conditions[key]
	if condition == nil {
		return fanOutFilterEvaluation{}, fmt.Errorf(
			"%w: compiled fan-out filter %q is missing",
			ErrValidation,
			key,
		)
	}
	result := fanOutFilterEvaluation{Candidates: make([]fanOutCandidate, 0, len(items))}
	for index, item := range items {
		variables := fanOutFilterVariables(namespace, node, item, index)
		evaluated, err := evaluatePredicate(
			key,
			condition,
			variables,
			PredicateRouting,
			node.OnEvalError,
		)
		if err != nil {
			return fanOutFilterEvaluation{}, fmt.Errorf("loop: evaluate fan-out %s filter: %w", node.ID, err)
		}
		result.Diagnostics = append(result.Diagnostics, evaluated.Diagnostics...)
		if evaluated.Disposition != nil {
			result.Disposition = evaluated.Disposition
			return result, nil
		}
		if evaluated.Value {
			result.Candidates = append(result.Candidates, fanOutCandidate{Index: index, Item: item})
		}
	}
	return result, nil
}

func fanOutFilterVariables(namespace map[string]any, node dsl.Node, item any, index int) map[string]any {
	variables := make(map[string]any, len(namespace)+4)
	maps.Copy(variables, namespace)
	variables["item"] = item
	variables["index"] = int64(index)
	if name := strings.TrimSpace(node.BindAs); name != "" {
		variables[name] = item
	}
	if name := strings.TrimSpace(node.IndexAs); name != "" {
		variables[name] = int64(index)
	}
	return variables
}

func isPureTemplateReference(raw string, reference refs.Reference) bool {
	trimmed := strings.TrimSpace(raw)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return false
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{"), "}}"))
	inner = strings.TrimPrefix(inner, ".")
	referenceRaw := strings.TrimPrefix(reference.Raw, ".")
	return inner == referenceRaw && !strings.ContainsAny(inner, " |")
}

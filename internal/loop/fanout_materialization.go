package loop

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
	"github.com/compozy/agh/internal/task"
)

const fanOutMaterializationKind = "fan_out"

type fanOutMaterialization struct {
	Kind        string  `json:"kind"`
	Branches    int     `json:"branches"`
	BatchSize   int     `json:"batch_size"`
	MaxParallel int     `json:"max_parallel"`
	Chunks      [][]any `json:"chunks"`
}

func buildFanOutMaterialization(
	node dsl.Node,
	items []any,
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
	if node.MaxFanOut > LoopMaxFanoutWidth || maxParallel > LoopMaxFanoutWidth {
		return fanOutMaterialization{}, fanOutCeilingTerminal()
	}
	chunks := chunkFanOutItems(items, batchSize)
	if node.MaxFanOut > 0 && len(chunks) > node.MaxFanOut {
		return fanOutMaterialization{}, fanOutCeilingTerminal()
	}
	if len(chunks) > LoopMaxFanoutWidth {
		return fanOutMaterialization{}, fanOutCeilingTerminal()
	}
	return fanOutMaterialization{
		Kind:        fanOutMaterializationKind,
		Branches:    len(chunks),
		BatchSize:   batchSize,
		MaxParallel: maxParallel,
		Chunks:      chunks,
	}, nil
}

func chunkFanOutItems(items []any, batchSize int) [][]any {
	if len(items) == 0 {
		return [][]any{}
	}
	branchCount := int(math.Ceil(float64(len(items)) / float64(batchSize)))
	chunks := make([][]any, 0, branchCount)
	for start := 0; start < len(items); start += batchSize {
		end := min(start+batchSize, len(items))
		chunks = append(chunks, append([]any(nil), items[start:end]...))
	}
	return chunks
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
		materialization, ok, err := parseFanOutMaterialization(output.OutputRef)
		if err != nil || !ok {
			return nil, false, err
		}
		if itemIndex < 0 || itemIndex >= len(materialization.Chunks) {
			return nil, false, nil
		}
		chunk := materialization.Chunks[itemIndex]
		// batch_size: 1 scopes `.item` to the fanned element itself; larger batch
		// sizes scope `.item` to the chunk slice, even for a short final chunk.
		if materialization.BatchSize == 1 && len(chunk) == 1 {
			return chunk[0], true, nil
		}
		return chunk, true, nil
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
				"%w: fan-out collection reference %q is unavailable",
				ErrValidation,
				tmpl.References[0].Raw,
			)
		}
		return collectionItems(value)
	}
	rendered, err := refs.RenderTemplateString(key, node.Collection, namespace)
	if err != nil {
		return nil, err
	}
	return collectionItems(rendered)
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

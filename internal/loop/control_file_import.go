package loop

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/loop/dsl/refs"
)

const fileImportParseRequiredMessage = "file-import parse is required (json|text)"

func isFileImportSourceNode(node dsl.Node) bool {
	return node.Class == dsl.NodeClassSource && node.Kind == string(dsl.SourceFileImport)
}

func evaluateFileImportNode(
	run Run,
	generation int,
	resolved *ResolvedDefinition,
	topology controlTopology,
	output GenerationOutput,
	node dsl.Node,
	outputs []GenerationOutput,
) (GenerationOutput, error) {
	namespace, err := runtimeNamespace(
		run,
		generation,
		resolved.Definition.Graph,
		topology,
		outputs,
		node.ID,
		output.ItemIndex,
	)
	if err != nil {
		return GenerationOutput{}, err
	}
	pattern, err := refs.RenderTemplateString(
		fmt.Sprintf("nodes.%s.pattern", node.ID),
		node.Pattern,
		namespace,
	)
	if err != nil {
		return GenerationOutput{}, err
	}
	path := strings.TrimSpace(pattern)
	if path == "" {
		return GenerationOutput{}, fmt.Errorf("%w: file-import pattern is required", ErrValidation)
	}
	ref, err := fileImportOutputRef(node.Parse, path)
	if err != nil {
		return GenerationOutput{}, err
	}
	output.Status = generationOutputSucceeded
	output.OutputRef = ref
	return output, nil
}

func fileImportOutputRef(parse dsl.FileParseKind, path string) (string, error) {
	var payload any
	switch parse {
	case dsl.FileParseJSON:
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("loop: read file-import %q: %w", path, err)
		}
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			return "", fmt.Errorf("loop: decode file-import JSON: %w", err)
		}
		payload = decoded
	case dsl.FileParseText:
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("loop: read file-import %q: %w", path, err)
		}
		payload = map[string]any{"text": string(data)}
	default:
		return "", fmt.Errorf("%w: %s", ErrValidation, fileImportParseRequiredMessage)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("loop: marshal file-import output: %w", err)
	}
	return string(encoded), nil
}

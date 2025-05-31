package ref

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ProcessReader processes YAML from an io.Reader and evaluates directives.
func ProcessReader(r io.Reader, options ...EvalConfigOption) (Node, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}
	return ProcessBytes(data, options...)
}

// ProcessBytes processes YAML bytes and evaluates directives.
func ProcessBytes(data []byte, options ...EvalConfigOption) (Node, error) {
	var node Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	ev := NewEvaluator(options...)
	result, err := ev.Eval(node)
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	return result, nil
}

// ProcessFile processes a YAML file and evaluates directives.
func ProcessFile(path string, options ...EvalConfigOption) (Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return ProcessBytes(data, options...)
}

// ProcessBytesWithEvaluator processes YAML bytes using a provided evaluator.
// This allows reusing the same evaluator (and its cache) across multiple operations.
func ProcessBytesWithEvaluator(data []byte, ev *Evaluator) (Node, error) {
	var node Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	result, err := ev.Eval(node)
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	return result, nil
}

// ProcessReaderWithEvaluator processes YAML from an io.Reader using a provided evaluator.
func ProcessReaderWithEvaluator(r io.Reader, ev *Evaluator) (Node, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}
	return ProcessBytesWithEvaluator(data, ev)
}

// ProcessFileWithEvaluator processes a YAML file using a provided evaluator.
func ProcessFileWithEvaluator(path string, ev *Evaluator) (Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return ProcessBytesWithEvaluator(data, ev)
}

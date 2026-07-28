package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type workspaceResolutionOutputValue struct {
	Source string
	Value  any
}

func (v workspaceResolutionOutputValue) MarshalJSON() ([]byte, error) {
	raw, err := marshalJSONWithoutHTMLEscape(v.Value)
	if err != nil {
		return nil, fmt.Errorf("cli: marshal resolved workspace output: %w", err)
	}
	source, err := marshalJSONWithoutHTMLEscape(strings.TrimSpace(v.Source))
	if err != nil {
		return nil, fmt.Errorf("cli: marshal workspace resolution source: %w", err)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		if len(trimmed) == 2 {
			return []byte(`{"resolution_source":` + string(source) + `}`), nil
		}
		return append(
			[]byte(`{"resolution_source":`+string(source)+`,`),
			trimmed[1:]...,
		), nil
	}
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(trimmed, &items); err != nil {
			return nil, fmt.Errorf("cli: decode resolved workspace output list: %w", err)
		}
		for index := range items {
			item := bytes.TrimSpace(items[index])
			if len(item) == 0 || item[0] != '{' {
				continue
			}
			if len(item) == 2 {
				items[index] = json.RawMessage(`{"resolution_source":` + string(source) + `}`)
				continue
			}
			items[index] = append(
				[]byte(`{"resolution_source":`+string(source)+`,`),
				item[1:]...,
			)
		}
		return marshalJSONWithoutHTMLEscape(items)
	}
	return marshalJSONWithoutHTMLEscape(struct {
		ResolutionSource string          `json:"resolution_source"`
		Data             json.RawMessage `json:"data"`
	}{
		ResolutionSource: strings.TrimSpace(v.Source),
		Data:             raw,
	})
}

func marshalJSONWithoutHTMLEscape(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(output.Bytes(), []byte{'\n'}), nil
}

func outputValueWithWorkspaceResolution(cmd *cobra.Command, value any) any {
	resolution, ok := commandWorkspaceResolution(cmd)
	if !ok {
		return value
	}
	return workspaceResolutionOutputValue{Source: resolution.Source, Value: value}
}

func outputToonWithWorkspaceResolution(cmd *cobra.Command, rendered string) string {
	if resolution, ok := commandWorkspaceResolution(cmd); ok {
		return "resolution_source: " + resolution.Source + "\n" + rendered
	}
	return rendered
}

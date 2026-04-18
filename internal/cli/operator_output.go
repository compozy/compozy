package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const (
	operatorOutputFormatText = "text"
	operatorOutputFormatJSON = "json"
)

func normalizeOperatorOutputFormat(value string) (string, error) {
	resolved := strings.TrimSpace(value)
	if resolved == "" {
		return operatorOutputFormatText, nil
	}

	switch resolved {
	case operatorOutputFormatText, operatorOutputFormatJSON:
		return resolved, nil
	default:
		return "", fmt.Errorf(
			"output format must be one of %s or %s",
			operatorOutputFormatText,
			operatorOutputFormatJSON,
		)
	}
}

func writeOperatorJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

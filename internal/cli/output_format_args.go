package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
)

func isStructuredAgentCommandError(err error) bool {
	agentErr, ok := errors.AsType[*agentidentity.Error](err)
	return ok && agentErr != nil
}

func requestedOutputFormat(args []string) OutputFormat {
	mode := OutputHuman
	for i := 0; i < len(args); i++ {
		switch arg := strings.TrimSpace(args[i]); {
		case arg == jsonFlagArg:
			mode = OutputJSON
		case arg == "-o" || arg == outputFlagArg:
			if i+1 < len(args) {
				mode = OutputFormat(strings.ToLower(strings.TrimSpace(args[i+1])))
				i++
			}
		case strings.HasPrefix(arg, outputFlagArg+"="):
			mode = OutputFormat(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, outputFlagArg+"="))))
		case strings.HasPrefix(arg, "-o="):
			mode = OutputFormat(strings.ToLower(strings.TrimSpace(strings.TrimPrefix(arg, "-o="))))
		}
	}
	return mode
}

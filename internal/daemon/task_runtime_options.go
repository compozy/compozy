package daemon

import (
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/session"
	speedpkg "github.com/compozy/compozy/internal/speed"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func applyTaskRuntimeOptions(
	opts *session.CreateOpts,
	reasoningEffort string,
	runtimeSpeed speedpkg.Speed,
	options []taskpkg.ACPOptionSelection,
) {
	if opts == nil {
		return
	}
	opts.ReasoningEffort = strings.TrimSpace(reasoningEffort)
	opts.Speed = runtimeSpeed
	opts.ACPOptions = make([]acp.SessionConfigOptionSelection, 0, len(options))
	for _, option := range options {
		selection := acp.SessionConfigOptionSelection{
			ID:      strings.TrimSpace(option.ID),
			ValueID: strings.TrimSpace(option.ValueID),
		}
		if option.BoolValue != nil {
			selection.BoolValue = new(*option.BoolValue)
		}
		opts.ACPOptions = append(opts.ACPOptions, selection)
	}
}

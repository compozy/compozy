package daemon

import toolspkg "github.com/compozy/compozy/internal/tools"

type resultViews[T any] struct {
	Compact func(T) any
	Full    func(T) any
	Preview func(T) string
}

func (v resultViews[T]) result(view toolspkg.ResultView, value T) (toolspkg.ToolResult, error) {
	var projected any
	if view == toolspkg.ResultViewFull {
		projected = value
		if v.Full != nil {
			projected = v.Full(value)
		}
	} else {
		projected = v.Compact(value)
	}
	preview := ""
	if v.Preview != nil {
		preview = v.Preview(value)
	}
	return structuredResult(projected, preview)
}

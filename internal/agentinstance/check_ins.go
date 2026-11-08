package agentinstance

import (
	"context"
	"time"
)

type CheckIn struct {
	OccurrenceID string
	ToolsStatus  string
	ToolCalls    []CheckInToolCall
	ID           string
	Result       string
	Detail       string
	Reason       string
	CreatedAt    time.Time
}

type CheckInReader interface {
	CountQuietCheckIns(context.Context, string, *time.Time) (int64, error)
	ListCheckIns(context.Context, string, string, int) ([]CheckIn, error)
}

package agentinstance

import (
	"context"
	"time"
)

type CheckIn struct {
	ID        string
	Result    string
	Reason    string
	CreatedAt time.Time
}

type CheckInReader interface {
	CountQuietCheckIns(context.Context, string, *time.Time) (int64, error)
	ListCheckIns(context.Context, string, string, int) ([]CheckIn, error)
}

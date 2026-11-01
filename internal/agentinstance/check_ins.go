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
	ListCheckIns(context.Context, string, string, int) ([]CheckIn, error)
}

package main

import (
	"context"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/compozy/internal/testutil/acpmock"
)

func (a *mockAgent) emitTextBurst(
	ctx context.Context, sessionID acpsdk.SessionId, update func(string) acpsdk.SessionUpdate, step acpmock.Step,
) (acpmock.DiagnosticsStep, error) {
	for range step.BurstCount {
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: sessionID, Update: update(step.Text),
		}); err != nil {
			return acpmock.DiagnosticsStep{}, err
		}
	}
	return acpmock.DiagnosticsStep{Kind: step.Kind, Text: strings.Repeat(step.Text, step.BurstCount)}, nil
}

package main

import (
	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/testutil/acpmock"
)

func promptResponseUsage(turn acpmock.TurnFixture) *acpsdk.Usage {
	if turn.Usage == nil {
		return nil
	}
	totalTokens := turn.Usage.InputTokens + turn.Usage.OutputTokens
	if turn.Usage.TotalTokens != nil {
		totalTokens = *turn.Usage.TotalTokens
	}
	return &acpsdk.Usage{
		InputTokens:  turn.Usage.InputTokens,
		OutputTokens: turn.Usage.OutputTokens,
		TotalTokens:  totalTokens,
	}
}

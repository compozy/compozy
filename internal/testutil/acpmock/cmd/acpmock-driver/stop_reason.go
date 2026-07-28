package main

import (
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

func stopReason(raw string) acpsdk.StopReason {
	switch strings.TrimSpace(raw) {
	case string(acpsdk.StopReasonMaxTokens):
		return acpsdk.StopReasonMaxTokens
	case string(acpsdk.StopReasonMaxTurnRequests):
		return acpsdk.StopReasonMaxTurnRequests
	case string(acpsdk.StopReasonRefusal):
		return acpsdk.StopReasonRefusal
	case string(acpsdk.StopReasonCancelled):
		return acpsdk.StopReasonCancelled
	default:
		return acpsdk.StopReasonEndTurn
	}
}

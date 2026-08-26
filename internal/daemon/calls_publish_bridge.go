package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/network"
)

const defaultCallPublicationThread = "thread_calls"

type daemonCallPublishBridge struct {
	network func() coreNetworkSender
}

type coreNetworkSender interface {
	Send(context.Context, network.SendRequest) (string, error)
}

var _ callspkg.PublishBridge = (*daemonCallPublishBridge)(nil)

func (b *daemonCallPublishBridge) PublishResultEvidence(
	ctx context.Context,
	evidence callspkg.ResultEvidence,
) (string, error) {
	if b == nil || b.network == nil || b.network() == nil {
		return "", fmt.Errorf("daemon: Network is unavailable")
	}
	threadID := strings.TrimSpace(evidence.ThreadID)
	if threadID == "" {
		threadID = defaultCallPublicationThread
	}
	surface := network.SurfaceThread
	body, err := json.Marshal(network.SayBody{
		Text: fmt.Sprintf(
			"Call result %s (%d B). Preview: %s. Fetch: %s",
			evidence.CallID,
			evidence.ResultBytes,
			strings.TrimSpace(string(evidence.ResultPreview)),
			evidence.FetchPath,
		),
		Intent: "result",
	})
	if err != nil {
		return "", fmt.Errorf("daemon: encode call publication: %w", err)
	}
	messageID := strings.TrimSpace(evidence.MessageID)
	return b.network().Send(ctx, network.SendRequest{
		SessionID:   strings.TrimSpace(evidence.SourceSessionID),
		WorkspaceID: strings.TrimSpace(evidence.WorkspaceID),
		Channel:     strings.TrimSpace(evidence.Channel),
		Surface:     &surface,
		ThreadID:    &threadID,
		Kind:        network.KindSay,
		Body:        body,
		ID:          &messageID,
	})
}

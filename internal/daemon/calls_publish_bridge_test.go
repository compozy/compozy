package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/compozy/compozy/internal/network"
	"github.com/compozy/compozy/internal/network/participation"
)

func TestDaemonCallPublishBridge(t *testing.T) {
	t.Parallel()

	t.Run("Should build transport fetch evidence and preserve the stable message identity", func(t *testing.T) {
		t.Parallel()
		sender := &recordingCallPublishSender{}
		bridge := &daemonCallPublishBridge{network: func() coreNetworkSender { return sender }}

		messageID, err := bridge.PublishResultEvidence(t.Context(), callspkg.ResultEvidence{
			CallID: "call-1", WorkspaceID: "ws-1", SourceSessionID: "ses-1",
			Channel: "engineering", ThreadID: "thread-1", MessageID: "msg-stable",
			ResultPreview: json.RawMessage(`{"verdict":"approved"}`), ResultBytes: 22,
		})

		if err != nil || messageID != "msg-stable" || len(sender.requests) != 1 {
			t.Fatalf("PublishResultEvidence() = %q, %v requests=%#v", messageID, err, sender.requests)
		}
		request := sender.requests[0]
		if request.ID == nil || *request.ID != "msg-stable" {
			t.Fatalf("Network message identity = %#v, want stable request id", request.ID)
		}
		var body network.SayBody
		if err := json.Unmarshal(request.Body, &body); err != nil {
			t.Fatalf("decode Network body error = %v", err)
		}
		if !strings.Contains(body.Text, "/api/workspaces/ws-1/calls/call-1/result") {
			t.Fatalf("Network evidence text = %q, want daemon-owned fetch path", body.Text)
		}
	})

	t.Run("Should map a missing live sender to no Network participation", func(t *testing.T) {
		t.Parallel()

		sender := &recordingCallPublishSender{err: network.ErrLocalPeerNotFound}
		bridge := &daemonCallPublishBridge{network: func() coreNetworkSender { return sender }}

		_, err := bridge.PublishResultEvidence(t.Context(), callspkg.ResultEvidence{
			CallID: "call-1", WorkspaceID: "ws-1", SourceSessionID: "ses-local",
			Channel: "engineering", MessageID: "msg-stable",
			ResultPreview: json.RawMessage(`{"verdict":"approved"}`), ResultBytes: 22,
		})

		if !errors.Is(err, participation.ErrNotParticipating) ||
			!errors.Is(err, network.ErrLocalPeerNotFound) {
			t.Fatalf("PublishResultEvidence() error = %v, want participation and transport causes", err)
		}
	})
}

type recordingCallPublishSender struct {
	requests []network.SendRequest
	err      error
}

func (s *recordingCallPublishSender) Send(_ context.Context, request network.SendRequest) (string, error) {
	s.requests = append(s.requests, request)
	if s.err != nil {
		return "", s.err
	}
	if request.ID == nil {
		return "", nil
	}
	return *request.ID, nil
}

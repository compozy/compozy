//go:build integration

package network

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestProtocolFixturesRoundTripWithoutSemanticDrift(t *testing.T) {
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	opts := ValidateOptions{Now: now, MaxReplayAge: DefaultMaxReplayAge}

	fixtures := []struct {
		name string
		raw  []byte
	}{
		{
			name: "greet",
			raw: []byte(`{
			  "protocol": "agh-network/v0",
			  "workspace_id": "wks_test",
			  "id": "msg_greet_01",
			  "kind": "greet",
			  "channel": "builders",
			  "from": "coder.sess-abc",
			  "to": null,
			  "ts": 1775822400,
			  "body": {
			    "peer_card": {
			      "peer_id": "coder.sess-abc",
			      "profiles_supported": ["agh-network/v0"],
			      "capabilities": ["workspace.patch.apply"],
			      "artifacts_supported": ["capability"],
			      "trust_modes_supported": ["unverified"]
			    }
			  }
			}`),
		},
		{
			name: "whois response",
			raw: []byte(`{
			  "protocol": "agh-network/v0",
			  "workspace_id": "wks_test",
			  "id": "msg_whois_01",
			  "kind": "whois",
			  "channel": "builders",
			  "from": "reviewer.sess-xyz",
			  "to": "coder.sess-abc",
			  "reply_to": "msg_greet_01",
			  "ts": 1775822400,
			  "body": {
			    "type": "response",
			    "peer_card": {
			      "peer_id": "reviewer.sess-xyz",
			      "profiles_supported": ["agh-network/v0"],
			      "capabilities": ["chat.review"],
			      "artifacts_supported": ["capability"],
			      "trust_modes_supported": ["unverified"]
			    }
			  }
			}`),
		},
		{
			name: "say",
			raw: []byte(`{
			  "protocol": "agh-network/v0",
			  "workspace_id": "wks_test",
			  "id": "msg_say_01",
			  "kind": "say",
			  "channel": "builders",
			  "surface": "thread",
			  "thread_id": "thread_patch_42",
			  "from": "coder.sess-abc",
			  "to": null,
			  "ts": 1775822400,
			  "body": {
			    "text": "I can take the failing migration tests.",
			    "intent": "status_update"
			  }
			}`),
		},
		{
			name: "direct room say",
			raw: []byte(`{
			  "protocol": "agh-network/v0",
			  "workspace_id": "wks_test",
			  "id": "msg_direct_01",
			  "kind": "say",
			  "channel": "builders",
			  "surface": "direct",
			  "direct_id": "direct_0123456789abcdef0123456789abcdef",
			  "from": "coder.sess-abc",
			  "to": "reviewer.sess-xyz",
			  "work_id": "work_patch_42",
			  "reply_to": "msg_say_01",
			  "trace_id": "trace_ops_patch_42",
			  "causation_id": "msg_say_01",
			  "ts": 1775822400,
			  "expires_at": 1775823000,
			  "body": {
			    "text": "Please inspect auth.go and tell me what is failing.",
			    "intent": "review_request"
			  },
			  "proof": null,
			  "ext": {
			    "agh.workflow": {"ticket": "NET-42"}
			  }
			}`),
		},
		{
			name: "capability",
			raw: []byte(`{
			  "protocol": "agh-network/v0",
			  "workspace_id": "wks_test",
			  "id": "msg_capability_01",
			  "kind": "capability",
			  "channel": "builders",
			  "surface": "thread",
			  "thread_id": "thread_patch_42",
			  "from": "curator.sess-123",
			  "to": null,
			  "work_id": "work_capability_01",
			  "ts": 1775822400,
			  "body": {
			    "capability": {
			      "id": "review-fix",
			      "summary": "Review Fix Flow",
			      "outcome": "A reusable review fix workflow.",
			      "version": "1.0.0",
			      "digest": "sha256:edb42ce4ca23d905aeeba399001ca6d23420610775ba41ef66e90f47f14dba0d",
			      "execution_outline": ["Inspect the issue", "Draft the fix"],
			      "requirements": ["workspace-write"]
			    }
			  },
			  "proof": null
			}`),
		},
		{
			name: "receipt",
			raw: []byte(`{
			  "protocol": "agh-network/v0",
			  "workspace_id": "wks_test",
			  "id": "msg_receipt_01",
			  "kind": "receipt",
			  "channel": "builders",
			  "surface": "direct",
			  "direct_id": "direct_0123456789abcdef0123456789abcdef",
			  "from": "reviewer.sess-xyz",
			  "to": "coder.sess-abc",
			  "work_id": "work_patch_42",
			  "reply_to": "msg_direct_01",
			  "ts": 1775822400,
			  "body": {
			    "for_id": "msg_direct_01",
			    "status": "accepted",
			    "detail": "Proceed and report progress with trace messages."
			  }
			}`),
		},
		{
			name: "trace",
			raw: []byte(`{
			  "protocol": "agh-network/v0",
			  "workspace_id": "wks_test",
			  "id": "msg_trace_01",
			  "kind": "trace",
			  "channel": "builders",
			  "surface": "direct",
			  "direct_id": "direct_0123456789abcdef0123456789abcdef",
			  "from": "reviewer.sess-xyz",
			  "to": "coder.sess-abc",
			  "work_id": "work_patch_42",
			  "reply_to": "msg_receipt_01",
			  "trace_id": "trace_ops_patch_42",
			  "causation_id": "msg_receipt_01",
			  "ts": 1775822400,
			  "body": {
			    "state": "working",
			    "message": "Started inspecting auth.go"
			  }
			}`),
		},
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			env, err := ParseEnvelope(fixture.raw, opts)
			if err != nil {
				t.Fatalf("ParseEnvelope() error = %v", err)
			}

			firstBody, err := env.DecodeBody()
			if err != nil {
				t.Fatalf("DecodeBody() error = %v", err)
			}

			encoded, err := json.Marshal(env)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			roundTrip, err := ParseEnvelope(encoded, opts)
			if err != nil {
				t.Fatalf("ParseEnvelope(round trip) error = %v", err)
			}

			secondBody, err := roundTrip.DecodeBody()
			if err != nil {
				t.Fatalf("DecodeBody(round trip) error = %v", err)
			}

			if !reflect.DeepEqual(envelopeSnapshot(env), envelopeSnapshot(roundTrip)) {
				t.Fatalf(
					"round-trip envelope mismatch = %#v, want %#v",
					envelopeSnapshot(roundTrip),
					envelopeSnapshot(env),
				)
			}
			if !reflect.DeepEqual(firstBody, secondBody) {
				t.Fatalf("round-trip body mismatch = %#v, want %#v", secondBody, firstBody)
			}
		})
	}
}

func envelopeSnapshot(env Envelope) map[string]any {
	snapshot := map[string]any{
		"protocol":     env.Protocol,
		"workspace_id": env.WorkspaceID,
		"id":           env.ID,
		"kind":         env.Kind,
		"channel":      env.Channel,
		"from":         env.From,
		"ts":           env.TS,
	}
	if env.To != nil {
		snapshot["to"] = *env.To
	}
	if env.Surface != nil {
		snapshot["surface"] = *env.Surface
	}
	if env.ThreadID != nil {
		snapshot["thread_id"] = *env.ThreadID
	}
	if env.DirectID != nil {
		snapshot["direct_id"] = *env.DirectID
	}
	if env.WorkID != nil {
		snapshot["work_id"] = *env.WorkID
	}
	if env.ReplyTo != nil {
		snapshot["reply_to"] = *env.ReplyTo
	}
	if env.TraceID != nil {
		snapshot["trace_id"] = *env.TraceID
	}
	if env.CausationID != nil {
		snapshot["causation_id"] = *env.CausationID
	}
	if env.ExpiresAt != nil {
		snapshot["expires_at"] = *env.ExpiresAt
	}
	snapshot["proof"] = proofSnapshot(env.Proof)
	snapshot["ext"] = extSnapshot(env.Ext)
	return snapshot
}

func proofSnapshot(proof *Proof) map[string]any {
	if proof == nil || len(*proof) == 0 {
		return map[string]any{}
	}

	snapshot := make(map[string]any, len(*proof))
	for key, value := range *proof {
		var decoded any
		if err := json.Unmarshal(value, &decoded); err != nil {
			snapshot[key] = string(value)
			continue
		}
		snapshot[key] = decoded
	}
	return snapshot
}

package notifications

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/diagnostics"
)

func TestEncodeDeliveryID(t *testing.T) {
	t.Parallel()

	t.Run("Should deterministically encode a versioned opaque identity", func(t *testing.T) {
		t.Parallel()

		identity := notificationDeliveryIdentityForTest()
		first, err := EncodeDeliveryID(identity)
		if err != nil {
			t.Fatalf("EncodeDeliveryID(first) error = %v", err)
		}
		second, err := EncodeDeliveryID(identity)
		if err != nil {
			t.Fatalf("EncodeDeliveryID(second) error = %v", err)
		}
		if first != second {
			t.Fatalf("deterministic delivery IDs = %q, %q, want equal", first, second)
		}
		if !strings.HasPrefix(first, deliveryIDPrefix) {
			t.Fatalf("delivery ID = %q, want %q version prefix", first, deliveryIDPrefix)
		}
		if strings.Contains(first, ":") {
			t.Fatalf("delivery ID = %q, want opaque non-delimited encoding", first)
		}
	})

	t.Run("Should reject invalid UTF-8 before JSON could collapse distinct identities", func(t *testing.T) {
		t.Parallel()

		invalidUTF8 := string([]byte{0xff})
		for _, test := range []struct {
			name     string
			identity DeliveryIdentity
		}{
			{
				name: "cursor consumer",
				identity: DeliveryIdentity{
					Cursor: CursorKey{
						Scope:      ScopeRef{Kind: ScopeKindGlobal},
						ConsumerID: invalidUTF8,
						StreamName: "task.run_completed",
						SubjectID:  "evt-1",
					},
					TargetIdentity: "target-1",
					Sequence:       1,
					Kind:           DeliveryKindDeliver,
				},
			},
			{
				name: "workspace id",
				identity: DeliveryIdentity{
					Cursor: CursorKey{
						Scope: ScopeRef{
							Kind:        ScopeKindWorkspace,
							WorkspaceID: invalidUTF8,
						},
						ConsumerID: "consumer-1",
						StreamName: "task.run_completed",
						SubjectID:  "evt-1",
					},
					TargetIdentity: "target-1",
					Sequence:       1,
					Kind:           DeliveryKindDeliver,
				},
			},
			{
				name: "target identity",
				identity: DeliveryIdentity{
					Cursor:         notificationCursorKeyForDeliveryTest(),
					TargetIdentity: invalidUTF8,
					Sequence:       1,
					Kind:           DeliveryKindDeliver,
				},
			},
			{
				name: "skip reason",
				identity: DeliveryIdentity{
					Cursor:         notificationCursorKeyForDeliveryTest(),
					TargetIdentity: "target-1",
					Sequence:       1,
					Kind:           DeliveryKindSkipped,
					Reason:         invalidUTF8,
				},
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if _, err := EncodeDeliveryID(test.identity); !errors.Is(err, ErrInvalidCursor) {
					t.Fatalf("EncodeDeliveryID() error = %v, want ErrInvalidCursor", err)
				}
			})
		}
	})

	t.Run("Should preserve valid opaque identity bytes that differ only by whitespace", func(t *testing.T) {
		t.Parallel()

		base := notificationDeliveryIdentityForTest()
		variants := []struct {
			name   string
			mutate func(*DeliveryIdentity)
		}{
			{
				name: "Should preserve workspace ID whitespace",
				mutate: func(identity *DeliveryIdentity) {
					identity.Cursor.Scope.WorkspaceID = " global"
				},
			},
			{
				name: "Should preserve consumer ID whitespace",
				mutate: func(identity *DeliveryIdentity) {
					identity.Cursor.ConsumerID = " consumer:terminal"
				},
			},
			{
				name: "Should preserve stream name whitespace",
				mutate: func(identity *DeliveryIdentity) {
					identity.Cursor.StreamName = " task.run_completed"
				},
			},
			{
				name: "Should preserve subject ID whitespace",
				mutate: func(identity *DeliveryIdentity) {
					identity.Cursor.SubjectID = " evt-terminal"
				},
			},
			{
				name: "Should preserve target identity whitespace",
				mutate: func(identity *DeliveryIdentity) {
					identity.TargetIdentity = " preset-target-1"
				},
			},
		}

		baseline, err := EncodeDeliveryID(base)
		if err != nil {
			t.Fatalf("EncodeDeliveryID(baseline) error = %v", err)
		}
		for _, variant := range variants {
			t.Run(variant.name, func(t *testing.T) {
				t.Parallel()

				identity := base
				variant.mutate(&identity)
				got, err := EncodeDeliveryID(identity)
				if err != nil {
					t.Fatalf("EncodeDeliveryID() error = %v", err)
				}
				if got == baseline {
					t.Fatalf(
						"EncodeDeliveryID() = %q, want identity distinct from %q",
						got,
						baseline,
					)
				}
			})
		}
	})

	t.Run("Should reject noncanonical scope kinds", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []ScopeKind{" global", "Global", "workspace "} {
			identity := notificationDeliveryIdentityForTest()
			identity.Cursor.Scope.Kind = kind
			if _, err := EncodeDeliveryID(identity); !errors.Is(err, ErrInvalidCursor) {
				t.Fatalf("EncodeDeliveryID(%q) error = %v, want ErrInvalidCursor", kind, err)
			}
		}
	})
}

func TestCursorQueryNormalize(t *testing.T) {
	t.Parallel()

	t.Run("Should allow an omitted scope only for an aggregate administrative list", func(t *testing.T) {
		t.Parallel()

		normalized, err := (CursorQuery{}).Normalize()
		if err != nil {
			t.Fatalf("CursorQuery{}.Normalize() error = %v", err)
		}
		if normalized.Scope != (ScopeRef{}) {
			t.Fatalf("aggregate CursorQuery scope = %#v, want omitted scope", normalized.Scope)
		}
	})

	t.Run("Should reject a workspace filter without its ownership kind", func(t *testing.T) {
		t.Parallel()

		_, err := (CursorQuery{Scope: ScopeRef{WorkspaceID: "ws-1"}}).Normalize()
		if !errors.Is(err, ErrInvalidCursor) {
			t.Fatalf("CursorQuery.Normalize() error = %v, want ErrInvalidCursor", err)
		}
	})
}

func TestCursorDeliveryIdentifiersNormalize(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve opaque acknowledgement delivery IDs", func(t *testing.T) {
		t.Parallel()

		advance, err := (AdvanceCursor{
			Key:          notificationCursorKeyForDeliveryTest(),
			LastSequence: 7,
			DeliveryID:   " delivery-ack ",
		}).Normalize(notificationTestNow())
		if err != nil {
			t.Fatalf("AdvanceCursor.Normalize() error = %v", err)
		}
		if got, want := advance.DeliveryID, " delivery-ack "; got != want {
			t.Fatalf("AdvanceCursor.Normalize() delivery ID = %q, want %q", got, want)
		}

		reset, err := (ResetCursor{
			Key:            notificationCursorKeyForDeliveryTest(),
			LastDeliveryID: " delivery-ack ",
			Reason:         "operator recovery",
		}).Normalize(notificationTestNow())
		if err != nil {
			t.Fatalf("ResetCursor.Normalize() error = %v", err)
		}
		if got, want := reset.LastDeliveryID, " delivery-ack "; got != want {
			t.Fatalf("ResetCursor.Normalize() last delivery ID = %q, want %q", got, want)
		}
	})
}

func TestCursorErrorNormalize(t *testing.T) {
	t.Parallel()

	t.Run("Should redact, bound, and preserve valid UTF-8 diagnostics", func(t *testing.T) {
		t.Parallel()

		const wantCursorErrorMaxBytes = 2048
		const dynamicSecret = "runtime-notification-secret"

		release := diagnostics.RegisterDynamicSecret(dynamicSecret)
		t.Cleanup(release)

		lastError := strings.Join([]string{
			"dynamic=" + dynamicSecret,
			"authorization=Bearer bearer-notification-secret",
			"claim=compozy_claim_notification-secret",
			"oauth_code=oauth-notification-secret",
			"code_verifier=pkce-notification-secret",
		}, " ") + string([]byte{0xff}) + strings.Repeat("é", 2048)

		normalized, err := (CursorError{
			Key:       notificationCursorKeyForDeliveryTest(),
			LastError: lastError,
		}).Normalize(notificationTestNow())
		if err != nil {
			t.Fatalf("CursorError.Normalize() error = %v", err)
		}
		if !utf8.ValidString(normalized.LastError) {
			t.Fatalf("CursorError.Normalize() last error = %q, want valid UTF-8", normalized.LastError)
		}
		if got, limit := len(normalized.LastError), wantCursorErrorMaxBytes; got > limit {
			t.Fatalf("len(CursorError.Normalize().LastError) = %d, want <= %d", got, limit)
		}
		for _, secret := range []string{
			dynamicSecret,
			"bearer-notification-secret",
			"compozy_claim_notification-secret",
			"oauth-notification-secret",
			"pkce-notification-secret",
		} {
			if strings.Contains(normalized.LastError, secret) {
				t.Fatalf("CursorError.Normalize() last error = %q, leaked %q", normalized.LastError, secret)
			}
		}
	})
}

func notificationDeliveryIdentityForTest() DeliveryIdentity {
	return DeliveryIdentity{
		Cursor:         notificationCursorKeyForDeliveryTest(),
		TargetIdentity: "preset-target-1",
		TargetIndex:    0,
		Sequence:       7,
		Kind:           DeliveryKindDeliver,
	}
}

func notificationCursorKeyForDeliveryTest() CursorKey {
	return CursorKey{
		Scope:      ScopeRef{Kind: ScopeKindWorkspace, WorkspaceID: "global"},
		ConsumerID: "consumer:terminal",
		StreamName: "task.run_completed",
		SubjectID:  "evt:terminal",
	}
}

func notificationTestNow() time.Time {
	return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
}

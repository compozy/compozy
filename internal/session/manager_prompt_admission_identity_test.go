package session

import (
	"testing"
)

func TestPromptAdmissionFingerprintAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should change the v3 fingerprint when the attachment set changes", func(t *testing.T) {
		t.Parallel()

		base := promptRequest{messageID: "msg-fingerprint", authoredMessage: "look at this"}
		one := base
		one.attachments = []AttachmentMeta{{
			ID:     "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			SHA256: "digest-a",
		}}
		two := base
		two.attachments = []AttachmentMeta{
			{ID: "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SHA256: "digest-a"},
			{ID: "att_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", SHA256: "digest-b"},
		}
		first, err := promptAdmissionFingerprint(storePromptOperation(), BusyInputModeQueue, one)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(one) error = %v", err)
		}
		second, err := promptAdmissionFingerprint(storePromptOperation(), BusyInputModeQueue, two)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(two) error = %v", err)
		}
		if first == second {
			t.Fatal("fingerprint stayed stable after the attachment set changed")
		}
	})

	t.Run("Should stay stable across attachment digest order", func(t *testing.T) {
		t.Parallel()

		base := promptRequest{messageID: "msg-order", authoredMessage: "look at these"}
		forward := base
		forward.attachments = []AttachmentMeta{
			{SHA256: "digest-b"},
			{SHA256: "digest-a"},
		}
		reverse := base
		reverse.attachments = []AttachmentMeta{
			{SHA256: "digest-a"},
			{SHA256: "digest-b"},
		}
		first, err := promptAdmissionFingerprint(storePromptOperation(), BusyInputModeQueue, forward)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(forward) error = %v", err)
		}
		second, err := promptAdmissionFingerprint(storePromptOperation(), BusyInputModeQueue, reverse)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(reverse) error = %v", err)
		}
		if first != second {
			t.Fatalf("fingerprint = %q vs %q, want order-stable v3 digest", first, second)
		}
	})
}

func storePromptOperation() string {
	return "prompt"
}

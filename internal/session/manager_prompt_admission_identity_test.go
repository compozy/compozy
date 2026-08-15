package session

import (
	"strings"
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
			t.Fatalf("fingerprint = %q vs %q, want order-stable v4 attachment identity", first, second)
		}
	})

	t.Run("Should stay stable when canonical metadata enriches an attachment ID", func(t *testing.T) {
		t.Parallel()

		attachmentID := "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		raw := promptRequest{
			messageID: "msg-canonical",
			attachments: []AttachmentMeta{{
				ID: attachmentID,
			}},
		}
		canonical := raw
		canonical.attachments = []AttachmentMeta{{
			ID: attachmentID, SHA256: strings.Repeat("a", 64), Name: "notes.txt", MIMEType: "text/plain",
		}}
		before, err := promptAdmissionFingerprint(storePromptOperation(), BusyInputModeQueue, raw)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(raw) error = %v", err)
		}
		after, err := promptAdmissionFingerprint(storePromptOperation(), BusyInputModeQueue, canonical)
		if err != nil {
			t.Fatalf("promptAdmissionFingerprint(canonical) error = %v", err)
		}
		if before != after {
			t.Fatalf("fingerprint = %q before canonicalization, %q after", before, after)
		}
	})
}

func storePromptOperation() string {
	return "prompt"
}

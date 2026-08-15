package inputqueue

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/store"
)

const testQueueAttachmentID = "att_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestServiceNewInsertAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should accept empty text when at least one attachment is present", func(t *testing.T) {
		t.Parallel()

		service := newTestQueueService(t)
		insert, err := service.newInsert(insertSpec{
			sessionID:   "sess-queue-attachments",
			mode:        store.SessionInputQueueModeQueue,
			delivery:    store.SessionInputDeliveryAfterTurn,
			attachments: []store.SessionInputAttachment{testQueueAttachment()},
		})
		if err != nil {
			t.Fatalf("newInsert() error = %v", err)
		}
		if insert.Text != "" {
			t.Fatalf("Text = %q, want empty", insert.Text)
		}
		if len(insert.Attachments) != 1 || insert.Attachments[0].ID != testQueueAttachmentID {
			t.Fatalf("Attachments = %#v, want the image-only snapshot", insert.Attachments)
		}
	})

	t.Run("Should reject empty text when attachments are absent", func(t *testing.T) {
		t.Parallel()

		service := newTestQueueService(t)
		_, err := service.newInsert(insertSpec{
			sessionID: "sess-queue-empty",
			mode:      store.SessionInputQueueModeQueue,
			delivery:  store.SessionInputDeliveryAfterTurn,
		})
		if err == nil || !strings.Contains(err.Error(), "text is required") {
			t.Fatalf("newInsert() error = %v, want text is required", err)
		}
	})

	t.Run("Should reject empty steer text even when attachments are present", func(t *testing.T) {
		t.Parallel()

		service := newTestQueueService(t)
		_, err := service.newInsert(insertSpec{
			sessionID:    "sess-queue-steer",
			mode:         store.SessionInputQueueModeSteer,
			delivery:     store.SessionInputDeliveryInterruptThenPrompt,
			targetTurnID: "turn-active",
			attachments:  []store.SessionInputAttachment{testQueueAttachment()},
		})
		if err == nil || !strings.Contains(err.Error(), "text is required") {
			t.Fatalf("newInsert() error = %v, want text is required", err)
		}
	})
}

func TestSameMutationAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should treat equal attachment snapshots as the same mutation", func(t *testing.T) {
		t.Parallel()

		attachment := testQueueAttachment()
		entry := store.SessionInputQueueEntry{
			Text:           "queued",
			MessageID:      "msg-1",
			IdempotencyKey: "idem-1",
			Mode:           store.SessionInputQueueModeQueue,
			Delivery:       store.SessionInputDeliveryAfterTurn,
			Attachments:    []store.SessionInputAttachment{attachment},
		}
		if !sameMutation(
			&entry,
			"queued",
			"msg-1",
			"idem-1",
			store.SessionInputQueueModeQueue,
			store.SessionInputDeliveryAfterTurn,
			"",
			nil,
			[]store.SessionInputAttachment{attachment},
		) {
			t.Fatal("sameMutation() = false, want true for identical attachments")
		}
	})

	t.Run("Should skip attachment comparison when the mutation does not assert attachments", func(t *testing.T) {
		t.Parallel()

		entry := store.SessionInputQueueEntry{
			Text:           "queued",
			MessageID:      "msg-1",
			IdempotencyKey: "idem-1",
			Mode:           store.SessionInputQueueModeQueue,
			Delivery:       store.SessionInputDeliveryAfterTurn,
			Attachments:    []store.SessionInputAttachment{testQueueAttachment()},
		}
		if !sameMutation(
			&entry,
			"queued",
			"msg-1",
			"idem-1",
			store.SessionInputQueueModeQueue,
			store.SessionInputDeliveryAfterTurn,
			"",
			nil,
			nil,
		) {
			t.Fatal("sameMutation() = false, want true when attachments are not asserted")
		}
	})

	t.Run("Should treat a changed attachment set as a different mutation", func(t *testing.T) {
		t.Parallel()

		entry := store.SessionInputQueueEntry{
			Text:           "queued",
			MessageID:      "msg-1",
			IdempotencyKey: "idem-1",
			Mode:           store.SessionInputQueueModeQueue,
			Delivery:       store.SessionInputDeliveryAfterTurn,
			Attachments:    []store.SessionInputAttachment{testQueueAttachment()},
		}
		changed := testQueueAttachment()
		changed.SHA256 = "different-digest"
		if sameMutation(
			&entry,
			"queued",
			"msg-1",
			"idem-1",
			store.SessionInputQueueModeQueue,
			store.SessionInputDeliveryAfterTurn,
			"",
			nil,
			[]store.SessionInputAttachment{changed},
		) {
			t.Fatal("sameMutation() = true, want false after the attachment digest changed")
		}
	})
}

func TestServiceMutateAttachments(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve source attachments when a replace does not assert them", func(t *testing.T) {
		t.Parallel()

		source := store.SessionInputQueueEntry{
			ID:          "inq-source",
			SessionID:   "sess-mutate",
			Text:        "queued",
			Mode:        store.SessionInputQueueModeQueue,
			Delivery:    store.SessionInputDeliveryAfterTurn,
			Status:      store.SessionInputQueueStatusQueued,
			Attachments: []store.SessionInputAttachment{testQueueAttachment()},
		}
		queueStore := newMutationQueueStore(source)
		service := newTestMutationService(t, queueStore)
		entry, _, err := service.Replace(
			context.Background(),
			"sess-mutate",
			"inq-source",
			"edited text",
			"msg-replace",
			"idem-replace",
			nil,
		)
		if err != nil {
			t.Fatalf("Replace() error = %v", err)
		}
		if queueStore.replaced == nil {
			t.Fatal("ReplaceSessionInput was not called")
		}
		if len(queueStore.replaced.Attachments) != 1 ||
			queueStore.replaced.Attachments[0].ID != testQueueAttachmentID {
			t.Fatalf("replacement attachments = %#v, want the source snapshot", queueStore.replaced.Attachments)
		}
		if len(entry.Attachments) != 1 {
			t.Fatalf("entry attachments = %#v, want the preserved snapshot", entry.Attachments)
		}
	})

	t.Run("Should refuse promoting attachment-bearing input to steer", func(t *testing.T) {
		t.Parallel()

		source := store.SessionInputQueueEntry{
			ID:          "inq-source",
			SessionID:   "sess-mutate",
			Text:        "queued",
			Mode:        store.SessionInputQueueModeQueue,
			Delivery:    store.SessionInputDeliveryAfterTurn,
			Status:      store.SessionInputQueueStatusQueued,
			Attachments: []store.SessionInputAttachment{testQueueAttachment()},
		}
		queueStore := newMutationQueueStore(source)
		service := newTestMutationService(t, queueStore)
		_, _, err := service.PromoteToSteer(
			context.Background(),
			"sess-mutate",
			"inq-source",
			"steer now",
			"turn-1",
			"msg-promote",
			"idem-promote",
			nil,
		)
		if !errors.Is(err, store.ErrSessionInputSteerTextOnly) {
			t.Fatalf("PromoteToSteer() error = %v, want ErrSessionInputSteerTextOnly", err)
		}
		if queueStore.promoted {
			t.Fatal("PromoteSessionInputToSteer must not run for attachment-bearing input")
		}
	})
}

type mutationQueueStore struct {
	recordingQueueStore
	entries  map[string]store.SessionInputQueueEntry
	replaced *store.SessionInputQueueInsert
	promoted bool
}

func newMutationQueueStore(entries ...store.SessionInputQueueEntry) *mutationQueueStore {
	indexed := make(map[string]store.SessionInputQueueEntry, len(entries))
	for _, entry := range entries {
		indexed[entry.ID] = entry
	}
	return &mutationQueueStore{entries: indexed}
}

func (s *mutationQueueStore) GetSessionInputQueueEntry(
	_ context.Context,
	_ string,
	entryID string,
) (store.SessionInputQueueEntry, error) {
	entry, ok := s.entries[entryID]
	if !ok {
		return store.SessionInputQueueEntry{}, store.ErrSessionInputQueueEntryNotFound
	}
	return entry, nil
}

func (s *mutationQueueStore) ReplaceSessionInput(
	_ context.Context,
	_ string,
	_ string,
	replacement store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, bool, error) {
	s.replaced = &replacement
	return store.SessionInputQueueEntry{
		ID:          replacement.ID,
		SessionID:   replacement.SessionID,
		Text:        replacement.Text,
		Attachments: replacement.Attachments,
	}, true, nil
}

func (s *mutationQueueStore) PromoteSessionInputToSteer(
	_ context.Context,
	_ string,
	_ string,
	replacement store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, bool, error) {
	s.promoted = true
	return store.SessionInputQueueEntry{ID: replacement.ID}, true, nil
}

var _ store.SessionInputQueueStore = (*mutationQueueStore)(nil)

func newTestMutationService(t *testing.T, queueStore store.SessionInputQueueStore) *Service {
	t.Helper()

	service, err := New(queueStore, Config{QueueCap: 10, MaxTextBytes: 64 << 10})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return service
}

func newTestQueueService(t *testing.T) *Service {
	t.Helper()

	service, err := New(&recordingQueueStore{}, Config{QueueCap: 10, MaxTextBytes: 64 << 10})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return service
}

func testQueueAttachment() store.SessionInputAttachment {
	return store.SessionInputAttachment{
		ID:       testQueueAttachmentID,
		Name:     "shot.png",
		MIMEType: "image/png",
		Bytes:    1024,
		SHA256:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Kind:     store.SessionInputAttachmentKindImage,
	}
}

type recordingQueueStore struct{}

func (s *recordingQueueStore) EnqueueSessionInput(
	_ context.Context,
	req store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, int, error) {
	return store.SessionInputQueueEntry{ID: req.ID, SessionID: req.SessionID, Attachments: req.Attachments}, 1, nil
}

func (s *recordingQueueStore) EnqueueInterruptSessionInput(
	context.Context,
	store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, int, error) {
	return store.SessionInputQueueEntry{}, 0, unusedQueueStoreError()
}

func (s *recordingQueueStore) StageSessionSteer(
	context.Context,
	store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, error) {
	return store.SessionInputQueueEntry{}, unusedQueueStoreError()
}

func (s *recordingQueueStore) ListPendingSessionInputs(
	context.Context,
	string,
) ([]store.SessionInputQueueEntry, error) {
	return nil, unusedQueueStoreError()
}

func (s *recordingQueueStore) ReplaceSessionInput(
	context.Context,
	string,
	string,
	store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, bool, error) {
	return store.SessionInputQueueEntry{}, false, unusedQueueStoreError()
}

func (s *recordingQueueStore) PromoteSessionInputToSteer(
	context.Context,
	string,
	string,
	store.SessionInputQueueInsert,
) (store.SessionInputQueueEntry, bool, error) {
	return store.SessionInputQueueEntry{}, false, unusedQueueStoreError()
}

func (s *recordingQueueStore) ClaimNextSessionInput(
	context.Context,
	string,
	time.Time,
) (store.SessionInputQueueEntry, bool, error) {
	return store.SessionInputQueueEntry{}, false, unusedQueueStoreError()
}

func (s *recordingQueueStore) PeekNextSessionInput(
	context.Context,
	string,
) (store.SessionInputQueueEntry, bool, error) {
	return store.SessionInputQueueEntry{}, false, unusedQueueStoreError()
}

func (s *recordingQueueStore) GetSessionInputQueueEntry(
	context.Context,
	string,
	string,
) (store.SessionInputQueueEntry, error) {
	return store.SessionInputQueueEntry{}, unusedQueueStoreError()
}

func (s *recordingQueueStore) MarkSessionInputSent(context.Context, string, string, time.Time) error {
	return unusedQueueStoreError()
}

func (s *recordingQueueStore) ReleaseSessionInput(context.Context, string, string, time.Time) error {
	return unusedQueueStoreError()
}

func (s *recordingQueueStore) MarkSessionInputFailed(context.Context, string, string, string, time.Time) error {
	return unusedQueueStoreError()
}

func (s *recordingQueueStore) CancelSessionInput(
	context.Context,
	string,
	string,
	time.Time,
) (store.SessionInputQueueEntry, error) {
	return store.SessionInputQueueEntry{}, unusedQueueStoreError()
}

func (s *recordingQueueStore) CancelPendingSessionInputs(
	context.Context,
	string,
	int64,
	time.Time,
) (int, error) {
	return 0, unusedQueueStoreError()
}

func (s *recordingQueueStore) AdvanceSessionInputGeneration(context.Context, string, time.Time) (int64, error) {
	return 0, unusedQueueStoreError()
}

func (s *recordingQueueStore) CurrentSessionInputGeneration(context.Context, string) (int64, error) {
	return 0, unusedQueueStoreError()
}

func (s *recordingQueueStore) SessionInputQueueSummary(
	context.Context,
	string,
) (store.SessionInputQueueSummary, error) {
	return store.SessionInputQueueSummary{}, unusedQueueStoreError()
}

func unusedQueueStoreError() error {
	return errors.New("inputqueue: unused store method")
}

var _ store.SessionInputQueueStore = (*recordingQueueStore)(nil)

package daemon

import (
	"context"
	"fmt"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	"github.com/compozy/compozy/internal/store"
)

type sessionAttachmentRetentionPins struct {
	queue sessionInputAttachmentPinStore
}

type sessionInputAttachmentPinStore interface {
	ListPendingSessionInputs(ctx context.Context, sessionID string) ([]store.SessionInputQueueEntry, error)
}

func (p sessionAttachmentRetentionPins) PinnedAttachmentIDs(
	ctx context.Context,
	sessionID string,
) (map[string]struct{}, error) {
	entries, err := p.queue.ListPendingSessionInputs(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	pins := make(map[string]struct{})
	for _, entry := range entries {
		for _, attachment := range entry.Attachments {
			pins[attachment.ID] = struct{}{}
		}
	}
	return pins, nil
}

func (d *Daemon) bootSessionAttachments(
	ctx context.Context,
	state *bootState,
	cleanup *bootCleanup,
) error {
	retention := attachmentspkg.AttachmentRetention{
		MaxCount: state.cfg.Session.Attachments.Retention.MaxCount,
		MaxBytes: state.cfg.Session.Attachments.Retention.MaxBytes,
		MaxAge:   state.cfg.Session.Attachments.Retention.MaxAge,
	}
	limits := attachmentspkg.StoreLimits{
		MaxFileBytes: state.cfg.Session.Attachments.MaxFileBytes,
		AllowedMIME:  state.cfg.Session.Attachments.AllowedMIME,
	}
	var retentionPins attachmentspkg.RetentionPinSource
	if queueStore := sessionInputAttachmentPinStoreDependency(state.registry); queueStore != nil {
		retentionPins = sessionAttachmentRetentionPins{queue: queueStore}
	}
	attachmentStore, err := attachmentspkg.OpenFilesystemAttachmentStore(ctx, attachmentspkg.FilesystemStoreConfig{
		Root:          d.homePaths.SessionAttachmentsDir,
		Retention:     retention,
		Limits:        limits,
		RetentionPins: retentionPins,
	})
	if err != nil {
		return fmt.Errorf("daemon: open session attachment store: %w", err)
	}
	sweeper := attachmentspkg.NewSweeper(
		attachmentStore,
		attachmentspkg.SweepInterval,
		func(err error) {
			state.logger.Error("session attachment retention sweep failed", "error", err)
		},
	)
	if err := sweeper.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start session attachment retention: %w", err)
	}
	cleanup.add(sweeper.Shutdown)
	state.sessionAttachments = attachmentStore
	state.runtimeWorkers.sessionAttachments = sweeper
	return nil
}

func sessionInputAttachmentPinStoreDependency(registry Registry) sessionInputAttachmentPinStore {
	queueStore, ok := registry.(sessionInputAttachmentPinStore)
	if !ok {
		return nil
	}
	return queueStore
}

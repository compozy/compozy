package daemon

import (
	"context"
	"fmt"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
)

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
	store, err := attachmentspkg.OpenFilesystemAttachmentStore(
		ctx,
		d.homePaths.SessionAttachmentsDir,
		retention,
		limits,
	)
	if err != nil {
		return fmt.Errorf("daemon: open session attachment store: %w", err)
	}
	sweeper := attachmentspkg.NewSweeper(
		store,
		attachmentspkg.SweepInterval,
		func(err error) {
			state.logger.Error("session attachment retention sweep failed", "error", err)
		},
	)
	if err := sweeper.Start(ctx); err != nil {
		return fmt.Errorf("daemon: start session attachment retention: %w", err)
	}
	cleanup.add(sweeper.Shutdown)
	state.sessionAttachments = store
	state.deps.SessionAttachments = store
	state.runtimeWorkers.sessionAttachments = sweeper
	return nil
}

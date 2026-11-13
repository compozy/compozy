package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/observe/attention"
	"github.com/compozy/compozy/internal/windowmanager"
)

type WindowManagerMigrationNotifications interface {
	MigrationNotifications(context.Context, []windowmanager.WorkspaceID) ([]windowmanager.MigrationNotice, error)
	AcknowledgeMigrationNotification(context.Context, windowmanager.MigrationNotice) error
}

func migrationNotificationID(notice windowmanager.MigrationNotice) string {
	return attention.Identity("layout-upgrade", string(notice.WorkspaceID), notice.ProfileID,
		notice.Status.At.UTC().Format(time.RFC3339Nano))
}

func (h *BaseHandlers) migrationNotificationItems(
	ctx context.Context, labels map[string]string,
) ([]contract.AttentionNotificationPayload, error) {
	notices, err := h.migrationNotifications(ctx, labels)
	if err != nil {
		return nil, err
	}
	items := make([]contract.AttentionNotificationPayload, 0, len(notices))
	for _, notice := range notices {
		items = append(items, contract.AttentionNotificationPayload{
			ID: migrationNotificationID(notice), Kind: "layout-upgrade", SourceID: string(notice.WorkspaceID),
			WorkspaceID: string(notice.WorkspaceID), Title: "Your windows are now tabs",
			Detail: migrationNotificationDetail(notice.Status.Summary), OccurredAt: notice.Status.At, Finished: true,
		})
	}
	return items, nil
}

func (h *BaseHandlers) migrationNotifications(
	ctx context.Context, labels map[string]string,
) ([]windowmanager.MigrationNotice, error) {
	source, ok := h.WindowManager.(WindowManagerMigrationNotifications)
	if !ok {
		return nil, nil
	}
	workspaces := make([]windowmanager.WorkspaceID, 0, len(labels))
	for id := range labels {
		workspaces = append(workspaces, windowmanager.WorkspaceID(id))
	}
	notices, err := source.MigrationNotifications(ctx, workspaces)
	if err != nil {
		return nil, err
	}
	eligible := make([]windowmanager.MigrationNotice, 0, len(notices))
	for _, notice := range notices {
		if _, registered := labels[string(notice.WorkspaceID)]; registered &&
			notice.Status.State == "migrated" && !notice.Status.Acknowledged && !notice.Status.At.IsZero() {
			eligible = append(eligible, notice)
		}
	}
	return eligible, nil
}

func (h *BaseHandlers) acknowledgeMigrationNotifications(ctx context.Context, ids []string) error {
	source, ok := h.WindowManager.(WindowManagerMigrationNotifications)
	if !ok {
		return nil
	}
	labels, err := h.bellWorkspaceLabels(ctx)
	if err != nil {
		return err
	}
	notices, err := h.migrationNotifications(ctx, labels)
	if err != nil {
		return err
	}
	members := make(map[string]bool, len(ids))
	for _, id := range ids {
		members[id] = true
	}
	for _, notice := range notices {
		if members[migrationNotificationID(notice)] {
			if err := source.AcknowledgeMigrationNotification(ctx, notice); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrationNotificationDetail(summary *windowmanager.MigrationSummary) string {
	var floating, minimized, pins int
	if summary != nil {
		floating, minimized, pins = summary.FloatingToTabs, summary.MinimizedToTabs, summary.PinsDropped
	}
	moved := make([]string, 0, 2)
	for _, count := range []struct {
		number int
		kind   string
	}{{floating, "floating"}, {minimized, "minimized"}} {
		if count.number > 0 {
			noun := "window"
			if count.number != 1 {
				noun = "windows"
			}
			moved = append(moved, fmt.Sprintf("%d %s %s", count.number, count.kind, noun))
		}
	}
	detail := "Floating and minimized windows are tabs now."
	if len(moved) > 0 {
		verb := "are tabs"
		if floating+minimized == 1 {
			verb = "is a tab"
		}
		detail = strings.Join(moved, " and ") + " " + verb + " now."
	}
	if pins == 1 {
		detail += " 1 pin inside a split was removed."
	} else if pins > 1 {
		detail += fmt.Sprintf(" %d pins inside splits were removed.", pins)
	}
	return detail
}

func (h *BaseHandlers) bellWorkspaceLabels(ctx context.Context) (map[string]string, error) {
	workspaces, err := h.Workspaces.List(ctx)
	if err != nil {
		return nil, err
	}
	labels := make(map[string]string, len(workspaces))
	for _, workspace := range workspaces {
		labels[workspace.ID] = workspace.Name
	}
	return labels, nil
}

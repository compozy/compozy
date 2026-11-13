import { useMutation, useQueryClient } from "@tanstack/react-query";
import { use, useState } from "react";

import { notifyUser } from "@/lib/user-feedback";

import {
  updateWindowManagerMigration,
  WindowManagerApiError,
} from "../adapters/window-manager-api";
import { DeskProfileContext } from "../contexts/desk-profile-context";
import { windowManagerKeys } from "../lib/window-manager-query";
import type {
  WindowManagerMigrationStatus,
  WindowManagerMigrationSummary,
  WindowManagerSnapshot,
} from "../lib/window-manager-types";
import { useDesktop } from "./use-desktop";

/**
 * `layout-changed`: the conversion failed and the layout changed since, so the daemon refuses a
 * plain retry; only a confirmed replace can still convert the preserved layout.
 */
export type UpgradeNoticeState = "migrated" | "failed" | "layout-changed" | "retrying";

export interface UpgradeNoticeModel {
  state: UpgradeNoticeState;
  summary: WindowManagerMigrationSummary | null;
  /** Whether the "Replace current layout" confirmation is open. */
  confirmingReplace: boolean;
  dismiss: () => void;
  retry: () => void;
  requestReplace: () => void;
  cancelReplace: () => void;
  confirmReplace: () => void;
}

/** Whether a status still asks for the notice: a migration nobody has acknowledged yet. */
export function upgradeNoticeState(
  status: WindowManagerMigrationStatus | null | undefined
): "migrated" | "failed" | null {
  if (!status || status.acknowledged) return null;
  if (status.state === "migrated" || status.state === "failed") return status.state;
  return null;
}

/**
 * Whether the daemon would refuse a plain retry of this failed migration: the same comparison it
 * makes, the current revision against the revision the failure left (absent on the wire as 0).
 */
export function retryRefused(
  status: WindowManagerMigrationStatus,
  revision: number | null | undefined
): boolean {
  if (status.state !== "failed" || revision === null || revision === undefined) return false;
  return revision !== (status.failureRevision ?? 0);
}

/**
 * The one-time notice after the layout moved to tabs (US-017.AC-6, EC-6), read from the
 * snapshot's `migration`. Acknowledging or retrying reaches every client of the workspace and
 * profile as a `snapshot` frame (same revision for an ack or a failed retry), so a dismissal
 * elsewhere hides it here and a reload keeps it until then.
 */
export function useOsUpgradeNotice(): UpgradeNoticeModel | null {
  const queryClient = useQueryClient();
  const profile = use(DeskProfileContext) ?? "";
  const workspaceId = useDesktop(state => state.snapshot?.workspaceId ?? "");
  const migration = useDesktop(state => state.snapshot?.migration ?? null);
  const revision = useDesktop(state => state.snapshot?.revision ?? null);
  // A 409 that still arrives (another client changed the layout first) holds the replace offer
  // for the status it answered; a new status (`at` moves on every write) drops it.
  const [refusedAt, setRefusedAt] = useState<string | null>(null);
  const [confirmingReplace, setConfirmingReplace] = useState(false);
  // The route's answer settles the cached snapshot now, so the notice neither waits for the
  // frame nor comes back before it: an ack clears `migration`, a retry shows its outcome.
  const settle = (next: WindowManagerMigrationStatus) =>
    queryClient.setQueryData<WindowManagerSnapshot>(
      windowManagerKeys.snapshot(workspaceId, profile),
      current =>
        current && { ...current, migration: upgradeNoticeState(next) === null ? null : next }
    );
  const acknowledge = useMutation({
    mutationFn: () => updateWindowManagerMigration(workspaceId, profile, "ack"),
    onSuccess: settle,
    onError: () =>
      notifyUser({ message: "Couldn't dismiss the notice. Try again.", tone: "error" }),
  });
  const retry = useMutation({
    mutationFn: (replace: boolean) =>
      updateWindowManagerMigration(workspaceId, profile, "retry", { replace }),
    onSuccess: settle,
    onError: error => {
      if (error instanceof WindowManagerApiError && error.status === 409) {
        setRefusedAt(migration?.at ?? "");
        return;
      }
      notifyUser({ message: "Couldn't move your layout to tabs. Try again.", tone: "error" });
    },
  });
  const noticeState = upgradeNoticeState(migration);
  // Hidden as soon as it is dismissed here; a failed dismissal shows it again.
  if (migration === null || noticeState === null || acknowledge.isPending) return null;
  const refused =
    retryRefused(migration, revision) ||
    (noticeState === "failed" && refusedAt !== null && refusedAt === (migration.at ?? ""));
  const state: UpgradeNoticeState = retry.isPending
    ? "retrying"
    : refused
      ? "layout-changed"
      : noticeState;
  return {
    state,
    summary: migration.summary,
    confirmingReplace: confirmingReplace && state === "layout-changed",
    dismiss: () => acknowledge.mutate(),
    retry: () => retry.mutate(false),
    requestReplace: () => setConfirmingReplace(true),
    cancelReplace: () => setConfirmingReplace(false),
    confirmReplace: () => {
      setConfirmingReplace(false);
      retry.mutate(true);
    },
  };
}

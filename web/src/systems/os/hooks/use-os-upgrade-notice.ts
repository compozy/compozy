import { useMutation, useQueryClient } from "@tanstack/react-query";
import { use } from "react";

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

export type UpgradeNoticeState = "migrated" | "failed" | "retrying";

export interface UpgradeNoticeModel {
  state: UpgradeNoticeState;
  summary: WindowManagerMigrationSummary | null;
  dismiss: () => void;
  retry: () => void;
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
    mutationFn: () => updateWindowManagerMigration(workspaceId, profile, "retry"),
    onSuccess: settle,
    onError: error =>
      notifyUser({
        message:
          error instanceof WindowManagerApiError && error.status === 409
            ? "Your layout changed after the upgrade failed, so it was kept as it is."
            : "Couldn't move your layout to tabs. Try again.",
        tone: "error",
      }),
  });
  const state = upgradeNoticeState(migration);
  // Hidden as soon as it is dismissed here; a failed dismissal shows it again.
  if (state === null || acknowledge.isPending) return null;
  return {
    state: retry.isPending ? "retrying" : state,
    summary: migration?.summary ?? null,
    dismiss: () => acknowledge.mutate(),
    retry: () => retry.mutate(),
  };
}

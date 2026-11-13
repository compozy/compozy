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
  WindowManagerSnapshot,
} from "../lib/window-manager-types";
import { useDesktop } from "./use-desktop";

/**
 * The inline notice's states, all of a failed conversion; a successful one is announced in the
 * bell (`layout-upgrade`). `layout-changed`: the layout changed since the failure, so the daemon
 * refuses a plain retry; only a confirmed replace can still convert the preserved layout.
 */
export type UpgradeNoticeState = "failed" | "layout-changed" | "retrying";

export interface UpgradeNoticeModel {
  state: UpgradeNoticeState;
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

export interface UpgradeNoticeInputs {
  migration: WindowManagerMigrationStatus | null;
  /** The snapshot's revision, which a plain retry is checked against. */
  revision: number | null;
  /** A dismissal in flight hides the notice at once; a failed one shows it again. */
  acknowledging: boolean;
  retrying: boolean;
  /** The status `at` a refused (409) retry answered, if any. */
  refusedAt: string | null;
}

/** What the inline notice shows, or `null` when it shows nothing. */
export function deriveUpgradeNoticeState({
  migration,
  revision,
  acknowledging,
  retrying,
  refusedAt,
}: UpgradeNoticeInputs): UpgradeNoticeState | null {
  if (migration === null || upgradeNoticeState(migration) !== "failed" || acknowledging) {
    return null;
  }
  if (retrying) return "retrying";
  const refused =
    retryRefused(migration, revision) || (refusedAt !== null && refusedAt === (migration.at ?? ""));
  return refused ? "layout-changed" : "failed";
}

/**
 * The inline notice of a failed move to tabs (US-017.AC-6, EC-6), read from the
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
  // frame nor comes back before it: an ack clears `migration`, a retry shows its outcome (a
  // success leaves the inline notice for the bell).
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
    // `at` is the status the retry was sent for; a 409 answers that status, not a later one.
    mutationFn: ({ replace }: { replace: boolean; at: string }) =>
      updateWindowManagerMigration(workspaceId, profile, "retry", { replace }),
    onSuccess: settle,
    onError: (error, { at }) => {
      if (error instanceof WindowManagerApiError && error.status === 409) {
        setRefusedAt(at);
        return;
      }
      notifyUser({ message: "Couldn't move your layout to tabs. Try again.", tone: "error" });
    },
  });
  const state = deriveUpgradeNoticeState({
    migration,
    revision,
    acknowledging: acknowledge.isPending,
    retrying: retry.isPending,
    refusedAt,
  });
  // The confirmation belongs to the offer on screen: once the notice leaves it (another client
  // replaced the layout), a later return to it must not reopen the dialog unasked.
  const confirming = confirmingReplace && state === "layout-changed";
  if (confirmingReplace && !confirming) setConfirmingReplace(false);
  if (migration === null || state === null) return null;
  const at = migration.at ?? "";
  return {
    state,
    confirmingReplace: confirming,
    dismiss: () => acknowledge.mutate(),
    retry: () => retry.mutate({ replace: false, at }),
    requestReplace: () => setConfirmingReplace(true),
    cancelReplace: () => setConfirmingReplace(false),
    confirmReplace: () => {
      setConfirmingReplace(false);
      retry.mutate({ replace: true, at });
    },
  };
}

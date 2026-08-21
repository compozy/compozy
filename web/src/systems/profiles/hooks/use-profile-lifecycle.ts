import { useState } from "react";
import { useSelector } from "@xstate/store-react";

import { ProfileApiError } from "../adapters/profiles-api";
import { profileErrorMessage } from "../lib/profile-copy";
import { closeProfileDialog, profileDialogStore } from "../stores/profile-dialog-store";
import type { ProfileDialogIntent, UnarchiveProfileResult } from "../types";

/** The refusal message for a mutation error, preferring our typed wording. */
export function lifecycleErrorMessage(error: unknown): string | null {
  if (error === null || error === undefined) return null;
  if (error instanceof ProfileApiError) return profileErrorMessage(error.code, error.message);
  return error instanceof Error ? error.message : null;
}

/** A stale plan is the daemon telling us the world moved; re-read and re-ask. */
export function isStalePlan(error: unknown): boolean {
  return error instanceof ProfileApiError && error.code === "profile_plan_stale";
}

export interface ProfileLifecycleState {
  intent: ProfileDialogIntent | null;
  close: () => void;
  renameName: string;
  setRenameName: (next: string) => void;
  acceptedRepos: string[];
  toggleRepo: (workspaceId: string) => void;
  unarchiveResult: UnarchiveProfileResult | null;
  setUnarchiveResult: (result: UnarchiveProfileResult | null) => void;
}

/**
 * Transient state for whichever lifecycle dialog is open.
 *
 * The intent itself lives in a module store so the palette and Settings raise
 * the same dialog; everything here is per-open-instance and resets with it.
 */
export function useProfileLifecycle(): ProfileLifecycleState {
  const intent = useSelector(profileDialogStore, state => state.context.intent);
  const [renameName, setRenameName] = useState("");
  const [acceptedRepos, setAcceptedRepos] = useState<string[]>([]);
  const [unarchiveResult, setUnarchiveResult] = useState<UnarchiveProfileResult | null>(null);

  const close = () => {
    closeProfileDialog();
    setRenameName("");
    setAcceptedRepos([]);
    setUnarchiveResult(null);
  };

  const toggleRepo = (workspaceId: string) => {
    setAcceptedRepos(current =>
      current.includes(workspaceId)
        ? current.filter(id => id !== workspaceId)
        : [...current, workspaceId]
    );
  };

  return {
    intent,
    close,
    renameName,
    setRenameName,
    acceptedRepos,
    toggleRepo,
    unarchiveResult,
    setUnarchiveResult,
  };
}

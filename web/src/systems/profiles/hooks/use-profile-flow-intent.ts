import { useEffect, useEffectEvent } from "react";

import { openProfileDialog } from "../stores/profile-dialog-store";
import type { ProfileLifecycleFlow } from "../types";

const FLOWS: readonly ProfileLifecycleFlow[] = [
  "create",
  "update",
  "rename",
  "archive",
  "unarchive",
  "delete",
];

function asFlow(value: string): ProfileLifecycleFlow | null {
  return FLOWS.find(flow => flow === value) ?? null;
}

export interface ProfileFlowSearch {
  flow: string;
  profile?: string;
}

/**
 * Raises the dialog a palette command navigated here to open.
 *
 * The intent arrives on the window route, which is an external system, so this
 * is one of the narrow cases an effect is the right tool: it syncs a URL the
 * shell already navigated into the dialog store. An unknown flow is ignored
 * rather than guessed.
 */
export function useProfileFlowIntent(intent: ProfileFlowSearch | undefined): void {
  const raise = useEffectEvent((flow: ProfileLifecycleFlow, profile?: string) => {
    openProfileDialog({ flow, ...(profile ? { profile } : {}) });
  });
  const flow = intent?.flow ?? "";
  const profile = intent?.profile ?? "";

  useEffect(() => {
    const resolved = asFlow(flow);
    if (resolved === null) return;
    raise(resolved, profile === "" ? undefined : profile);
  }, [flow, profile]);
}

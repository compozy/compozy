import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";

import { useIntentPreload } from "@/hooks/use-intent-preload";
import { preloadOsAppCode } from "@/systems/os";

import { preloadSessionEntry } from "../lib/session-entry-preload";
import type { SessionPayload } from "../types";

/** Read-only intent shared by list rows and Kanban cards, without router navigation. */
export function useSessionEntryPreload(session: SessionPayload, enabled = true) {
  const queryClient = useQueryClient();
  const { id, workspace_id, profile_name } = session;
  const preload = useCallback(
    () =>
      Promise.all([
        preloadOsAppCode("session"),
        preloadSessionEntry(queryClient, { id, workspace_id, profile_name }),
      ]),
    [id, profile_name, queryClient, workspace_id]
  );
  return useIntentPreload(preload, enabled);
}

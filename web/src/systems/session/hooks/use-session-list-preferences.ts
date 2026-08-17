import { useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";

import {
  settingsKeys,
  useSettingsShell,
  useUpdateSettingsShell,
  type SettingsShellSection,
} from "@/systems/settings";

import {
  toSessionListScope,
  toSessionListSort,
  type SessionListPreferences,
  type SessionListScope,
  type SessionListSort,
} from "../lib/session-list-preferences";

export interface SessionListPreferencesModel extends SessionListPreferences {
  setSort: (sort: SessionListSort) => void;
  setScope: (scope: SessionListScope) => void;
  /** True while the daemon has not answered yet — controls stay usable on defaults. */
  loading: boolean;
  /** Full-section writes are serialized to prevent stale last-writer wins. */
  saving: boolean;
}

/**
 * The operator's session-list sort and scope, read from and written to the
 * daemon's `[shell.sessions]` section, so the choice survives a reload, a
 * restart, and a second client — and round-trips with
 * `compozy config get/set shell.sessions.*` like every other setting.
 *
 * Writes send the whole section, matching every other settings surface, and
 * apply live: there is no save bar for a preference the operator changes by
 * clicking the control they are already looking at. The cached envelope is
 * patched first so the control responds immediately; the mutation's own
 * invalidation is the reconcile, which doubles as the rollback if the daemon
 * rejects the write.
 *
 * Until the first read lands, the documented defaults render, so the list is
 * never blank waiting on a preference.
 */
export function useSessionListPreferences(): SessionListPreferencesModel {
  const queryClient = useQueryClient();
  const query = useSettingsShell();
  const mutation = useUpdateSettingsShell();
  const writePendingRef = useRef(false);
  const [writePending, setWritePending] = useState(false);
  const sessions = query.data?.config.sessions;
  const current: SessionListPreferences = {
    sort: toSessionListSort(sessions?.sort),
    scope: toSessionListScope(sessions?.scope),
  };
  const write = (next: SessionListPreferences) => {
    if (writePendingRef.current || mutation.isPending) return;
    writePendingRef.current = true;
    setWritePending(true);
    queryClient.setQueryData<SettingsShellSection>(settingsKeys.section("shell"), cached =>
      cached === undefined ? cached : { ...cached, config: { ...cached.config, sessions: next } }
    );
    mutation.mutate(
      { config: { sessions: next } },
      {
        onSettled: () => {
          writePendingRef.current = false;
          setWritePending(false);
        },
      }
    );
  };

  return {
    ...current,
    setSort: sort => write({ ...current, sort }),
    setScope: scope => write({ ...current, scope }),
    loading: query.isLoading,
    saving: mutation.isPending || writePending,
  };
}

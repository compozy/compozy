import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import {
  settingsKeys,
  useSettingsShell,
  useUpdateSettingsShell,
  type SettingsShellSection,
} from "@/systems/settings";
import { observeLegacySessionsScope } from "@/systems/workspace";

import {
  toSessionListGroupBy,
  toSessionListSort,
  type SessionListGroupBy,
  type SessionListPreferences,
  type SessionListSort,
} from "../lib/session-list-preferences";

export interface SessionListPreferencesModel extends SessionListPreferences {
  setSort: (sort: SessionListSort) => void;
  setGroupBy: (groupBy: SessionListGroupBy) => void;
  /** True while the daemon has not answered yet — controls stay usable on defaults. */
  loading: boolean;
  /** Full-section writes are serialized to prevent stale last-writer wins. */
  saving: boolean;
}

/**
 * The operator's session-list sort and grouping, read from and written to the
 * daemon's `[shell.sessions]` section, so the choice survives a reload, a
 * restart, and a second client — and round-trips with
 * `compozy config get/set shell.sessions.*` like every other setting.
 *
 * Writes send `sort` and `group_by` and apply live: there is no save bar for a
 * preference the operator changes by clicking the control they are already
 * looking at. They never send the deprecated `scope`, so a write here never
 * bumps its revision. The cached envelope is patched first so the control
 * responds immediately; the mutation's own invalidation is the reconcile,
 * which doubles as the rollback if the daemon rejects the write.
 *
 * The response's `legacy_scope` carries an old client's breadth intent; it is
 * handed to the active-workspace store, which applies each newer explicit
 * revision once (ADR-004).
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
    groupBy: toSessionListGroupBy(sessions?.group_by),
  };
  const legacy = query.data?.legacy_scope;
  const legacyRevision = legacy?.revision;
  const legacyValue = legacy?.value;
  const legacyExplicit = legacy?.explicit;
  useEffect(() => {
    if (legacyRevision === undefined || legacyValue === undefined) return;
    observeLegacySessionsScope({
      revision: legacyRevision,
      value: legacyValue,
      explicit: legacyExplicit === true,
    });
  }, [legacyExplicit, legacyRevision, legacyValue]);

  const write = (next: SessionListPreferences) => {
    if (writePendingRef.current || mutation.isPending) return;
    writePendingRef.current = true;
    setWritePending(true);
    const sessionsConfig = { sort: next.sort, group_by: next.groupBy };
    queryClient.setQueryData<SettingsShellSection>(settingsKeys.section("shell"), cached =>
      cached === undefined
        ? cached
        : {
            ...cached,
            config: {
              ...cached.config,
              sessions: { ...cached.config.sessions, ...sessionsConfig },
            },
          }
    );
    mutation.mutate(
      { config: { sessions: sessionsConfig } },
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
    setGroupBy: groupBy => write({ ...current, groupBy }),
    loading: query.isLoading,
    saving: mutation.isPending || writePending,
  };
}

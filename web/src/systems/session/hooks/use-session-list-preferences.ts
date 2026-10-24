import { useEffect } from "react";

import { useSettingsShell } from "@/systems/settings";
import { observeLegacySessionsScope } from "@/systems/workspace";

import {
  toSessionListGroupBy,
  toSessionListSort,
  type SessionListPreferences,
} from "../lib/session-list-preferences";

export interface SessionListPreferencesModel extends SessionListPreferences {
  /** True while the daemon has not answered yet — the documented defaults render meanwhile. */
  loading: boolean;
}

/**
 * The operator's daemon-wide session order and grouping, read from the
 * `[shell.sessions]` section (`compozy config get/set shell.sessions.*`). It
 * seeds a session window that has not chosen its own order yet, and orders the
 * sessions modal; windows keep their own choice (`useSessionWindowOrder`) and
 * never write it back, so one window's choice cannot move another's.
 *
 * The response's `legacy_scope` carries an old client's breadth intent; it is
 * handed to the active-workspace store, which applies each newer explicit
 * revision once (ADR-004).
 *
 * Until the first read lands, the documented defaults render, so the list is
 * never blank waiting on a preference.
 */
export function useSessionListPreferences(): SessionListPreferencesModel {
  const query = useSettingsShell();
  const sessions = query.data?.config.sessions;
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

  return {
    sort: toSessionListSort(sessions?.sort),
    groupBy: toSessionListGroupBy(sessions?.group_by),
    loading: query.isLoading,
  };
}

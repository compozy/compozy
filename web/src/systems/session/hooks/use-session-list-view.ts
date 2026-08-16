import { useState } from "react";

import { useActiveWorkspace } from "@/systems/workspace";

import { useSessionListPreferences } from "./use-session-list-preferences";
import {
  useWorkspaceSessionGroups,
  type WorkspaceSessionGroup,
} from "./use-workspace-session-groups";
import type { SessionListScope, SessionListSort } from "../lib/session-list-preferences";

export interface SessionListViewModel {
  scope: SessionListScope;
  sort: SessionListSort;
  saving: boolean;
  setScope: (scope: SessionListScope) => void;
  setSort: (sort: SessionListSort) => void;
  /** Populated only in the all-workspaces scope. */
  workspaceGroups: WorkspaceSessionGroup[];
  collapsedWorkspaceIds: ReadonlySet<string>;
  toggleWorkspace: (workspaceId: string) => void;
}

/**
 * View model behind every session list: the operator's persisted scope and
 * order, plus the per-workspace groups the widest scope needs.
 *
 * The widened queries only run in the scope that shows them, so choosing
 * "Recent" costs nothing extra. Group collapse is deliberately transient — it
 * is a way of looking at the list right now, not a preference worth
 * round-tripping through config.
 */
export function useSessionListView(): SessionListViewModel {
  const preferences = useSessionListPreferences();
  const { workspaces } = useActiveWorkspace();
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() => new Set());
  const workspaceGroups = useWorkspaceSessionGroups({
    workspaces,
    sort: preferences.sort,
    enabled: preferences.scope === "all-workspaces",
  });

  return {
    scope: preferences.scope,
    sort: preferences.sort,
    saving: preferences.saving,
    setScope: preferences.setScope,
    setSort: preferences.setSort,
    workspaceGroups: preferences.scope === "all-workspaces" ? workspaceGroups : [],
    collapsedWorkspaceIds: collapsed,
    toggleWorkspace: workspaceId =>
      setCollapsed(current => {
        const next = new Set(current);
        if (!next.delete(workspaceId)) next.add(workspaceId);
        return next;
      }),
  };
}

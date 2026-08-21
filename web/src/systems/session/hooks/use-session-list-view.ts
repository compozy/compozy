import { useState } from "react";

import { useProfileReadScope, type ProfileOwner, type ProfileOwnerLabel } from "@/systems/profiles";
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
  /** True while the list shows the archive instead of the active catalog. */
  archived: boolean;
  saving: boolean;
  setScope: (scope: SessionListScope) => void;
  setSort: (sort: SessionListSort) => void;
  setArchived: (archived: boolean) => void;
  /** Populated only in the all-workspaces scope. */
  workspaceGroups: WorkspaceSessionGroup[];
  collapsedWorkspaceIds: ReadonlySet<string>;
  toggleWorkspace: (workspaceId: string) => void;
  /** The aggregate is on — rows name their owner (US-011.AC-1). */
  aggregate: boolean;
  /**
   * The profile the list is bounded by, or `null` under the aggregate. Named by
   * the empty state; the create target is deliberately not used there.
   */
  scopeLabel: string | null;
  /** Resolves a row's owner tag. Only called while `aggregate` is true. */
  ownerOf: (session: ProfileOwnerLabel) => ProfileOwner;
}

/**
 * View model behind every session list: the operator's persisted scope and
 * order, plus the per-workspace groups the widest scope needs.
 *
 * The widened queries only run in the scope that shows them, so staying in this
 * workspace costs nothing extra. Group collapse and the archive are deliberately
 * transient — they are ways of looking at the list right now, not preferences
 * worth round-tripping through config.
 */
export function useSessionListView(): SessionListViewModel {
  const preferences = useSessionListPreferences();
  const { workspaces } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() => new Set());
  const [archived, setArchived] = useState(false);
  const workspaceGroups = useWorkspaceSessionGroups({
    workspaces,
    sort: preferences.sort,
    archived,
    enabled: preferences.scope === "all-workspaces",
  });

  return {
    scope: preferences.scope,
    sort: preferences.sort,
    archived,
    saving: preferences.saving,
    setScope: preferences.setScope,
    setSort: preferences.setSort,
    setArchived,
    workspaceGroups: preferences.scope === "all-workspaces" ? workspaceGroups : [],
    aggregate: profile.aggregate,
    scopeLabel: profile.scopeLabel,
    ownerOf: profile.ownerOf,
    collapsedWorkspaceIds: collapsed,
    toggleWorkspace: workspaceId =>
      setCollapsed(current => {
        const next = new Set(current);
        if (!next.delete(workspaceId)) next.add(workspaceId);
        return next;
      }),
  };
}

import { useQueries } from "@tanstack/react-query";

import { useState } from "react";

import { useDebouncedInput } from "@/hooks/use-debounced-input";

import {
  useProfileReadScope,
  useProfiles,
  type ProfileOwner,
  type ProfileOwnerLabel,
} from "@/systems/profiles";
import { useActiveWorkspace, workspaceSelectorOrAll, type DataScope } from "@/systems/workspace";

import { sessionFoldersOptions } from "../lib/session-catalog-options";
import type { SessionFolderPayload } from "../types";

import { useSessionListPreferences } from "./use-session-list-preferences";
import {
  useWorkspaceSessionGroups,
  type WorkspaceSessionGroup,
} from "./use-workspace-session-groups";
import { useSessionCatalog } from "./use-session-catalog";
import { useSessionFolders, type SessionListLayout } from "./use-session-folders";
import {
  sessionListSortParam,
  sessionListSubagentsParam,
  type SessionListScope,
  type SessionListSort,
} from "../lib/session-list-preferences";

export interface SessionProfileFolderGroup {
  owner: ProfileOwner;
  workspaceId: string;
  folders?: SessionFolderPayload[];
  canCreateSession: boolean;
}

export interface SessionListViewModel {
  profileFolderGroups?: SessionProfileFolderGroup[];
  profileFoldersLoading?: boolean;
  profileFoldersError?: { retry: () => void };
  catalog?: ReturnType<typeof useSessionCatalog>;
  search?: string;
  setSearch?: (search: string) => void;
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
  /**
   * Folders and manual order of this workspace for the acting profile. Absent in
   * the widest scope; the profile aggregate carries one layout group per owner.
   */
  layout?: SessionListLayout;
  /** The worktree the catalog is narrowed to; other worktrees' sessions are hidden. */
  worktreeId?: string;
  /** The folder list failed to load; rows stay flat and unorderable until a retry loads it. */
  layoutError?: { retry: () => void };
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
export function useSessionListView(
  options: {
    /** The list's data scope; omitted follows the menubar destination. */
    scope?: DataScope | null;
    worktreeId?: string;
    enabled?: boolean;
  } = {}
): SessionListViewModel {
  const preferences = useSessionListPreferences();
  const { registeredWorkspaces: workspaces, dataScope } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(() => new Set());
  const {
    draftValue: search,
    committedValue: remoteSearch,
    setDraftValue: setSearch,
  } = useDebouncedInput({ externalValue: "", onCommit: () => undefined });
  const [archived, setArchived] = useState(false);
  const enabled = options.enabled ?? true;
  const workspaceScoped = enabled && preferences.scope === "workspace";
  const searchSettled = search === remoteSearch;
  const workspaceGroups = useWorkspaceSessionGroups({
    workspaces,
    sort: preferences.sort,
    archived,
    enabled: enabled && preferences.scope === "all-workspaces",
    search: remoteSearch,
  });

  const scope = options.scope === undefined ? dataScope : options.scope;
  // Folders and manual order belong to one project or to User; the aggregate has none.
  const workspaceId = scope === null ? null : workspaceSelectorOrAll(scope);
  const catalog = useSessionCatalog(
    scope,
    {
      include_health: true,
      limit: 100,
      sort: sessionListSortParam(preferences.sort),
      q: remoteSearch,
      search_fields: "title_agent",
      subagents: sessionListSubagentsParam(remoteSearch),
      worktree: options.worktreeId,
      ...(archived ? { archive: "only" as const } : {}),
    },
    workspaceScoped
  );

  const profileFolders = useProfileFolderGroups(
    workspaceId,
    profile.destination,
    workspaceScoped && profile.aggregate
  );

  const folders = useSessionFolders({
    workspaceId,
    profile: profile.destination,
    enabled: workspaceScoped && !profile.aggregate,
  });

  return {
    ...profileFolders,
    layout: folders.layout,
    layoutError: folders.loadError,
    worktreeId: options.worktreeId,
    catalog: {
      ...catalog,
      paging: catalog.paging || !searchSettled,
      next: searchSettled && catalog.next,
      previous: searchSettled && catalog.previous,
    },
    search,
    setSearch,
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

function useProfileFolderGroups(
  workspaceId: string | null,
  destination: string,
  enabled: boolean
): Pick<
  SessionListViewModel,
  "profileFolderGroups" | "profileFoldersLoading" | "profileFoldersError"
> {
  const active = enabled && workspaceId !== null;
  const profiles = useProfiles(active);
  const owners = enabled && workspaceId ? (profiles.data ?? []) : [];
  const folderQueries = useQueries({
    queries: owners.map(owner =>
      sessionFoldersOptions({ workspaceId: workspaceId ?? "", profile: owner.name })
    ),
  });
  return {
    profileFoldersLoading: active && profiles.isPending,
    profileFoldersError:
      active && profiles.isError
        ? {
            retry: () => {
              void profiles.refetch();
            },
          }
        : undefined,
    profileFolderGroups: owners.map((owner, index) => ({
      owner: { ...owner, archived: owner.archived_at != null },
      workspaceId: workspaceId ?? "",
      folders: folderQueries[index]?.data,
      canCreateSession: owner.name === destination,
    })),
  };
}

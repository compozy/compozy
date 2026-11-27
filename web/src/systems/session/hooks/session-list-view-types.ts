import type { ComputerId } from "@/lib/computer-scope";
import type { ProfileOwner, ProfileOwnerLabel } from "@/systems/profiles";
import type { DataScope, WorkspaceBadgeIdentity } from "@/systems/workspace";

import type { SessionListGroupSpec, SessionListPair } from "../lib/session-list-groups";
import type {
  SessionListComputerLayout,
  SessionListGroupBy,
  SessionListSort,
} from "../lib/session-list-preferences";
import type { SessionListShelf } from "../lib/session-list-shelf";
import type { SessionWindowComputerScope } from "../lib/session-sidebar-scope";
import type { ProjectGroupTree } from "../lib/workspace-folders";
import type { SessionListFilters, SessionPayload } from "../types";
import type { useSessionCatalog } from "./use-session-catalog";
import type { SessionListLayout } from "./use-session-folders";
import type { SessionListTotals } from "./use-session-list-groups";
import type { SessionListPartitions } from "./use-session-list-partitions";
import type { SessionListPinnedRead } from "./use-session-list-pinned-read";
import type { SessionFoldAll, SessionFoldSet } from "./use-session-sidebar-state";
import type { WorkspaceLayout } from "./use-workspace-layout";

export interface SessionListViewModel {
  /** Owning computer for this view (`"local"` or a remote computer id). */
  computerId?: ComputerId;
  /** True when the computer is offline or lacks `sessions:operate` (`F10`, `F11`). */
  readOnly?: boolean;
  /** Workspaces catalog for this computer (used by split subgroups and badges). */
  workspaces?: ReadonlyArray<{ id: string; name: string }>;
  /** Matched workspace ids on a remote computer when topbar focuses one project (`Q5`). */
  matchedWorkspaceIds?: readonly string[] | null;
  /** Active per-window Computers layer (`together` | `separate`). */
  computers?: SessionListComputerLayout;
  setComputers?: (computers: SessionListComputerLayout) => void;
  /** Active per-window computer scope (`all` | `local` | `<computerId>`). */
  computerScope?: SessionWindowComputerScope;
  setComputerScope?: (scope: SessionWindowComputerScope) => void;
  /** The flat catalog; absent while the list is grouped (each group owns its page). */
  catalog?: ReturnType<typeof useSessionCatalog>;
  partitions?: SessionListPartitions;
  folderOf?: ReadonlyMap<string, string | null>;
  search?: string;
  setSearch?: (search: string) => void;
  /** The stored sort, kept even where `manual` cannot apply. */
  sort: SessionListSort;
  /** The stored grouping; `groupBy` below is what renders. */
  storedGroupBy: SessionListGroupBy;
  /** The grouping that renders: an inapplicable stored value renders as `none`. */
  groupBy: SessionListGroupBy;
  /** Group-by options available in the current breadth and profile lens. */
  groupByOptions: readonly SessionListGroupBy[];
  /** Muted toolbar note while `manual` falls back to `last_activity`. */
  sortNote: string | null;
  /**
   * Why the list's New folder is unavailable: a flat aggregate spans pairs, and
   * a grouped list adds folders from each group's own `+`. `null` where it applies:
   * one pair's session folders, or the project folders of All projects by project.
   */
  newFolderUnavailable: string | null;
  /** What the list shows: the active catalog, the archive, or the sessions in Later. */
  shelf: SessionListShelf;
  /** Sessions parked in Later across the list's scope; `undefined` until counted. */
  laterCount?: number;
  /** Archived sessions across the list's scope, counted on the archive shelf only. */
  archivedCount?: number;
  setSort: (sort: SessionListSort) => void;
  setGroupBy: (groupBy: SessionListGroupBy) => void;
  /** Project grouping also lists projects with no sessions in view (#1085); per window. */
  showEmptyProjects: boolean;
  setShowEmptyProjects: (show: boolean) => void;
  /** "Show automation runs" (#1355): automation job and trigger sessions join the list. */
  showAutomation: boolean;
  setShowAutomation: (show: boolean) => void;
  setShelf: (shelf: SessionListShelf) => void;
  /** Group-by groups from server facets; empty while flat. */
  groups: SessionListGroupSpec[];
  groupsLoading: boolean;
  groupsError?: { retry: () => void };
  collapsedGroupKeys: SessionFoldSet;
  toggleGroup: (key: string) => void;
  /** The last Collapse all or Expand all; shared with this window's Kanban. */
  foldAll: SessionFoldAll;
  setFoldAll: (foldAll: Exclude<SessionFoldAll, null>) => void;
  /** Threads opened one by one, which stay open under Collapse all. */
  openThreadIds: readonly string[];
  /** Filters every group query shares; each group adds its scope, lens, and order. */
  groupFilters: SessionListFilters;
  /** The aggregate is on — rows name their owner (US-011.AC-1). */
  aggregate: boolean;
  /**
   * The profile the list is bounded by, or `null` under the aggregate. Named by
   * the empty state; the create target is deliberately not used there.
   */
  scopeLabel: string | null;
  /** The list's one concrete scope: a workspace id or `@user`; `null` under All projects. */
  focusedWorkspaceId: string | null;
  /** The list spans every project (and User scope). */
  allProjects?: boolean;
  focusedScopeName: string;
  /** Resolves a row's owner tag. Only called while `aggregate` is true. */
  ownerOf: (session: ProfileOwnerLabel) => ProfileOwner;
  /** Resolves a row's project badge; present only under All projects (BR-10). */
  badgeOf?: (session: SessionPayload) => WorkspaceBadgeIdentity;
  /** Folders and manual order of the one (computer, workspace, profile) pair the flat list reads. */
  layout?: SessionListLayout;
  /** The one (computer, workspace, profile) pair this flat view owns, if any. */
  pair?: SessionListPair | null;
  /**
   * Project folders of the one profile an All projects list grouped by project
   * reads (issue #1044): the layout verbs and the groups arranged into folders.
   */
  projectFolders?: { layout: WorkspaceLayout; tree: ProjectGroupTree };
  /** The worktree the catalog is narrowed to; other worktrees' sessions are hidden. */
  worktreeId?: string;
  /** The folder list failed to load; rows stay flat and unorderable until a retry loads it. */
  layoutError?: { retry: () => void };
  /** Identity of the list's bulk selection, shared by every group of a grouped list. */
  selectionKey: string;
  /** Exact live-state totals of the list's whole scope, from facets; absent in the archive. */
  totals?: SessionListTotals;
  /** The Pinned section's read, above every group and folder; absent off the active shelf. */
  pinned?: SessionListPinnedRead;
}

export interface UseSessionListViewOptions {
  /** The list's data scope; omitted follows the menubar destination. */
  scope?: DataScope | null;
  worktreeId?: string;
  enabled?: boolean;
  /**
   * The list is open but still waiting on what it reads under (the destination or its
   * worktree filter): it reads as loading, not as an empty list, until they settle.
   */
  pending?: boolean;
  /** Target computer for this view (`F5`); omitted reads the local daemon. */
  computerId?: ComputerId;
  /** Workspaces catalog for a remote computer (`P1`). */
  workspaces?: ReadonlyArray<{ id: string; name: string }>;
  /** Matched workspace ids on a remote computer when topbar focuses one project (`Q5`). */
  matchedWorkspaceIds?: readonly string[] | null;
  /** True when the computer is offline or lacks `sessions:operate`. */
  readOnly?: boolean;
  /** Controlled search string from a parent multi-computer container (`F7`, `F12`). */
  search?: string;
  /** Whether the window currently shows multiple computers (`O3`, `Q-G`). */
  multiComputer?: boolean;
  /** Skip the Pinned read when a parent container renders one combined Pinned section (`F12`). */
  includePinned?: boolean;
}

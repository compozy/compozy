import type { ProfileOwner, ProfileScopeParams } from "@/systems/profiles";
import {
  ownerDataScope,
  workspaceBadgeIdentity,
  workspaceSelectorForScope,
  workspaceSelectorOrAll,
  type DataScope,
  type FocusedDataScope,
  type WorkspaceBadgeIdentity,
} from "@/systems/workspace";

import type { fetchSessionFacets } from "../adapters/session-catalog-api";
import type { SessionListFilters } from "../types";
import { sessionListSortParam, type SessionListSort } from "./session-list-preferences";
import { sessionListShelfNoun, type SessionListShelf } from "./session-list-shelf";

type SessionCatalogFacets = Awaited<ReturnType<typeof fetchSessionFacets>>;

/** Facet counts of one group, as the daemon served them (never page-derived). */
interface GroupFacets {
  all: number;
  working: number;
  needs_you: number;
}

/** One (workspace, profile) pair: the only place folders and manual order live (BR-11). */
export interface SessionListPair {
  workspaceId: string;
  profile: string;
}

/**
 * One Group-by group: its identity, its server counts, and its own query, which
 * pages independently of every other group.
 */
export interface SessionListGroupSpec {
  key: string;
  kind: "project" | "profile";
  /** Project groups: the project badge (User for sessions without one). */
  badge?: WorkspaceBadgeIdentity;
  /** Profile groups: the profile owner. */
  owner?: ProfileOwner;
  total: number;
  working: number;
  needsYou: number;
  /** The group's own data scope. */
  scope: DataScope;
  /** The group's own profile lens. */
  lens: ProfileScopeParams;
  /** Set only when the group's own query is exactly one (workspace, profile) pair. */
  pair: SessionListPair | null;
}

function counts(facets: GroupFacets) {
  return { total: facets.all, working: facets.working, needsYou: facets.needs_you };
}

/** The pair a bounded scope and a single profile resolve to; `null` for any aggregate. */
export function sessionListPair(
  scope: DataScope | null,
  lens: ProfileScopeParams
): SessionListPair | null {
  if (scope === null || scope.kind === "all" || !("profile" in lens) || !lens.profile) {
    return null;
  }
  return { workspaceId: workspaceSelectorForScope(scope), profile: lens.profile };
}

/**
 * Project groups from `by_workspace`: registered projects in switcher order,
 * then unregistered ids, then one User group for sessions without a project.
 * Only groups with members exist; each reads its own scope under the list's lens.
 */
export function projectSessionGroups(
  facets: Pick<SessionCatalogFacets, "by_workspace"> | undefined,
  input: {
    workspaces: ReadonlyArray<{ id: string; name: string }>;
    lens: ProfileScopeParams;
  }
): SessionListGroupSpec[] {
  if (!facets) return [];
  const order = new Map(input.workspaces.map((workspace, index) => [workspace.id, index]));
  const rows = facets.by_workspace.flatMap(row =>
    row.facets.all > 0 ? [{ row, scope: ownerDataScope(row.workspace_id) }] : []
  );
  const rank = (scope: FocusedDataScope) =>
    scope.kind === "user" ? Number.MAX_SAFE_INTEGER : (order.get(scope.workspaceId) ?? order.size);
  rows.sort((left, right) => rank(left.scope) - rank(right.scope));
  return rows.map(({ row }) => projectSessionGroup(row.workspace_id, row.facets, input));
}

/** One project group: a workspace id (`""` for User) with its served counts. */
export function projectSessionGroup(
  workspaceId: string,
  facets: GroupFacets,
  input: {
    workspaces: ReadonlyArray<{ id: string; name: string }>;
    lens: ProfileScopeParams;
  }
): SessionListGroupSpec {
  const names = new Map(input.workspaces.map(workspace => [workspace.id, workspace.name]));
  const scope = ownerDataScope(workspaceId);
  return {
    key: scope.kind === "user" ? "user" : `project:${scope.workspaceId}`,
    kind: "project",
    badge: workspaceBadgeIdentity(workspaceId, names),
    ...counts(facets),
    scope,
    lens: input.lens,
    pair: sessionListPair(scope, input.lens),
  };
}

/**
 * Profile groups from `by_profile`, in profile-list order; each reads the
 * list's scope through that one profile.
 */
export function profileSessionGroups(
  facets: Pick<SessionCatalogFacets, "by_profile"> | undefined,
  input: { profiles: readonly ProfileOwner[]; scope: DataScope }
): SessionListGroupSpec[] {
  if (!facets) return [];
  const byId = new Map(facets.by_profile.map(row => [row.profile_id, row.facets]));
  return input.profiles.flatMap(owner => {
    const row = byId.get(owner.id);
    if (!row || row.all === 0) return [];
    const lens: ProfileScopeParams = { profile: owner.name };
    return [
      {
        key: `profile:${owner.id}`,
        kind: "profile" as const,
        owner,
        ...counts(row),
        scope: input.scope,
        lens,
        pair: sessionListPair(input.scope, lens),
      },
    ];
  });
}

/** A group's own page query: the list's shared filters in the order its pair allows. */
export function sessionListGroupFilters(
  view: { groupFilters: SessionListFilters; sort: SessionListSort },
  group: Pick<SessionListGroupSpec, "pair">
): SessionListFilters {
  return { ...view.groupFilters, sort: sessionListSortParam(view.sort, group.pair !== null) };
}

/**
 * The project (a workspace id or `@user`) and profile a group settles; an axis
 * the group spans — All projects or All profiles — stays `null`.
 */
export function sessionListGroupTarget(
  group: Pick<SessionListGroupSpec, "pair" | "scope" | "lens">
) {
  return {
    workspaceId: group.pair?.workspaceId ?? workspaceSelectorOrAll(group.scope),
    profile: group.pair?.profile ?? ("profile" in group.lens ? group.lens.profile : null) ?? null,
  };
}

/** What a group's header names: its project (User included) or its profile. */
export function sessionListGroupName(group: Pick<SessionListGroupSpec, "badge" | "owner">): string {
  return group.badge?.name ?? group.owner?.name ?? "group";
}

/**
 * An empty group names itself, never the list's profile: inside the project
 * group "compozy" the empty state is "No sessions in compozy yet." (#1085).
 */
export function sessionListGroupEmptyMessage(
  group: Pick<SessionListGroupSpec, "badge" | "owner">,
  shelf: SessionListShelf
): string {
  return `No ${sessionListShelfNoun(shelf)} in ${sessionListGroupName(group)} yet.`;
}

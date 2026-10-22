/**
 * Where the operator is working. A project, or **User**: the profile's work
 * that belongs to no project. Focus survives All projects, so turning the
 * globe off returns to it.
 */
export type Focus = { kind: "project"; workspaceId: string } | { kind: "user" };

/** The globe: every project plus User (`all`), or only the focus (`focused`). */
export type BreadthMode = "all" | "focused";

/**
 * The only consumer-facing data-scope type. `null` and `""` never mean User:
 * a reader that needs User must receive `{ kind: "user" }`.
 */
export type DataScope =
  | { kind: "all" }
  | { kind: "project"; workspaceId: string }
  | { kind: "user" };

/** A bounded scope: the one a workspace selector can name. */
export type FocusedDataScope = Exclude<DataScope, { kind: "all" }>;

export const USER_FOCUS: Focus = { kind: "user" };
export const ALL_DATA_SCOPE: DataScope = { kind: "all" };

/** Public workspace reference for the User scope (route path, `workspace`/`workspace_id`). */
export const USER_SCOPE_SELECTOR = "@user";

/** Reserved daemon partition for the All projects desktop of a profile. */
export const GLOBAL_DESKTOP_WORKSPACE_ID = "global";
/** Reserved daemon partition for the User desktop of a profile. */
export const USER_DESKTOP_WORKSPACE_ID = "user";

/** Reserved partitions have no project: no workspace config overlay, no workspace data. */
export function isReservedDesktopPartition(workspaceId: string | null | undefined): boolean {
  return workspaceId === GLOBAL_DESKTOP_WORKSPACE_ID || workspaceId === USER_DESKTOP_WORKSPACE_ID;
}

/**
 * The desktop partition a destination mounts: the focused project's, `user`
 * for User, and the reserved `global` desktop under All projects, so opening
 * an item there never lands in a project's layout.
 */
export function desktopPartitionFor(breadthMode: BreadthMode, focus: Focus): string {
  if (breadthMode === "all") return GLOBAL_DESKTOP_WORKSPACE_ID;
  return focus.kind === "project" ? focus.workspaceId : USER_DESKTOP_WORKSPACE_ID;
}

export function dataScopeForFocus(focus: Focus): FocusedDataScope {
  return focus.kind === "user" ? { kind: "user" } : focus;
}

/**
 * The scope an item lives in, from its own `workspace_id`. Items report User
 * work with the field absent or empty, so this — not a resolver default — is
 * the one place where empty maps to User.
 */
export function ownerDataScope(workspaceId: string | null | undefined): FocusedDataScope {
  const id = workspaceId?.trim() ?? "";
  return id === "" ? { kind: "user" } : { kind: "project", workspaceId: id };
}

/** Workspace selector a bounded scope sends: the project id, or `@user`. */
export function workspaceSelectorForScope(scope: FocusedDataScope): string {
  return scope.kind === "user" ? USER_SCOPE_SELECTOR : scope.workspaceId;
}

/** Inverse of `workspaceSelectorForScope`; `null` for an empty selector. */
export function dataScopeFromSelector(selector: string): FocusedDataScope | null {
  const value = selector.trim();
  if (value === "") return null;
  return value === USER_SCOPE_SELECTOR ? { kind: "user" } : { kind: "project", workspaceId: value };
}

/** Selector for a scope that may be the aggregate; `null` under All projects. */
export function workspaceSelectorOrAll(scope: DataScope): string | null {
  return scope.kind === "all" ? null : workspaceSelectorForScope(scope);
}

/**
 * Selector a focused-scope surface reads with: the project id, `@user`, or
 * `""` when nothing focused is known (All projects, unsettled) — the
 * surface's existing empty-workspace gate keeps it idle.
 */
export function focusedWorkspaceSelector(scope: DataScope | null | undefined): string {
  return scope ? (workspaceSelectorOrAll(scope) ?? "") : "";
}

export function sameDataScope(left: DataScope, right: DataScope): boolean {
  if (left.kind !== right.kind) return false;
  return left.kind !== "project" || left.workspaceId === (right as typeof left).workspaceId;
}

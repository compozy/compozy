import {
  dataScopeFromSelector,
  GLOBAL_DESKTOP_WORKSPACE_ID,
  USER_DESKTOP_WORKSPACE_ID,
  workspaceSelectorForScope,
  type DataScope,
  type FocusedDataScope,
} from "./destination";

/** Route search key that binds a window to the data it shows (`"<id>"` or `"@user"`). */
export const WINDOW_BINDING_SEARCH_KEY = "workspace";

/** The serialized binding of an item's scope: its project id, or `@user`. */
export function windowBindingValue(scope: FocusedDataScope): string {
  return workspaceSelectorForScope(scope);
}

/**
 * A window's data scope (`_spec.md` window binding state table). A serialized
 * binding wins in every partition; an unbound window reads its partition —
 * `global` the aggregate, `user` User, a project that project. `null` while
 * the partition is unknown.
 */
export function windowDataScope(
  partition: string | null | undefined,
  binding: unknown
): DataScope | null {
  const bound = typeof binding === "string" ? dataScopeFromSelector(binding) : null;
  if (bound) return bound;
  const id = partition?.trim() ?? "";
  if (id === "") return null;
  if (id === GLOBAL_DESKTOP_WORKSPACE_ID) return { kind: "all" };
  if (id === USER_DESKTOP_WORKSPACE_ID) return { kind: "user" };
  return { kind: "project", workspaceId: id };
}

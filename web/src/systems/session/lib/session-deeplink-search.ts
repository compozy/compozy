import type { SearchSchemaInput } from "@tanstack/react-router";

export const SESSION_WORKSPACE_SWITCH_STATES = ["confirm", "declined"] as const;
export type SessionWorkspaceSwitchState = (typeof SESSION_WORKSPACE_SWITCH_STATES)[number];

/**
 * Routed state for the cross-workspace deep-link confirmation: the URL owns whether the
 * confirmation is open (`confirm`) or was declined (`declined`), and nothing else. The owning
 * workspace id and name are never read from the URL — they come from the owner projection the
 * loader resolved — so a hand-written search value cannot manufacture a confirmation payload.
 */
export interface SessionDeepLinkSearch {
  workspaceSwitch?: SessionWorkspaceSwitchState;
  /**
   * The window's data binding (`"<id>"` or `"@user"`), set by open-in-place under
   * All projects. It only says which scope the window reads; ownership is still
   * proven by the loader, never taken from the URL.
   */
  workspace?: string;
}

export function validateSessionDeepLinkSearch(
  search: Record<string, unknown>
): SessionDeepLinkSearch;
export function validateSessionDeepLinkSearch(
  search: SessionDeepLinkSearch & SearchSchemaInput
): SessionDeepLinkSearch;
export function validateSessionDeepLinkSearch(
  search: Record<string, unknown> | SessionDeepLinkSearch
): SessionDeepLinkSearch {
  const workspaceSwitch = search.workspaceSwitch;
  const workspace = typeof search.workspace === "string" ? search.workspace.trim() : "";
  return {
    ...(typeof workspaceSwitch === "string" &&
    (SESSION_WORKSPACE_SWITCH_STATES as readonly string[]).includes(workspaceSwitch)
      ? { workspaceSwitch: workspaceSwitch as SessionWorkspaceSwitchState }
      : {}),
    ...(workspace !== "" ? { workspace } : {}),
  };
}

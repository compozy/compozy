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
  return typeof workspaceSwitch === "string" &&
    (SESSION_WORKSPACE_SWITCH_STATES as readonly string[]).includes(workspaceSwitch)
    ? { workspaceSwitch: workspaceSwitch as SessionWorkspaceSwitchState }
    : {};
}

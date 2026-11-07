import { FILES_APP_PATH, filesAppRoute, matchFilesInstance } from "./app-catalog";
import type { OsOpenTarget, OsWindow, OsWindowRoute } from "./os-types";

/** A project id a window route names: a Files instance or an All projects binding. */
function routeProject(win: Pick<OsWindow, "app" | "instanceKey" | "route">): string | null {
  if (win.app === "files" && win.instanceKey) return win.instanceKey;
  const binding = win.route.search.workspace;
  if (typeof binding !== "string") return null;
  const trimmed = binding.trim();
  // `@user` (and the legacy `user`) name the User scope, which has no project files.
  return trimmed === "" || trimmed.startsWith("@") || trimmed === "user" ? null : trimmed;
}

/**
 * The project the Files dock icon opens (US-002.AC-2): the focused window's project, else the
 * most recently used one (`fallback`, the remembered project focus).
 */
export function filesProjectForOpen(
  state: { windows: Readonly<Record<string, OsWindow>>; focusedId: string | null },
  fallback: string | null
): string | null {
  const focused = state.focusedId === null ? undefined : state.windows[state.focusedId];
  return (focused ? routeProject(focused) : null) ?? fallback;
}

/**
 * The open-or-focus target for a project's Files window. The instance key is the project, so a
 * second open focuses the same window (US-002.AC-5); with no project the bare route opens the
 * window's "No projects yet" state.
 */
export function filesOpenTarget(workspaceId: string | null): OsOpenTarget {
  if (workspaceId === null)
    return { app: "files", route: { pathname: FILES_APP_PATH, search: {} } };
  return { app: "files", instanceKey: workspaceId, route: filesAppRoute(workspaceId) };
}

/** The project a palette or agent route names (`/files/<id>`), if any. */
export function filesRouteProject(route: OsWindowRoute | null): string | null {
  return route === null ? null : matchFilesInstance(route.pathname);
}

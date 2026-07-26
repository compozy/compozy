import type { AghApiJsonResponseFor, AghApiOkJsonResponseFor } from "@/storybook/openapi-msw";
import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";

export const windowManagerStoryDesktopId = "desktop-launch";
export const windowManagerStoryWindowId = "app:settings";

export const windowManagerSnapshotFixture: AghApiOkJsonResponseFor<
  "get",
  "/api/workspaces/{workspace_id}/window-manager"
> = {
  // SnapshotVersion (internal/windowmanager/types.go:9).
  version: 2,
  workspace_id: storyDefaultWorkspaceId,
  revision: 12,
  desktops: [
    {
      id: windowManagerStoryDesktopId,
      name: "Launch",
      order: 0,
      purpose: "standard",
      groups: [
        {
          id: "group-settings",
          frame: { x: 0, y: 0, width: 1, height: 1 },
          root: {
            id: "leaf-settings",
            kind: "leaf",
            window_id: windowManagerStoryWindowId,
          },
        },
      ],
      floating: [],
    },
  ],
  windows: {
    [windowManagerStoryWindowId]: {
      id: windowManagerStoryWindowId,
      app: "settings",
      route: { pathname: "/settings/layouts", search: {} },
      placement: "tiled",
      desktop_id: windowManagerStoryDesktopId,
      floating_rect: { x: 0.08, y: 0.08, width: 0.84, height: 0.84 },
      minimized: false,
    },
  },
  history: { undo: [], redo: [] },
  overrides: {},
  updated_at: "2026-07-23T01:00:00Z",
};

export function windowManagerClientFixture(
  clientId: string,
  workspaceId: string = storyDefaultWorkspaceId
): AghApiJsonResponseFor<"post", "/api/workspaces/{workspace_id}/window-manager/clients", 201> {
  return {
    workspace_id: workspaceId,
    client_id: clientId,
    presentation_revision: 1,
    active_desktop_id: windowManagerStoryDesktopId,
    focused_window_id: windowManagerStoryWindowId,
    focus_order: [windowManagerStoryWindowId],
    connected_at: "2026-07-23T01:00:00Z",
  };
}

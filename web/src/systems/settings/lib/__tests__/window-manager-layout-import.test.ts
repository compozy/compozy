// Suite: layout file import
// Invariant: a layout exported before the Automations merge (version 4, with
// Jobs and Triggers windows) still imports; its retired windows become
// Automations windows on the rewritten routes and no window is dropped.
// Owning layer: Settings layout import parsing. Boundary OUT: the daemon's
// stored-snapshot migration (internal/windowmanager), which owns the same rule.
import { describe, expect, it } from "vitest";

import { parseImportedWindowManagerLayoutDocument } from "../window-manager-layout-import";

function wireWindow(id: string, app: string, pathname: string, search: Record<string, unknown>) {
  return {
    id,
    app,
    route: { pathname, search },
    nav_stack: [{ pathname, search }],
    pinned: false,
    placement: "tiled",
    desktop_id: "desktop-one",
    floating_rect: { x: 0.2, y: 0.2, width: 0.4, height: 0.4 },
    minimized: false,
    zoomed: false,
  };
}

function wireDocument(version: number) {
  return {
    version,
    workspace_id: "workspace-a",
    desktops: [
      {
        id: "desktop-one",
        name: "Main",
        order: 0,
        floating: [],
        floating_stacks: [],
        groups: [
          {
            id: "group-main",
            frame: { x: 0, y: 0, width: 1, height: 1 },
            root: {
              id: "split",
              kind: "split",
              axis: "horizontal",
              weights: [1, 1, 1],
              children: [
                { id: "leaf-jobs", kind: "leaf", window_id: "w-jobs" },
                { id: "leaf-triggers", kind: "leaf", window_id: "w-triggers" },
                { id: "leaf-tasks", kind: "leaf", window_id: "w-tasks" },
              ],
            },
          },
        ],
      },
    ],
    windows: {
      "w-jobs": wireWindow("w-jobs", "jobs", "/jobs/morning-digest", {
        q: "digest",
        source: "dynamic",
      }),
      "w-triggers": wireWindow("w-triggers", "triggers", "/triggers", {
        event: "session.stopped",
      }),
      "w-tasks": wireWindow("w-tasks", "tasks", "/tasks", {}),
    },
    overrides: {},
  };
}

describe("parseImportedWindowManagerLayoutDocument", () => {
  it("Should upgrade a version 4 export and move its Jobs and Triggers windows to Automations", () => {
    const document = parseImportedWindowManagerLayoutDocument(wireDocument(4));

    expect(document.version).toBe(5);
    expect(Object.keys(document.windows)).toEqual(["w-jobs", "w-triggers", "w-tasks"]);
    expect(document.windows["w-jobs"]).toMatchObject({
      app: "automations",
      route: {
        pathname: "/automations/jobs/morning-digest",
        search: { q: "digest", source: "dynamic" },
      },
      navStack: [
        {
          pathname: "/automations/jobs/morning-digest",
          search: { q: "digest", source: "dynamic" },
        },
      ],
    });
    expect(document.windows["w-triggers"]).toMatchObject({
      app: "automations",
      route: { pathname: "/automations", search: { start: "event", q: "session.stopped" } },
    });
    expect(document.windows["w-tasks"]).toMatchObject({
      app: "tasks",
      route: { pathname: "/tasks", search: {} },
    });
  });

  it("Should import a version 5 document unchanged and reject other versions", () => {
    expect(parseImportedWindowManagerLayoutDocument(wireDocument(5)).windows["w-jobs"]?.app).toBe(
      "jobs"
    );
    expect(() => parseImportedWindowManagerLayoutDocument(wireDocument(3))).toThrow();
  });
});

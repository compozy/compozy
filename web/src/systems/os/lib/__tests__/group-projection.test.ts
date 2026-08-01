// Suite: group/frame projection
// Invariant: every window renders through exactly one frame per desktop — tiled panes, floating
// windows, and floating tab stacks — with display-active resolved per ADR-009
// (client stackActive ?? durable activeId) in a single pass over the snapshot.
// Boundary OUT: React rendering and pixel work-area measurement.
import { describe, expect, it } from "vitest";

import { buildDesktopFrames, frameForWindow } from "../group-projection";
import { projectLayout } from "../layout-projection";
import type {
  LayoutDesktop,
  LayoutProjection,
  WindowManagerClientView,
  WindowManagerSnapshot,
  WindowManagerWindow,
} from "../window-manager-types";

const WORK_AREA = { x: 0, y: 0, w: 1440, h: 820 };

function windowFixture(
  id: string,
  desktopId: string,
  overrides: Partial<WindowManagerWindow> = {}
): WindowManagerWindow {
  return {
    id,
    app: "tasks",
    instanceKey: null,
    route: { pathname: "/tasks", search: {} },
    navStack: [],
    pinned: false,
    placement: "floating",
    desktopId,
    floatingRect: { x: 0.1, y: 0.1, w: 0.4, h: 0.4 },
    minimized: false,
    returnAnchor: null,
    ...overrides,
  };
}

function clientFixture(overrides: Partial<WindowManagerClientView> = {}): WindowManagerClientView {
  return {
    workspaceId: "workspace:test",
    clientId: "client:web",
    activeDesktopId: "desktop:a",
    focusedWindowId: null,
    focusOrder: [],
    stackActive: {},
    connectedAt: "2026-07-30T00:00:00Z",
    presentationRevision: 1,
    ...overrides,
  };
}

function snapshotFixture(
  desktops: LayoutDesktop[],
  windows: WindowManagerWindow[]
): WindowManagerSnapshot {
  return {
    version: 3,
    workspaceId: "workspace:test",
    revision: 4,
    desktops,
    windows: Object.fromEntries(windows.map(window => [window.id, window])),
    closedEntryCount: 0,
    overrides: {},
    updatedAt: "2026-07-30T00:00:00Z",
  };
}

function projections(
  snapshot: WindowManagerSnapshot,
  client: WindowManagerClientView | null
): Readonly<Record<string, LayoutProjection>> {
  const result: Record<string, LayoutProjection> = {};
  for (const desktop of snapshot.desktops) {
    result[desktop.id] = projectLayout({
      revision: snapshot.revision,
      desktop,
      workArea: WORK_AREA,
      gaps: { inner: 8, top: 0, right: 0, bottom: 0, left: 0 },
      stackActive: client?.stackActive ?? {},
    });
  }
  return result;
}

describe("buildDesktopFrames", () => {
  it("Should project floating stacks as one frame with client active over durable active", () => {
    const desktop: LayoutDesktop = {
      id: "desktop:a",
      name: "A",
      order: 0,
      purpose: "standard",
      focusOwner: null,
      groups: [],
      floating: ["w-solo"],
      floatingStacks: [
        {
          id: "fs-1",
          windowIds: ["w-1", "w-2", "w-3"],
          activeId: "w-2",
          rect: { x: 0.2, y: 0.2, w: 0.5, h: 0.5 },
          minimized: false,
        },
      ],
    };
    const snapshot = snapshotFixture(
      [desktop],
      [
        windowFixture("w-solo", "desktop:a"),
        windowFixture("w-1", "desktop:a", { placement: "stacked" }),
        windowFixture("w-2", "desktop:a", { placement: "stacked" }),
        windowFixture("w-3", "desktop:a", { placement: "stacked" }),
      ]
    );
    const client = clientFixture({ stackActive: { "fs-1": "w-3" } });

    const frames = buildDesktopFrames({
      snapshot,
      client,
      projections: projections(snapshot, client),
      workArea: WORK_AREA,
      raiseOnFocus: true,
    });

    const desktopFrames = frames["desktop:a"] ?? [];
    expect(desktopFrames).toHaveLength(2);
    const stackFrame = desktopFrames.find(frame => frame.id === "fs-1");
    expect(stackFrame).toMatchObject({
      kind: "floating",
      members: ["w-1", "w-2", "w-3"],
      activeWindowId: "w-3",
      stackId: "fs-1",
      minimized: false,
    });

    const durable = buildDesktopFrames({
      snapshot,
      client: clientFixture(),
      projections: projections(snapshot, clientFixture()),
      workArea: WORK_AREA,
      raiseOnFocus: true,
    });
    expect(durable["desktop:a"]?.find(frame => frame.id === "fs-1")?.activeWindowId).toBe("w-2");
  });

  it("Should carry the floating stack's minimized flag on its frame", () => {
    const desktop: LayoutDesktop = {
      id: "desktop:a",
      name: "A",
      order: 0,
      purpose: "standard",
      focusOwner: null,
      groups: [],
      floating: [],
      floatingStacks: [
        {
          id: "fs-1",
          windowIds: ["w-1", "w-2"],
          activeId: "w-1",
          rect: { x: 0.1, y: 0.1, w: 0.4, h: 0.4 },
          minimized: true,
        },
      ],
    };
    const snapshot = snapshotFixture(
      [desktop],
      [
        windowFixture("w-1", "desktop:a", { placement: "stacked" }),
        windowFixture("w-2", "desktop:a", { placement: "stacked" }),
      ]
    );

    const frames = buildDesktopFrames({
      snapshot,
      client: null,
      projections: projections(snapshot, null),
      workArea: WORK_AREA,
      raiseOnFocus: false,
    });

    expect(frames["desktop:a"]?.[0]?.minimized).toBe(true);
  });

  it("Should rank a floating frame by its most recently focused member", () => {
    const desktop: LayoutDesktop = {
      id: "desktop:a",
      name: "A",
      order: 0,
      purpose: "standard",
      focusOwner: null,
      groups: [],
      floating: ["w-solo"],
      floatingStacks: [
        {
          id: "fs-1",
          windowIds: ["w-1", "w-2"],
          activeId: "w-1",
          rect: { x: 0.3, y: 0.3, w: 0.4, h: 0.4 },
          minimized: false,
        },
      ],
    };
    const snapshot = snapshotFixture(
      [desktop],
      [
        windowFixture("w-solo", "desktop:a"),
        windowFixture("w-1", "desktop:a", { placement: "stacked" }),
        windowFixture("w-2", "desktop:a", { placement: "stacked" }),
      ]
    );
    const client = clientFixture({ focusOrder: ["w-2", "w-solo"] });

    const frames = buildDesktopFrames({
      snapshot,
      client,
      projections: projections(snapshot, client),
      workArea: WORK_AREA,
      raiseOnFocus: true,
    });

    const byId = new Map((frames["desktop:a"] ?? []).map(frame => [frame.id, frame]));
    expect(byId.get("fs-1")!.layer).toBeGreaterThan(byId.get("w-solo")!.layer);
  });

  it("Should project 200 windows across 12 groups in one pass with correct frames [UT-178]", () => {
    const desktops: LayoutDesktop[] = [];
    const windows: WindowManagerWindow[] = [];
    const stackActive: Record<string, string> = {};
    for (let desktopIndex = 0; desktopIndex < 4; desktopIndex += 1) {
      const desktopId = `desktop:${desktopIndex}`;
      const floatingStacks = [];
      for (let stackIndex = 0; stackIndex < 3; stackIndex += 1) {
        const stackId = `fs-${desktopIndex}-${stackIndex}`;
        const members: string[] = [];
        // 200 windows total: 4 desktops × 3 groups × 16 members = 192 + 8 solos.
        for (let member = 0; member < 16; member += 1) {
          const windowId = `w-${desktopIndex}-${stackIndex}-${member}`;
          members.push(windowId);
          windows.push(windowFixture(windowId, desktopId, { placement: "stacked" }));
        }
        floatingStacks.push({
          id: stackId,
          windowIds: members,
          activeId: members[3] ?? null,
          rect: { x: 0.05 * stackIndex, y: 0.05, w: 0.3, h: 0.3 },
          minimized: false,
        });
        stackActive[stackId] = members[7] ?? "";
      }
      const soloIds = [`w-solo-${desktopIndex}-0`, `w-solo-${desktopIndex}-1`];
      for (const soloId of soloIds) windows.push(windowFixture(soloId, desktopId));
      desktops.push({
        id: desktopId,
        name: `D${desktopIndex}`,
        order: desktopIndex,
        purpose: "standard",
        focusOwner: null,
        groups: [],
        floating: soloIds,
        floatingStacks,
      });
    }
    const snapshot = snapshotFixture(desktops, windows);
    const client = clientFixture({ stackActive });
    expect(Object.keys(snapshot.windows)).toHaveLength(200);

    let indexCalls = 0;
    const countingFocusOrder = new Proxy([] as string[], {
      get(target, property, receiver) {
        if (property === "indexOf") indexCalls += 1;
        return Reflect.get(target, property, receiver);
      },
    });

    const frames = buildDesktopFrames({
      snapshot,
      client: { ...client, focusOrder: countingFocusOrder },
      projections: {},
      workArea: WORK_AREA,
      raiseOnFocus: true,
    });

    const allFrames = Object.values(frames).flat();
    expect(allFrames).toHaveLength(12 + 8);
    for (const desktop of desktops) {
      for (const stack of desktop.floatingStacks) {
        const frame = allFrames.find(candidate => candidate.id === stack.id);
        expect(frame?.members).toEqual(stack.windowIds);
        expect(frame?.activeWindowId).toBe(stackActive[stack.id]);
      }
    }
    // One focus-rank scan per member, never per member × frame (quadratic).
    expect(indexCalls).toBeLessThanOrEqual(200);
  });
});

describe("frameForWindow", () => {
  it("Should find the frame owning a member and null for unknown windows", () => {
    const desktop: LayoutDesktop = {
      id: "desktop:a",
      name: "A",
      order: 0,
      purpose: "standard",
      focusOwner: null,
      groups: [],
      floating: [],
      floatingStacks: [
        {
          id: "fs-1",
          windowIds: ["w-1", "w-2"],
          activeId: "w-1",
          rect: { x: 0.1, y: 0.1, w: 0.4, h: 0.4 },
          minimized: false,
        },
      ],
    };
    const snapshot = snapshotFixture(
      [desktop],
      [
        windowFixture("w-1", "desktop:a", { placement: "stacked" }),
        windowFixture("w-2", "desktop:a", { placement: "stacked" }),
      ]
    );
    const frames = buildDesktopFrames({
      snapshot,
      client: null,
      projections: {},
      workArea: WORK_AREA,
      raiseOnFocus: false,
    });

    expect(frameForWindow(frames, "w-2")?.id).toBe("fs-1");
    expect(frameForWindow(frames, "w-missing")).toBeNull();
  });
});

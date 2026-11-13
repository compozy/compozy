// Suite: layout document edits
// Invariant: every edit the Settings layout editor makes — tab add/remove/reorder/pin,
// split, unsplit, axis and weight changes, window assignment — returns a version 6
// document the daemon's topology validation accepts, or returns the document itself
// when the edit would break a rule (a desktop without tabs, a window shown twice).
// Owning layer: Settings layout lib (pure document functions).
// Boundary IN: one v6 document + one edit. Boundary OUT: React state, transport.
import { describe, expect, it } from "vitest";

import { layoutDocumentProblems } from "../window-manager-layout-problems";
import {
  closeLayoutTab,
  closeLayoutWindow,
  createLayoutIdFactory,
  locateLayoutWindow,
  moveLayoutTab,
  moveLayoutWindowBeside,
  moveLayoutWindowToNewTab,
  normalizeLayoutDocument,
  replaceLayoutTabRoot,
  setLayoutDefaultTab,
  setLayoutTabPinned,
  swapLayoutWindowPlaces,
} from "../window-manager-layout-mutations";
import {
  moveLayoutSeam,
  setLayoutSplitAxis,
  setLayoutSplitWeights,
} from "../window-manager-layout-tree";
import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
  WindowManagerLayoutWindow,
} from "../window-manager-layout-types";

function windowOf(id: string, tabId: string): WindowManagerLayoutWindow {
  return {
    id,
    app: id,
    instanceKey: null,
    route: { pathname: `/${id}`, search: {} },
    navStack: [],
    desktopId: "desktop-main",
    tabId,
  };
}

/** The contract example: a pinned session tab and a two-way split of terminal and tasks. */
function seedDocument(): WindowManagerLayoutDocument {
  return {
    version: 6,
    workspaceId: "workspace-a",
    desktops: [
      {
        id: "desktop-main",
        name: "Main",
        order: 0,
        activeTabId: "tab-work",
        tabs: [
          {
            id: "tab-session",
            pinned: true,
            root: { id: "leaf-session", kind: "leaf", windowId: "session" },
          },
          {
            id: "tab-work",
            pinned: false,
            root: {
              id: "split-work",
              kind: "split",
              axis: "horizontal",
              weights: [0.6, 0.4],
              children: [
                { id: "leaf-terminal", kind: "leaf", windowId: "terminal" },
                { id: "leaf-tasks", kind: "leaf", windowId: "tasks" },
              ],
            },
          },
          {
            id: "tab-files",
            pinned: false,
            root: { id: "leaf-files", kind: "leaf", windowId: "files" },
          },
        ],
      },
    ],
    windows: {
      session: windowOf("session", "tab-session"),
      terminal: windowOf("terminal", "tab-work"),
      tasks: windowOf("tasks", "tab-work"),
      files: windowOf("files", "tab-files"),
    },
    overrides: {},
  };
}

function desktop(document: WindowManagerLayoutDocument) {
  const first = document.desktops[0];
  if (!first) throw new Error("seed has a desktop");
  return first;
}

function tabIds(document: WindowManagerLayoutDocument): string[] {
  return desktop(document).tabs.map(tab => tab.id);
}

function rootOf(document: WindowManagerLayoutDocument, tabId: string): WindowManagerLayoutNode {
  const tab = desktop(document).tabs.find(candidate => candidate.id === tabId);
  if (!tab) throw new Error(`no tab ${tabId}`);
  return tab.root;
}

function expectValid(document: WindowManagerLayoutDocument) {
  expect(layoutDocumentProblems(document)).toEqual([]);
}

describe("layoutDocumentProblems", () => {
  it("Should accept the contract example and report each rule the daemon enforces", () => {
    expectValid(seedDocument());

    const broken: WindowManagerLayoutDocument = {
      ...seedDocument(),
      desktops: [
        {
          ...desktop(seedDocument()),
          order: 3,
          activeTabId: "tab-missing",
          tabs: [
            {
              id: "tab-files",
              pinned: false,
              root: { id: "leaf-files", kind: "leaf", windowId: "files" },
            },
            {
              id: "tab-session",
              pinned: true,
              root: {
                id: "split-one",
                kind: "split",
                axis: "horizontal",
                weights: [1],
                children: [{ id: "leaf-session", kind: "leaf", windowId: "session" }],
              },
            },
          ],
        },
      ],
    };
    const problems = layoutDocumentProblems(broken);

    expect(problems).toEqual(
      expect.arrayContaining([
        "Desktop desktop-main is out of order.",
        "Desktop desktop-main opens on a tab it does not have.",
        "Pinned tabs on desktop desktop-main are not all at the start.",
        "Split split-one has fewer than two members.",
        "Window terminal is not shown in any tab.",
      ])
    );
  });

  it("Should refuse weights that do not sum to 1 and nested splits on the same axis", () => {
    const document = replaceLayoutTabRoot(seedDocument(), "desktop-main", "tab-work", {
      id: "split-work",
      kind: "split",
      axis: "horizontal",
      weights: [0.6, 0.6],
      children: [
        { id: "leaf-terminal", kind: "leaf", windowId: "terminal" },
        { id: "leaf-tasks", kind: "leaf", windowId: "tasks" },
      ],
    });

    // Edits normalize, so weights handed in unbalanced are stored summing to 1.
    expectValid(document);

    const raw = seedDocument();
    const rawDesktop = desktop(raw);
    const nested: WindowManagerLayoutDocument = {
      ...raw,
      desktops: [
        {
          ...rawDesktop,
          tabs: rawDesktop.tabs.map(tab =>
            tab.id !== "tab-work"
              ? tab
              : {
                  ...tab,
                  root: {
                    id: "split-work",
                    kind: "split",
                    axis: "horizontal",
                    weights: [0.5, 0.6],
                    children: [
                      { id: "leaf-terminal", kind: "leaf", windowId: "terminal" },
                      {
                        id: "split-inner",
                        kind: "split",
                        axis: "horizontal",
                        weights: [0.5, 0.5],
                        children: [
                          { id: "leaf-tasks", kind: "leaf", windowId: "tasks" },
                          { id: "leaf-x", kind: "leaf", windowId: "files" },
                        ],
                      },
                    ],
                  },
                }
          ),
        },
      ],
    };

    expect(layoutDocumentProblems(nested)).toEqual(
      expect.arrayContaining([
        "Split split-work has weights that are not positive or do not add up to 1.",
        "Split split-inner repeats its parent's direction.",
        "Window files is shown twice.",
      ])
    );
  });
});

describe("tab edits", () => {
  it("Should keep pinned tabs a contiguous prefix when pinning and moving", () => {
    const pinned = setLayoutTabPinned(seedDocument(), "desktop-main", "tab-files", true);
    expect(tabIds(pinned)).toEqual(["tab-session", "tab-files", "tab-work"]);
    expectValid(pinned);

    const moved = moveLayoutTab(seedDocument(), "desktop-main", "tab-files", -1);
    expect(tabIds(moved)).toEqual(["tab-session", "tab-files", "tab-work"]);
    expectValid(moved);
  });

  it("Should refuse to move a regular tab before a pinned one", () => {
    const document = seedDocument();
    expect(moveLayoutTab(document, "desktop-main", "tab-work", -1)).toBe(document);
  });

  it("Should close a tab with its windows but keep a desktop's last tab", () => {
    const closed = closeLayoutTab(seedDocument(), "desktop-main", "tab-work");
    expect(tabIds(closed)).toEqual(["tab-session", "tab-files"]);
    expect(Object.keys(closed.windows).sort()).toEqual(["files", "session"]);
    // The desktop opened on the closed tab, so it falls back to the first one.
    expect(desktop(closed).activeTabId).toBe("tab-session");
    expectValid(closed);

    const single = closeLayoutTab(
      closeLayoutTab(closed, "desktop-main", "tab-files"),
      "desktop-main",
      "tab-session"
    );
    expect(tabIds(single)).toEqual(["tab-session"]);
  });

  it("Should only open a desktop on a tab it has", () => {
    const document = seedDocument();
    expect(setLayoutDefaultTab(document, "desktop-main", "tab-missing")).toBe(document);
    expect(desktop(setLayoutDefaultTab(document, "desktop-main", "tab-files")).activeTabId).toBe(
      "tab-files"
    );
  });
});

describe("split edits", () => {
  it("Should split beside a window and close the tab the moved window left", () => {
    const document = seedDocument();
    const next = moveLayoutWindowBeside(
      document,
      "files",
      "terminal",
      "right",
      createLayoutIdFactory(document)
    );

    expect(tabIds(next)).toEqual(["tab-session", "tab-work"]);
    const root = rootOf(next, "tab-work");
    // Same axis as the parent: flattened into one three-member split, terminal's share halved.
    expect(root).toMatchObject({ kind: "split", axis: "horizontal" });
    if (root.kind !== "split") throw new Error("split");
    expect(root.children.map(child => (child.kind === "leaf" ? child.windowId : child.id))).toEqual(
      ["terminal", "files", "tasks"]
    );
    expect(root.weights[0]).toBeCloseTo(0.3);
    expect(root.weights[1]).toBeCloseTo(0.3);
    expect(next.windows.files?.tabId).toBe("tab-work");
    expectValid(next);
  });

  it("Should nest a split on the other axis when splitting above or below", () => {
    const document = seedDocument();
    const next = moveLayoutWindowBeside(
      document,
      "files",
      "tasks",
      "bottom",
      createLayoutIdFactory(document)
    );
    const root = rootOf(next, "tab-work");
    if (root.kind !== "split") throw new Error("split");
    expect(root.children[1]).toMatchObject({ kind: "split", axis: "vertical" });
    expectValid(next);
  });

  it("Should refuse to move a window beside itself", () => {
    const document = seedDocument();
    expect(
      moveLayoutWindowBeside(document, "tasks", "tasks", "left", createLayoutIdFactory(document))
    ).toBe(document);
  });

  it("Should unsplit a member into its own tab right after the one it left", () => {
    const document = seedDocument();
    const next = moveLayoutWindowToNewTab(document, "tasks", createLayoutIdFactory(document));

    expect(tabIds(next)).toEqual(["tab-session", "tab-work", "tab:settings-1", "tab-files"]);
    // The split collapsed to the remaining window.
    expect(rootOf(next, "tab-work")).toEqual({
      id: "leaf-terminal",
      kind: "leaf",
      windowId: "terminal",
    });
    expect(locateLayoutWindow(next, "tasks")?.tab.id).toBe("tab:settings-1");
    expectValid(next);

    // A window that already has a tab of its own stays put.
    expect(moveLayoutWindowToNewTab(next, "files", createLayoutIdFactory(next))).toBe(next);
  });

  it("Should change axis and weights without breaking the weight sum", () => {
    const document = seedDocument();
    const root = rootOf(document, "tab-work");
    const rows = setLayoutSplitAxis(root, "split-work", "vertical");
    expect(rows).toMatchObject({ axis: "vertical" });

    const weighted = setLayoutSplitWeights(rows, "split-work", [3, 1]);
    if (weighted.kind !== "split") throw new Error("split");
    expect(weighted.weights[0]).toBeCloseTo(0.75);

    const dragged = moveLayoutSeam(weighted, "split-work", 0, 0.999);
    if (dragged.kind !== "split") throw new Error("split");
    // Each member keeps at least the minimum share.
    expect(dragged.weights[1]).toBeGreaterThanOrEqual(0.05 - 1e-9);

    const next = replaceLayoutTabRoot(document, "desktop-main", "tab-work", dragged);
    expectValid(next);
  });

  it("Should swap two windows' places and keep each window shown once", () => {
    const next = swapLayoutWindowPlaces(seedDocument(), "session", "tasks");

    expect(locateLayoutWindow(next, "tasks")?.tab.id).toBe("tab-session");
    expect(next.windows.session?.tabId).toBe("tab-work");
    expectValid(next);
  });

  it("Should remove a window and close its tab when it was the only member", () => {
    const next = closeLayoutWindow(seedDocument(), "files");
    expect(tabIds(next)).toEqual(["tab-session", "tab-work"]);
    expect(next.windows.files).toBeUndefined();
    expectValid(next);
  });
});

describe("normalizeLayoutDocument", () => {
  it("Should derive window placement and desktop order from the topology", () => {
    const seed = seedDocument();
    const drifted: WindowManagerLayoutDocument = {
      ...seed,
      desktops: [{ ...desktop(seed), order: 4 }],
      windows: { ...seed.windows, tasks: { ...windowOf("tasks", "tab-files"), desktopId: "x" } },
    };

    const next = normalizeLayoutDocument(drifted);
    expect(desktop(next).order).toBe(0);
    expect(next.windows.tasks).toMatchObject({ desktopId: "desktop-main", tabId: "tab-work" });
    expectValid(next);
  });
});

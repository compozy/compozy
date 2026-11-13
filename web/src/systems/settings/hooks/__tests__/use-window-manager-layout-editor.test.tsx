// Suite: layout draft reconciliation
// Invariant: the workspace layout is live daemon state shared with the OS shell,
// so it can move while someone is editing. A clean editor follows it; a dirty one
// keeps the edits and loses its review, because Apply carries `expected_revision`
// and must not ride a revision the daemon never checked the draft against.
// Imported files and layouts saved before tabs and splits reach the draft only
// as the version 6 document the daemon converted them to.
// Boundary IN: successive layout states from the query; the convert adapter.
// Boundary OUT: canvas rendering, transport, the profile editor.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const apiMocks = vi.hoisted(() => ({ convert: vi.fn() }));

vi.mock("../../adapters/window-manager-layouts-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/window-manager-layouts-api")>()),
  convertWindowManagerLayoutDocument: apiMocks.convert,
}));

import { renameLayoutDesktop } from "../../lib/window-manager-layout-mutations";

import {
  useWindowManagerLayoutEditor,
  windowManagerLayoutEditorLogic,
} from "../use-window-manager-layout-editor";
import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutState,
} from "../../lib/window-manager-layout-types";

function documentWith(desktopName: string): WindowManagerLayoutDocument {
  return {
    version: 6,
    workspaceId: "workspace-a",
    desktops: [
      {
        id: "desktop-one",
        name: desktopName,
        order: 0,
        activeTabId: "tab-main",
        tabs: [
          {
            id: "tab-main",
            pinned: false,
            root: { id: "leaf-one", kind: "leaf", windowId: "app:tasks" },
          },
        ],
      },
    ],
    windows: {
      "app:tasks": {
        id: "app:tasks",
        app: "tasks",
        instanceKey: null,
        route: { pathname: "/tasks", search: {} },
        navStack: [],
        desktopId: "desktop-one",
        tabId: "tab-main",
      },
    },
    overrides: {},
  } as unknown as WindowManagerLayoutDocument;
}

function stateAt(revision: number, desktopName: string): WindowManagerLayoutState {
  return { revision, document: documentWith(desktopName) };
}

function renderEditor(initial: WindowManagerLayoutState, profile = "marketing") {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return renderHook(
    ({ state, profile: currentProfile }) =>
      useWindowManagerLayoutEditor("workspace-a", currentProfile, state),
    {
      initialProps: { state: initial, profile },
      wrapper,
    }
  );
}

function renameDesktop(document: WindowManagerLayoutDocument, name: string) {
  return renameLayoutDesktop(document, "desktop-one", name);
}

const EARLIER_WIRE = {
  version: 5,
  workspace_id: "workspace-b",
  desktops: [{ id: "desktop-one", name: "Classic", order: 0, groups: [], floating: [] }],
  windows: {},
  overrides: {},
};

describe("useWindowManagerLayoutEditor", () => {
  beforeEach(() => {
    apiMocks.convert.mockReset();
  });

  it("Should fence a stale daemon baseline during render-time reconciliation", () => {
    const { result, rerender } = renderEditor(stateAt(42, "Ship"));
    rerender({ state: stateAt(41, "Build"), profile: "marketing" });

    expect(result.current.revision).toBe(42);
    expect(result.current.draft.desktops[0]?.name).toBe("Ship");
  });

  it("Should ignore a late Apply result after the draft changes", () => {
    const store = windowManagerLayoutEditorLogic.createStore({ initial: stateAt(42, "Ship") });
    let snapshot = store.getInitialSnapshot();
    const fingerprint = snapshot.context.baselineFingerprint;
    [snapshot] = store.transition(snapshot, {
      type: "applyRequested",
      execute: async () => undefined,
    });
    [snapshot] = store.transition(snapshot, {
      type: "draftChanged",
      draft: renameDesktop(snapshot.context.draft, "Mine"),
    });
    [snapshot] = store.transition(snapshot, {
      type: "applySucceeded",
      fingerprint,
      operation: 1,
      revision: 42,
    });

    expect(snapshot.context.draft.desktops[0]?.name).toBe("Mine");
    expect(snapshot.context.phase).toBe("dirty");
  });

  it("Should follow the daemon while the draft is untouched", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Build"));
    expect(result.current.dirty).toBe(false);

    rerender({ state: stateAt(42, "Ship"), profile: "marketing" });

    expect(result.current.revision).toBe(42);
    expect(result.current.draft.desktops[0]?.name).toBe("Ship");
    expect(result.current.dirty).toBe(false);
  });

  it("Should keep unapplied edits when the workspace moves underneath them", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Build"));
    act(() => {
      result.current.updateDraft(renameDesktop(result.current.draft, "Mine"));
    });
    expect(result.current.dirty).toBe(true);

    rerender({ state: stateAt(42, "Theirs"), profile: "marketing" });

    expect(result.current.draft.desktops[0]?.name).toBe("Mine");
    expect(result.current.dirty).toBe(true);
    expect(result.current.revision).toBe(42);
    expect(result.current.reviewCurrent).toBe(false);
    expect(result.current.reviewed).toBe(null);
  });

  it("Should reset to what the daemon holds now, not to a stale snapshot", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Build"));
    act(() => {
      result.current.updateDraft(renameDesktop(result.current.draft, "Mine"));
    });
    rerender({ state: stateAt(42, "Theirs"), profile: "marketing" });

    act(() => {
      result.current.reset();
    });

    expect(result.current.draft.desktops[0]?.name).toBe("Theirs");
    expect(result.current.dirty).toBe(false);
  });

  it("Should isolate drafts when only the profile changes in one workspace", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Marketing"));
    act(() => result.current.updateDraft(renameDesktop(result.current.draft, "Mine")));

    rerender({ state: stateAt(9, "Sales"), profile: "sales" });

    expect(result.current.draft.desktops[0]?.name).toBe("Sales");
    expect(result.current.revision).toBe(9);
    expect(result.current.dirty).toBe(false);
  });

  it("Should import a file of any accepted version as the document the daemon converted it to", async () => {
    apiMocks.convert.mockResolvedValue({
      ...documentWith("Converted"),
      workspaceId: "workspace-b",
    });
    const { result } = renderEditor(stateAt(41, "Build"));
    const file = new File([JSON.stringify(EARLIER_WIRE)], "layout.json", {
      type: "application/json",
    });

    await act(async () => {
      await result.current.importDocument(file);
    });

    expect(apiMocks.convert).toHaveBeenCalledTimes(1);
    const [workspace, profile, revision, wire] = apiMocks.convert.mock.calls[0] ?? [];
    expect(workspace).toBe("workspace-a");
    expect(profile).toBe("marketing");
    expect(revision).toBe(41);
    expect(wire).toEqual(EARLIER_WIRE);
    expect(result.current.draft.version).toBe(6);
    expect(result.current.draft.desktops[0]?.name).toBe("Converted");
    // The draft always belongs to the workspace being edited.
    expect(result.current.draft.workspaceId).toBe("workspace-a");
    expect(result.current.dirty).toBe(true);
  });

  it("Should keep the draft and report the daemon's refusal when a file cannot be converted", async () => {
    apiMocks.convert.mockRejectedValue(new Error("layout version 3 is not supported"));
    const { result } = renderEditor(stateAt(41, "Build"));
    const file = new File([JSON.stringify({ ...EARLIER_WIRE, version: 3 })], "layout.json");

    await act(async () => {
      await result.current.importDocument(file);
    });

    expect(result.current.importError).toBe("layout version 3 is not supported");
    expect(result.current.draft.desktops[0]?.name).toBe("Build");
  });

  it("Should refuse a file that is not a JSON object without asking the daemon", async () => {
    const { result } = renderEditor(stateAt(41, "Build"));

    await act(async () => {
      await result.current.importDocument(new File(["[1, 2]"], "layout.json"));
    });

    expect(apiMocks.convert).not.toHaveBeenCalled();
    expect(result.current.importError).toBe("The selected file is not a layout document.");
  });

  it("Should open a profile saved before tabs and splits through the daemon's conversion", async () => {
    apiMocks.convert.mockResolvedValue(documentWith("Classic"));
    const { result } = renderEditor(stateAt(41, "Build"));

    await act(async () => {
      await result.current.loadSource({ kind: "earlier", version: 5, wire: EARLIER_WIRE });
    });

    expect(apiMocks.convert.mock.calls[0]?.[3]).toEqual(EARLIER_WIRE);
    expect(result.current.draft.version).toBe(6);
    expect(result.current.draft.desktops[0]?.name).toBe("Classic");
  });

  it("Should open a current profile directly, without a daemon round trip", async () => {
    const { result } = renderEditor(stateAt(41, "Build"));

    await act(async () => {
      await result.current.loadSource({ kind: "current", document: documentWith("Saved") });
    });

    expect(apiMocks.convert).not.toHaveBeenCalled();
    expect(result.current.draft.desktops[0]?.name).toBe("Saved");
  });
});

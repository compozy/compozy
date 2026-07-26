// Suite: layout draft reconciliation
// Invariant: the workspace layout is live daemon state shared with the OS shell,
// so it can move while someone is editing. A clean editor follows it; a dirty one
// keeps the edits and loses its review, because Apply carries `expected_revision`
// and must not ride a revision the daemon never checked the draft against.
// Boundary IN: successive layout states from the query.
// Boundary OUT: canvas rendering, transport, the profile editor.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { describe, expect, it } from "vitest";

import { useWindowManagerLayoutEditor } from "../use-window-manager-layout-editor";
import type {
  WindowManagerLayoutDocument,
  WindowManagerLayoutState,
} from "../../lib/window-manager-layout-types";

function documentWith(desktopName: string): WindowManagerLayoutDocument {
  return {
    version: 2,
    workspaceId: "workspace-a",
    desktops: [
      {
        id: "desktop-one",
        name: desktopName,
        order: 0,
        purpose: "standard",
        focusOwner: null,
        floating: [],
        groups: [
          {
            id: "group-main",
            frame: { x: 0, y: 0, w: 1, h: 1 },
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
        placement: "tiled",
        desktopId: "desktop-one",
        floatingRect: { x: 0.2, y: 0.2, w: 0.4, h: 0.4 },
        minimized: false,
        returnAnchor: null,
      },
    },
    overrides: {},
  };
}

function stateAt(revision: number, desktopName: string): WindowManagerLayoutState {
  return { revision, document: documentWith(desktopName) };
}

function renderEditor(initial: WindowManagerLayoutState) {
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  const wrapper = ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children);
  return renderHook(({ state }) => useWindowManagerLayoutEditor("workspace-a", state), {
    initialProps: { state: initial },
    wrapper,
  });
}

function renameDesktop(document: WindowManagerLayoutDocument, name: string) {
  const next = structuredClone(document);
  const desktop = next.desktops[0];
  if (desktop) desktop.name = name;
  return next;
}

describe("useWindowManagerLayoutEditor", () => {
  it("Should follow the daemon while the draft is untouched", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Build"));
    expect(result.current.dirty).toBe(false);

    rerender({ state: stateAt(42, "Ship") });

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

    rerender({ state: stateAt(42, "Theirs") });

    expect(result.current.draft.desktops[0]?.name).toBe("Mine");
    expect(result.current.dirty).toBe(true);
    expect(result.current.revision).toBe(42);
  });

  it("Should disarm Apply until the draft is reviewed against the new revision", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Build"));
    act(() => {
      result.current.updateDraft(renameDesktop(result.current.draft, "Mine"));
    });

    rerender({ state: stateAt(42, "Theirs") });

    expect(result.current.reviewCurrent).toBe(false);
    expect(result.current.reviewed).toBe(null);
  });

  it("Should reset to what the daemon holds now, not to a stale snapshot", () => {
    const { result, rerender } = renderEditor(stateAt(41, "Build"));
    act(() => {
      result.current.updateDraft(renameDesktop(result.current.draft, "Mine"));
    });
    rerender({ state: stateAt(42, "Theirs") });

    act(() => {
      result.current.reset();
    });

    expect(result.current.draft.desktops[0]?.name).toBe("Theirs");
    expect(result.current.dirty).toBe(false);
  });
});

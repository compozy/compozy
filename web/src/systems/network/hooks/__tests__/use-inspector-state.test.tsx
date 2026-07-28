// Suite: Network inspector preference
// Invariant: inspector preferences stay isolated by workspace and channel while persisting local UI state.
// Boundary IN: useInspectorState and networkInspectorLogic.
// Boundary OUT: Network member queries, owned by use-network-inspector-view.test.tsx.
// @vitest-environment jsdom

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  NETWORK_INSPECTOR_STORAGE_KEY,
  networkInspectorLogic,
  useInspectorState,
} from "../use-inspector-state";

describe("useInspectorState", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  afterEach(() => {
    window.localStorage.clear();
  });

  it("keeps matching channel preferences isolated across workspace transitions", () => {
    const store = networkInspectorLogic.createStore();
    const [alpha] = store.transition(store.getInitialSnapshot(), {
      type: "inspectorOpened",
      workspaceId: "ws_alpha",
      channel: "operations",
      tab: "work",
    });
    const [beta] = store.transition(alpha, {
      type: "inspectorOpened",
      workspaceId: "ws_beta",
      channel: "operations",
      tab: "members",
    });

    expect(beta.context.byWorkspace).toEqual({
      ws_alpha: { operations: { open: true, tab: "work" } },
      ws_beta: { operations: { open: true, tab: "members" } },
    });
  });

  it("persists a channel tab without opening the same channel in another workspace", () => {
    const { result, rerender } = renderHook(
      ({ workspaceId }) => useInspectorState({ channel: "operations", workspaceId }),
      { initialProps: { workspaceId: "ws_preference_alpha" } }
    );

    act(() => {
      result.current.openWith("work");
    });
    expect(result.current).toMatchObject({ open: true, tab: "work" });

    rerender({ workspaceId: "ws_preference_beta" });
    expect(result.current).toMatchObject({ open: false, tab: "members" });

    const persisted = JSON.parse(
      window.localStorage.getItem(NETWORK_INSPECTOR_STORAGE_KEY) ?? "{}"
    ) as { context?: { byWorkspace?: Record<string, Record<string, unknown>> } };
    expect(persisted.context?.byWorkspace).toMatchObject({
      ws_preference_alpha: {
        operations: { open: true, tab: "work" },
      },
    });
  });
});

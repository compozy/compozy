// Suite: session inspector open/closed preference
// Invariant: inspector defaults closed; transitions persist per sessionId without leaking between sessions.
// Boundary IN: useSessionInspectorState + window.localStorage.
// Boundary OUT: SessionInspector mount, topbar chrome, DetailInspector breakpoints.

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  requestSessionInspectorTab,
  SESSION_INSPECTOR_STORAGE_KEY,
  sessionInspectorLogic,
  sessionInspectorStore,
  useSessionInspectorState,
} from "../use-session-inspector-state";

beforeEach(() => {
  window.localStorage.clear();
  act(() => {
    sessionInspectorStore.trigger.reset();
  });
});

afterEach(() => {
  window.localStorage.clear();
  act(() => {
    sessionInspectorStore.trigger.reset();
  });
});

describe("useSessionInspectorState", () => {
  it("Should default closed when no preference is stored", () => {
    const { result } = renderHook(() => useSessionInspectorState("sess_default"));
    expect(result.current.open).toBe(false);
  });

  it("Should toggle open and persist per sessionId", () => {
    const { result } = renderHook(() => useSessionInspectorState("sess_toggle"));

    act(() => {
      result.current.toggle();
    });
    expect(result.current.open).toBe(true);
    expect(
      JSON.parse(window.localStorage.getItem(SESSION_INSPECTOR_STORAGE_KEY) ?? "{}")
    ).toMatchObject({
      context: { bySession: { sess_toggle: true } },
    });

    act(() => {
      result.current.toggle();
    });
    expect(result.current.open).toBe(false);
    expect(
      JSON.parse(window.localStorage.getItem(SESSION_INSPECTOR_STORAGE_KEY) ?? "{}")
    ).toMatchObject({
      context: { bySession: { sess_toggle: false } },
    });
  });

  it("Should isolate open state across session ids", () => {
    const { result: a } = renderHook(() => useSessionInspectorState("sess_isolated_a"));
    const { result: b } = renderHook(() => useSessionInspectorState("sess_isolated_b"));

    act(() => {
      a.current.setOpen(true);
    });
    expect(a.current.open).toBe(true);
    expect(b.current.open).toBe(false);

    act(() => {
      b.current.setOpen(true);
    });
    expect(b.current.open).toBe(true);
    expect(a.current.open).toBe(true);
    expect(
      JSON.parse(window.localStorage.getItem(SESSION_INSPECTOR_STORAGE_KEY) ?? "{}")
    ).toMatchObject({
      context: {
        bySession: {
          sess_isolated_a: true,
          sess_isolated_b: true,
        },
      },
    });
  });

  it("Should keep an unnamed session closed without persisting a preference", () => {
    const { result } = renderHook(() => useSessionInspectorState(undefined));
    expect(result.current.open).toBe(false);

    act(() => {
      result.current.toggle();
    });
    expect(result.current.open).toBe(false);
    expect(window.localStorage.getItem(SESSION_INSPECTOR_STORAGE_KEY)).toBeNull();

    act(() => {
      result.current.close();
    });
    expect(result.current.open).toBe(false);
  });

  it("Should keep session values isolated in pure transitions", () => {
    const store = sessionInspectorLogic.createStore();
    const [opened] = store.transition(store.getInitialSnapshot(), {
      type: "inspectorVisibilityChanged",
      sessionId: "sess_transition_a",
      open: true,
    });
    const [closed] = store.transition(opened, {
      type: "inspectorClosed",
      sessionId: "sess_transition_b",
    });

    expect(closed.context.bySession).toEqual({
      sess_transition_a: true,
      sess_transition_b: false,
    });
    expect(closed.context.tabBySession).toEqual({});
  });

  it("Should open a requested tab atomically and isolate it by session", () => {
    const { result: a } = renderHook(() => useSessionInspectorState("sess_tab_a"));
    const { result: b } = renderHook(() => useSessionInspectorState("sess_tab_b"));

    act(() => {
      a.current.openTab("calls");
    });
    expect(a.current.open).toBe(true);
    expect(a.current.tab).toBe("calls");
    expect(b.current.open).toBe(false);
    expect(b.current.tab).toBe("usage");

    act(() => {
      requestSessionInspectorTab("sess_tab_b", "memory");
    });
    expect(b.current.open).toBe(true);
    expect(b.current.tab).toBe("memory");
    expect(a.current.tab).toBe("calls");
  });

  it("Should forget the requested tab when the inspector closes", () => {
    const { result } = renderHook(() => useSessionInspectorState("sess_tab_close"));

    act(() => {
      result.current.openTab("calls");
    });
    expect(result.current.tab).toBe("calls");

    act(() => {
      result.current.close();
    });
    expect(result.current.open).toBe(false);
    expect(result.current.tab).toBe("usage");
  });

  it("Should keep a requested tab out of persisted preferences", () => {
    const { result } = renderHook(() => useSessionInspectorState("sess_tab_persist"));

    act(() => {
      result.current.openTab("calls");
    });
    expect(
      JSON.parse(window.localStorage.getItem(SESSION_INSPECTOR_STORAGE_KEY) ?? "{}")
    ).toMatchObject({
      context: { bySession: { sess_tab_persist: true } },
    });
    expect(
      JSON.parse(window.localStorage.getItem(SESSION_INSPECTOR_STORAGE_KEY) ?? "{}").context
        .tabBySession
    ).toBeUndefined();
  });
});

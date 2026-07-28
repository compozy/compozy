// Suite: session inspector open/closed preference
// Invariant: inspector defaults closed; toggle/setOpen persist per sessionId in localStorage.
// Boundary IN: useSessionInspectorState + window.localStorage.
// Boundary OUT: SessionInspector mount, topbar chrome, DetailInspector breakpoints.

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { useSessionInspectorState } from "../use-session-inspector-state";

const STORAGE_KEY = "session:inspector-open";

beforeEach(() => {
  window.localStorage.clear();
});

afterEach(() => {
  window.localStorage.clear();
});

describe("useSessionInspectorState", () => {
  it("Should default closed when no preference is stored", () => {
    const { result } = renderHook(() => useSessionInspectorState("sess_a"));
    expect(result.current.open).toBe(false);
  });

  it("Should toggle open and persist per sessionId", () => {
    const { result } = renderHook(() => useSessionInspectorState("sess_a"));

    act(() => {
      result.current.toggle();
    });
    expect(result.current.open).toBe(true);
    expect(JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "{}")).toEqual({
      sess_a: true,
    });

    act(() => {
      result.current.toggle();
    });
    expect(result.current.open).toBe(false);
    expect(JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "{}")).toEqual({
      sess_a: false,
    });
  });

  it("Should isolate open state across session ids", () => {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify({ sess_a: true, sess_b: false }));

    const { result: a } = renderHook(() => useSessionInspectorState("sess_a"));
    const { result: b } = renderHook(() => useSessionInspectorState("sess_b"));

    expect(a.current.open).toBe(true);
    expect(b.current.open).toBe(false);

    act(() => {
      b.current.setOpen(true);
    });
    expect(b.current.open).toBe(true);
    expect(a.current.open).toBe(true);
    expect(JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "{}")).toEqual({
      sess_a: true,
      sess_b: true,
    });
  });

  it("Should close without affecting an empty session key", () => {
    const { result } = renderHook(() => useSessionInspectorState(undefined));
    expect(result.current.open).toBe(false);

    act(() => {
      result.current.toggle();
    });
    expect(result.current.open).toBe(true);
    expect(window.localStorage.getItem(STORAGE_KEY)).toBeNull();

    act(() => {
      result.current.close();
    });
    expect(result.current.open).toBe(false);
  });
});

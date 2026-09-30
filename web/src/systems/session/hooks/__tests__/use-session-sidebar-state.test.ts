// Suite: session-window sessions rail open/closed preference
// Invariant: the rail opens by default in every session window; a persisted
// operator toggle (localStorage) wins over the default and survives a reload;
// a pre-default blob's incidental `open: false` never masks the default.
// Boundary IN: useSessionSidebarState + its persisted store + window.localStorage.
// Boundary OUT: SessionSidebar layout, topbar toggle chrome, the session list query.

import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const STORAGE_KEY = "compozy:session:sidebar:v1";

async function loadFreshSidebarState() {
  vi.resetModules();
  return import("../use-session-sidebar-state");
}

beforeEach(() => {
  window.localStorage.clear();
});

afterEach(() => {
  window.localStorage.clear();
});

describe("useSessionSidebarState", () => {
  it("Should open the rail when no preference is stored", async () => {
    const { useSessionSidebarState } = await loadFreshSidebarState();
    const { result } = renderHook(() => useSessionSidebarState());
    expect(result.current.open).toBe(true);
  });

  it("Should keep a closed rail closed across a reload once the operator closed it", async () => {
    const first = await loadFreshSidebarState();
    const { result } = renderHook(() => first.useSessionSidebarState());
    act(() => {
      result.current.toggle();
    });
    expect(result.current.open).toBe(false);
    expect(JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "{}")).toMatchObject({
      context: { open: false },
    });

    const reloaded = await loadFreshSidebarState();
    const { result: afterReload } = renderHook(() => reloaded.useSessionSidebarState());
    expect(afterReload.current.open).toBe(false);
  });

  it("Should open the rail and keep collapsed threads when upgrading a pre-default blob", async () => {
    // Written before the rail opened by default: collapsing a thread persisted
    // the whole context, including the old closed default.
    window.localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({
        context: { open: false, collapsedThreads: { sess_root: true } },
        version: 0,
      })
    );

    const upgraded = await loadFreshSidebarState();
    const { result } = renderHook(() => upgraded.useSessionSidebarState());
    expect(result.current.open).toBe(true);
    expect(result.current.collapsedThreadIds).toEqual(["sess_root"]);

    act(() => {
      result.current.toggle();
    });
    const reloaded = await loadFreshSidebarState();
    const { result: afterReload } = renderHook(() => reloaded.useSessionSidebarState());
    expect(afterReload.current.open).toBe(false);
    expect(afterReload.current.collapsedThreadIds).toEqual(["sess_root"]);
  });
});

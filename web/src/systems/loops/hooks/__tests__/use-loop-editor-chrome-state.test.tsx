import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import {
  LOOP_EDITOR_CHROME_STORAGE_KEY,
  loopEditorChromeLogic,
  loopEditorChromeStore,
  useLoopEditorChromeState,
} from "../use-loop-editor-chrome-state";

function resetChrome() {
  loopEditorChromeStore.trigger.chromeVisibilityChanged({
    palette: false,
    inspector: false,
    dockCollapsed: true,
  });
}

function persistedChrome(): unknown {
  return JSON.parse(window.localStorage.getItem(LOOP_EDITOR_CHROME_STORAGE_KEY) ?? "{}");
}

async function writeFromAnotherTab(value: string) {
  window.localStorage.setItem(LOOP_EDITOR_CHROME_STORAGE_KEY, value);
  await act(async () => {
    window.dispatchEvent(new StorageEvent("storage", { key: LOOP_EDITOR_CHROME_STORAGE_KEY }));
    await Promise.resolve();
  });
}

beforeEach(() => {
  resetChrome();
  window.localStorage.clear();
});

afterEach(() => {
  resetChrome();
  window.localStorage.clear();
});

describe("useLoopEditorChromeState", () => {
  it("Should open with both rails collapsed and the dock folded", () => {
    expect(loopEditorChromeLogic.createStore().getInitialSnapshot().context).toEqual({
      paletteOpen: false,
      inspectorOpen: false,
      dockCollapsed: true,
    });

    const { result } = renderHook(() => useLoopEditorChromeState());
    expect(result.current.paletteOpen).toBe(false);
    expect(result.current.inspectorOpen).toBe(false);
    expect(result.current.dockCollapsed).toBe(true);
  });

  it("Should toggle each surface and persist the preference under one flat key", () => {
    const { result } = renderHook(() => useLoopEditorChromeState());

    act(() => {
      result.current.togglePalette();
    });
    expect(result.current.paletteOpen).toBe(true);
    expect(persistedChrome()).toMatchObject({ context: { paletteOpen: true } });

    act(() => {
      result.current.toggleDock();
    });
    expect(result.current.dockCollapsed).toBe(false);
    expect(persistedChrome()).toMatchObject({
      context: { paletteOpen: true, dockCollapsed: false },
    });

    act(() => {
      result.current.togglePalette();
    });
    expect(result.current.paletteOpen).toBe(false);
    expect(persistedChrome()).toMatchObject({
      context: { paletteOpen: false, dockCollapsed: false },
    });
  });

  it("Should open the inspector contextually without disturbing the other surfaces", () => {
    const { result } = renderHook(() => useLoopEditorChromeState());

    act(() => {
      result.current.openInspector();
    });
    expect(result.current.inspectorOpen).toBe(true);
    expect(result.current.paletteOpen).toBe(false);
    expect(result.current.dockCollapsed).toBe(true);

    act(() => {
      result.current.openInspector();
    });
    expect(result.current.inspectorOpen).toBe(true);
  });

  it("Should share one preference across every editor mount", () => {
    const { result: first } = renderHook(() => useLoopEditorChromeState());
    const { result: second } = renderHook(() => useLoopEditorChromeState());

    act(() => {
      first.current.togglePalette();
    });
    expect(second.current.paletteOpen).toBe(true);
    expect(second.current.inspectorOpen).toBe(false);
  });

  it("Should adopt another tab's chrome preference when the shared key changes", async () => {
    const { result } = renderHook(() => useLoopEditorChromeState());

    await writeFromAnotherTab(
      JSON.stringify({
        context: { paletteOpen: true, inspectorOpen: true, dockCollapsed: false },
        version: 0,
      })
    );

    await waitFor(() => expect(result.current.paletteOpen).toBe(true));
    expect(result.current.inspectorOpen).toBe(true);
    expect(result.current.dockCollapsed).toBe(false);
  });

  it("Should ignore a hostile persisted value instead of adopting it", async () => {
    const { result } = renderHook(() => useLoopEditorChromeState());

    act(() => {
      result.current.togglePalette();
    });
    expect(result.current.paletteOpen).toBe(true);

    await writeFromAnotherTab(
      JSON.stringify({
        context: { paletteOpen: "closed", inspectorOpen: 1, dockCollapsed: null },
        version: 0,
      })
    );

    expect(result.current.paletteOpen).toBe(true);
    expect(result.current.inspectorOpen).toBe(false);
    expect(result.current.dockCollapsed).toBe(true);
  });

  it("Should apply only the named flags in a pure visibility transition", () => {
    const store = loopEditorChromeLogic.createStore();
    const [opened] = store.transition(store.getInitialSnapshot(), {
      type: "chromeVisibilityChanged",
      inspector: true,
    });
    expect(opened.context).toEqual({
      paletteOpen: false,
      inspectorOpen: true,
      dockCollapsed: true,
    });

    const [unfolded] = store.transition(opened, {
      type: "chromeVisibilityChanged",
      dockCollapsed: false,
    });
    expect(unfolded.context).toEqual({
      paletteOpen: false,
      inspectorOpen: true,
      dockCollapsed: false,
    });
  });
});

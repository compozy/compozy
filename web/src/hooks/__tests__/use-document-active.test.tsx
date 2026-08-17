// Suite: document activity boundary
// Invariant: the app is active only while its document is visible and its
// browser window owns focus.
// Owning layer: browser state hook. No prior suite owned this combined state.
import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useDocumentActive } from "../use-document-active";

describe("useDocumentActive", () => {
  afterEach(() => vi.restoreAllMocks());

  it("Should update when the visible browser window loses and regains focus", () => {
    let focused = true;
    vi.spyOn(document, "hasFocus").mockImplementation(() => focused);
    const { result } = renderHook(() => useDocumentActive());

    expect(result.current).toBe(true);
    act(() => {
      focused = false;
      window.dispatchEvent(new Event("blur"));
    });
    expect(result.current).toBe(false);
    act(() => {
      focused = true;
      window.dispatchEvent(new Event("focus"));
    });
    expect(result.current).toBe(true);
  });
});

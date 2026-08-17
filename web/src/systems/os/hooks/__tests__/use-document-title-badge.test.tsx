// Suite: document title badge
// Invariant: the tab title carries the cross-workspace needs-you total while it
// is above zero, returns to the clean title at zero, and never accumulates the
// count into its own base.
// Owning layer: unit (systems/os/hooks + lib)
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { formatTitleBadge, resetDocumentTitleBase } from "../../lib/document-title";
import { useDocumentTitleBadge } from "../use-document-title-badge";

const BASE_TITLE = "CompozyOS";

beforeEach(() => {
  resetDocumentTitleBase();
  document.title = BASE_TITLE;
});

describe("useDocumentTitleBadge (UT-054)", () => {
  it("Should carry the cross-workspace total in the title", () => {
    // Three needs-you sessions spread across two workspaces is still one number:
    // the operator is blocked on three things, wherever they live.
    renderHook(() => useDocumentTitleBadge(3));

    expect(document.title).toBe("(3) CompozyOS");
  });

  it("Should return to a clean title at zero", () => {
    const { rerender } = renderHook(({ count }) => useDocumentTitleBadge(count), {
      initialProps: { count: 4 },
    });
    expect(document.title).toBe("(4) CompozyOS");

    rerender({ count: 0 });

    expect(document.title).toBe(BASE_TITLE);
  });

  it("Should replace the count rather than nest it as the count changes", () => {
    const { rerender } = renderHook(({ count }) => useDocumentTitleBadge(count), {
      initialProps: { count: 1 },
    });

    rerender({ count: 2 });
    rerender({ count: 12 });

    expect(document.title).toBe("(12) CompozyOS");
  });

  it("Should print the exact number even past the menubar pill's 9+ cap", () => {
    renderHook(() => useDocumentTitleBadge(137));

    expect(document.title).toBe("(137) CompozyOS");
  });

  it("Should restore the clean title on unmount", () => {
    const { unmount } = renderHook(() => useDocumentTitleBadge(2));
    expect(document.title).toBe("(2) CompozyOS");

    unmount();

    expect(document.title).toBe(BASE_TITLE);
  });

  it("Should format over whatever base title the document declares", () => {
    expect(formatTitleBadge("CompozyOS", 0)).toBe("CompozyOS");
    expect(formatTitleBadge("CompozyOS", 5)).toBe("(5) CompozyOS");
  });
});

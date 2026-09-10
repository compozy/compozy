// Suite: document title badge
// Invariant: the tab title carries the cross-workspace needs-you total while it
// is above zero, returns to the clean title at zero, and never accumulates the
// count into its own base.
// Owning layer: unit (systems/os/hooks + lib)
import { act, renderHook } from "@testing-library/react";
import { focusManager, QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createElement, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/systems/profiles", () => ({
  useProfileReadScope: () => ({ destination: "default" }),
}));
vi.mock("@/systems/notifications/adapters/attention-api", () => ({
  listAttentionNotifications: vi.fn(),
  acknowledgeAttentionNotifications: vi.fn(),
}));

import { listAttentionNotifications } from "@/systems/notifications/adapters/attention-api";
import { useBellNotifications } from "../use-bell-notifications";

import { formatTitleBadge, resetDocumentTitleBase } from "../../lib/document-title";
import { useDocumentTitleBadge } from "../use-document-title-badge";

const BASE_TITLE = "CompozyOS";

beforeEach(() => {
  resetDocumentTitleBase();
  document.title = BASE_TITLE;
});

describe("useDocumentTitleBadge (UT-054)", () => {
  it("Should refresh unread counts and clear the title while the tab stays hidden", async () => {
    vi.useFakeTimers();
    const visibility = vi.spyOn(document, "visibilityState", "get").mockReturnValue("hidden");
    focusManager.setFocused(false);
    const client = new QueryClient();
    let count = 2;
    vi.mocked(listAttentionNotifications).mockImplementation(async () => ({
      snapshot: "snapshot",
      total: count,
      needs_you: count,
      finished: 0,
      items: [],
    }));
    const { unmount } = renderHook(
      () => useDocumentTitleBadge(useBellNotifications(new Set()).count),
      {
        wrapper: ({ children }: { children: ReactNode }) =>
          createElement(QueryClientProvider, { client }, children),
      }
    );
    try {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1);
      });
      expect(document.title).toBe("(2) CompozyOS");
      count = 3;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5_001);
      });
      expect(document.title).toBe("(3) CompozyOS");
      count = 0;
      await act(async () => {
        await vi.advanceTimersByTimeAsync(5_001);
      });
      expect(document.title).toBe(BASE_TITLE);
    } finally {
      unmount();
      client.clear();
      visibility.mockRestore();
      focusManager.setFocused(undefined);
      vi.useRealTimers();
    }
  });

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

import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  DETAIL_INSPECTOR_INLINE_BREAKPOINT,
  DETAIL_INSPECTOR_INLINE_WIDTH,
  DetailInspector,
  type DetailInspectorTab,
} from "../detail-inspector";

interface MediaMock {
  matches: boolean;
  media: string;
  onchange: null;
  addEventListener: ReturnType<typeof vi.fn>;
  removeEventListener: ReturnType<typeof vi.fn>;
  addListener: ReturnType<typeof vi.fn>;
  removeListener: ReturnType<typeof vi.fn>;
  dispatchEvent: () => boolean;
  _listeners: Set<(event: MediaQueryListEvent) => void>;
  _fire: (matches: boolean) => void;
}

const ORIGINAL_MATCH_MEDIA = window.matchMedia;
let lastMock: MediaMock | null = null;

function installMatchMedia(matches: boolean) {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: (query: string): MediaQueryList => {
      const listeners = new Set<(event: MediaQueryListEvent) => void>();
      const mock: MediaMock = {
        matches,
        media: query,
        onchange: null,
        addEventListener: vi.fn((_type: string, cb: (event: MediaQueryListEvent) => void) => {
          listeners.add(cb);
        }),
        removeEventListener: vi.fn((_type: string, cb: (event: MediaQueryListEvent) => void) => {
          listeners.delete(cb);
        }),
        addListener: vi.fn(),
        removeListener: vi.fn(),
        dispatchEvent: () => false,
        _listeners: listeners,
        _fire: next => {
          mock.matches = next;
          listeners.forEach(cb => cb({ matches: next } as MediaQueryListEvent));
        },
      };
      lastMock = mock;
      return mock as unknown as MediaQueryList;
    },
  });
}

beforeEach(() => {
  lastMock = null;
});

afterEach(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: ORIGINAL_MATCH_MEDIA,
  });
});

const TABS: DetailInspectorTab[] = [
  { id: "summary", label: "Summary" },
  { id: "events", label: "Events" },
];

describe("DetailInspector", () => {
  it("Should render inline at 320 px when viewport is at or above the breakpoint", () => {
    installMatchMedia(true);
    const { container } = render(
      <DetailInspector title="Inspector" tabs={TABS} activeTab="summary" onTabChange={() => {}}>
        <p>body</p>
      </DetailInspector>
    );

    const root = container.querySelector<HTMLElement>('[data-slot="detail-inspector"]');
    expect(root?.dataset.mode).toBe("inline");
    expect(root?.style.width).toBe(`${DETAIL_INSPECTOR_INLINE_WIDTH}px`);
    expect(root?.tagName.toLowerCase()).toBe("aside");
    expect(screen.getByText("body")).toBeInTheDocument();
    expect(screen.getByText("Summary")).toBeInTheDocument();
  });

  it("Should render the Sheet drawer below the breakpoint when open is true", () => {
    installMatchMedia(false);
    const { container } = render(
      <DetailInspector title="Inspector" open onOpenChange={() => {}}>
        <p>drawer body</p>
      </DetailInspector>
    );

    expect(
      container.querySelector('[data-slot="detail-inspector"][data-mode="inline"]')
    ).not.toBeInTheDocument();
    const sheet = document.querySelector<HTMLElement>(
      '[data-slot="detail-inspector"][data-mode="drawer"]'
    );
    expect(sheet).not.toBeNull();
    expect(screen.getByText("drawer body")).toBeInTheDocument();
  });

  it("Should subscribe to the matchMedia change event and clean up on unmount", () => {
    installMatchMedia(true);
    const { unmount } = render(
      <DetailInspector>
        <p>body</p>
      </DetailInspector>
    );

    expect(lastMock).not.toBeNull();
    expect(lastMock?.addEventListener).toHaveBeenCalledWith("change", expect.any(Function));

    unmount();

    expect(lastMock?.removeEventListener).toHaveBeenCalledWith("change", expect.any(Function));
    expect(lastMock?._listeners.size).toBe(0);
  });

  it("Should default the breakpoint to 1440", () => {
    expect(DETAIL_INSPECTOR_INLINE_BREAKPOINT).toBe(1440);
    expect(DETAIL_INSPECTOR_INLINE_WIDTH).toBe(320);
  });

  it("Should honour a custom inlineBreakpoint when computing the layout mode", () => {
    installMatchMedia(false);
    const { container } = render(
      <DetailInspector inlineBreakpoint={Number.MAX_SAFE_INTEGER} open>
        <p>body</p>
      </DetailInspector>
    );
    expect(
      container.querySelector('[data-slot="detail-inspector"][data-mode="inline"]')
    ).not.toBeInTheDocument();
    expect(lastMock?.media).toContain(`${Number.MAX_SAFE_INTEGER}`);
  });
});

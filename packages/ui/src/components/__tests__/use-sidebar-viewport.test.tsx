import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import {
  SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
  SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
  useSidebarViewport,
} from "../hooks/use-sidebar-viewport";

interface MediaMock {
  matches: boolean;
  media: string;
  onchange: null;
  addEventListener: (type: string, cb: (event: MediaQueryListEvent) => void) => void;
  removeEventListener: (type: string, cb: (event: MediaQueryListEvent) => void) => void;
  dispatchEvent: () => boolean;
  addListener: () => void;
  removeListener: () => void;
  _fire: (matches: boolean) => void;
}

function installMatchMedia(resolve: (query: string) => boolean) {
  const listeners = new Map<string, Set<(event: MediaQueryListEvent) => void>>();
  const instances = new Map<string, MediaMock>();

  const factory = (query: string): MediaQueryList => {
    if (instances.has(query)) return instances.get(query) as unknown as MediaQueryList;
    const mock: MediaMock = {
      matches: resolve(query),
      media: query,
      onchange: null,
      addEventListener: (_type, cb) => {
        const set = listeners.get(query) ?? new Set();
        set.add(cb);
        listeners.set(query, set);
      },
      removeEventListener: (_type, cb) => {
        listeners.get(query)?.delete(cb);
      },
      dispatchEvent: () => false,
      addListener: () => {},
      removeListener: () => {},
      _fire: matches => {
        mock.matches = matches;
        listeners.get(query)?.forEach(cb => cb({ matches } as MediaQueryListEvent));
      },
    };
    instances.set(query, mock);
    return mock as unknown as MediaQueryList;
  };

  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: factory,
  });

  return {
    fire(query: string, matches: boolean) {
      const mock = instances.get(query);
      if (mock) mock._fire(matches);
    },
  };
}

const ORIGINAL_MATCH_MEDIA = window.matchMedia;

afterEach(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    configurable: true,
    value: ORIGINAL_MATCH_MEDIA,
  });
});

describe("useSidebarViewport", () => {
  it("Should expose useSidebarViewport returning 'default' when no query matches", () => {
    installMatchMedia(() => false);
    const { result } = renderHook(() =>
      useSidebarViewport({
        drawer: SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
        md: SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
      })
    );
    expect(result.current).toBe("default");
  });

  it("Should expose useSidebarViewport returning 'md' when only the md query matches", () => {
    installMatchMedia(query => {
      const match = query.match(/max-width:\s*(\d+)px/);
      if (!match) return false;
      const px = Number(match[1]);
      return px === SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT - 1;
    });
    const { result } = renderHook(() =>
      useSidebarViewport({
        drawer: SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
        md: SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
      })
    );
    expect(result.current).toBe("md");
  });

  it("Should expose useSidebarViewport returning 'drawer' when the drawer query matches", () => {
    installMatchMedia(() => true);
    const { result } = renderHook(() =>
      useSidebarViewport({
        drawer: SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
        md: SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
      })
    );
    expect(result.current).toBe("drawer");
  });
});

// Suite: docs page hooks
// Invariant: An overlay belongs to the page it was opened on: navigating closes it for good, the
// drawer closes once the window reaches the rail breakpoint, and pressing ⌘K inside the open palette
// keeps the element that first opened it. The section tab strip scrolls only when the current tab
// is out of view. A page loaded with a fragment to an id inside a closed findable panel opens it.
// Boundary IN: useDocsOverlays, useCurrentTabInView, useOpenAtFragment.
// Boundary OUT: The drawer and palette markup; real layout (boxes are stubbed).
import { act, fireEvent, render, renderHook, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useCurrentTabInView } from "@/components/docs/hooks/use-current-tab-in-view";
import { useDocsOverlays } from "@/components/docs/hooks/use-docs-overlays";
import { useOpenAtFragment } from "@/components/docs/hooks/use-open-at-fragment";

afterEach(() => {
  vi.restoreAllMocks();
  document.body.innerHTML = "";
});

describe("useDocsOverlays", () => {
  function overlays(pathname = "/docs/sessions") {
    return renderHook(({ path }) => useDocsOverlays(path), { initialProps: { path: pathname } });
  }

  it("closes an overlay on navigation and does not reopen it when the reader comes back", () => {
    const hook = overlays();
    act(() => hook.result.current.openMenu());
    expect(hook.result.current.menu).toBe(true);

    hook.rerender({ path: "/docs/terminal" });
    expect(hook.result.current.menu).toBe(false);
    hook.rerender({ path: "/docs/sessions" });
    expect(hook.result.current.menu).toBe(false);
    expect(hook.result.current.search).toBe(false);
  });

  it("closes the drawer when the window widens to the rail breakpoint", () => {
    const listeners = new Set<() => void>();
    const rail = { matches: false };
    vi.spyOn(window, "matchMedia").mockImplementation(
      query =>
        ({
          media: query,
          get matches() {
            return rail.matches;
          },
          addEventListener: (_: string, listener: () => void) => listeners.add(listener),
          removeEventListener: (_: string, listener: () => void) => listeners.delete(listener),
        }) as unknown as MediaQueryList
    );
    const hook = overlays();
    act(() => hook.result.current.openMenu());
    expect(hook.result.current.menu).toBe(true);

    rail.matches = true;
    act(() => listeners.forEach(listener => listener()));
    expect(hook.result.current.menu).toBe(false);
  });

  it("keeps the first opener when ⌘K is pressed again inside the open palette", () => {
    document.body.innerHTML =
      '<button id="opener">Search</button><div role="dialog"><input aria-label="Query" /></div>';
    const opener = screen.getByRole("button", { name: "Search" });
    const hook = overlays();
    opener.focus();
    fireEvent.keyDown(window, { key: "k", metaKey: true });
    expect(hook.result.current.search).toBe(true);
    expect(hook.result.current.searchOpener.current).toBe(opener);

    screen.getByRole("textbox", { name: "Query" }).focus();
    fireEvent.keyDown(window, { key: "k", metaKey: true });
    expect(hook.result.current.searchOpener.current).toBe(opener);
  });
});

describe("useCurrentTabInView", () => {
  const STRIP_WIDTH = 390;
  const TAB_WIDTH = 100;
  /** Where each tab starts in the strip's content: Reference is past the right edge. */
  const TAB_LEFT: Record<string, number> = { Guides: 20, Concepts: 200, Reference: 600 };

  function Strip({ current }: { current: string }) {
    const strip = useCurrentTabInView(current);
    return (
      <nav ref={strip} data-testid="strip">
        {Object.keys(TAB_LEFT).map(tab => (
          <a key={tab} href={`#${tab}`} aria-current={tab === current ? "page" : undefined}>
            {tab}
          </a>
        ))}
      </nav>
    );
  }

  it("scrolls the strip only when the current tab is out of view", () => {
    let scrollLeft = 0;
    const box = (left: number, width: number) =>
      ({ left, right: left + width, width, top: 0, bottom: 48, height: 48 }) as DOMRect;
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (
      this: HTMLElement
    ) {
      if (this.tagName === "NAV") return box(0, STRIP_WIDTH);
      return box((TAB_LEFT[this.textContent ?? ""] ?? 0) - scrollLeft, TAB_WIDTH);
    });
    const view = render(<Strip current="Guides" />);
    const strip = screen.getByTestId("strip");
    Object.defineProperties(strip, {
      scrollWidth: { configurable: true, value: 800 },
      clientWidth: { configurable: true, value: STRIP_WIDTH },
      scrollLeft: {
        configurable: true,
        get: () => scrollLeft,
        set: (value: number) => {
          scrollLeft = value;
        },
      },
    });
    scrollLeft = 10;
    view.rerender(<Strip current="Concepts" />);
    expect(scrollLeft).toBe(10);

    view.rerender(<Strip current="Reference" />);
    // Centred: 600 into the content, less half the room around the tab.
    expect(scrollLeft).toBe(600 - (STRIP_WIDTH - TAB_WIDTH) / 2);
  });
});

describe("useOpenAtFragment", () => {
  function Panel() {
    const [open, setOpen] = useState(false);
    const frame = useOpenAtFragment<HTMLDivElement>(open, setOpen);
    return (
      <div ref={frame}>
        <p>{open ? "open" : "closed"}</p>
        <span id="response-field">session_id</span>
      </div>
    );
  }

  afterEach(() => {
    window.location.hash = "";
  });

  it("opens the panel holding the loaded fragment's target and scrolls to it", () => {
    // jsdom has no scrolling: record which element asked to be scrolled into view.
    const scrolled = vi.fn();
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      value(this: HTMLElement) {
        scrolled(this.id);
      },
    });
    try {
      window.location.hash = "#response-field";
      render(<Panel />);
      expect(screen.getByText("open")).toBeTruthy();
      expect(scrolled).toHaveBeenCalledWith("response-field");
    } finally {
      Reflect.deleteProperty(HTMLElement.prototype, "scrollIntoView");
    }
  });

  it("leaves the page working when the fragment is a malformed escape", () => {
    window.location.hash = "#100%";
    render(<Panel />);
    expect(screen.getByText("closed")).toBeTruthy();
  });

  it("leaves the panel closed when the fragment names an id outside it", () => {
    document.body.innerHTML = '<h2 id="elsewhere">Elsewhere</h2>';
    window.location.hash = "#elsewhere";
    render(<Panel />);
    expect(screen.getByText("closed")).toBeTruthy();
  });
});

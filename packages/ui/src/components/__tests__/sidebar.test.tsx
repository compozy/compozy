import { renderHook, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT,
  SIDEBAR_PANEL_WIDTH_DEFAULT,
  SIDEBAR_PANEL_WIDTH_MD,
  SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT,
  SIDEBAR_RAIL_WIDTH,
  Sidebar,
  useSidebarViewport,
} from "../sidebar";
import { UIProvider } from "../custom/ui-provider";

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

describe("Sidebar", () => {
  it("Should render rail, header, nav, and footer slots when provided", () => {
    render(
      <Sidebar
        rail={<span data-testid="rail-content">rail</span>}
        header={<span data-testid="header-content">header</span>}
        nav={<span data-testid="nav-content">nav</span>}
        footer={<span data-testid="footer-content">footer</span>}
      />
    );

    expect(screen.getByTestId("rail-content")).toBeInTheDocument();
    expect(screen.getByTestId("header-content")).toBeInTheDocument();
    expect(screen.getByTestId("nav-content")).toBeInTheDocument();
    expect(screen.getByTestId("footer-content")).toBeInTheDocument();
  });

  it("Should expose the rail at 56px regardless of collapsed state", () => {
    const { container, rerender } = render(<Sidebar nav={<span>nav</span>} collapsed={false} />);
    const rail = container.querySelector<HTMLElement>("[data-slot=sidebar-rail]");
    expect(rail).not.toBeNull();
    expect(rail?.style.width).toBe("56px");

    rerender(<Sidebar nav={<span>nav</span>} collapsed={true} />);
    expect(rail?.style.width).toBe("56px");
  });

  it("Should export the viewport ladder constants", () => {
    expect(SIDEBAR_RAIL_WIDTH).toBe(56);
    expect(SIDEBAR_PANEL_WIDTH_DEFAULT).toBe(244);
    expect(SIDEBAR_PANEL_WIDTH_MD).toBe(220);
    expect(SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT).toBe(1100);
    expect(SIDEBAR_COLLAPSE_BREAKPOINT_DEFAULT).toBe(880);
  });

  it("Should render a narrow scrim that closes the sidebar when activated", async () => {
    const user = userEvent.setup();
    installMatchMedia(q => q.includes("max-width"));
    const { container } = render(
      <UIProvider reducedMotion="always">
        <Sidebar nav={<button type="button">nav</button>} />
      </UIProvider>
    );
    await user.click(screen.getByRole("button", { name: "Open sidebar navigation" }));
    const scrim = container.querySelector<HTMLElement>(
      "aside button[aria-label='Close sidebar navigation']"
    );
    expect(scrim).not.toBeNull();
  });

  it("Should call onCollapse(true) when the collapse control is activated from expanded", async () => {
    const onCollapse = vi.fn();
    const user = userEvent.setup();
    render(<Sidebar nav={<span>nav</span>} collapsed={false} onCollapse={onCollapse} />);

    const trigger = screen.getByRole("button", { name: "Toggle sidebar" });
    expect(trigger).toHaveAttribute("aria-expanded", "true");

    await user.click(trigger);

    expect(onCollapse).toHaveBeenCalledWith(true);
  });

  it("Should flip aria-expanded and data-state when uncontrolled collapse toggles", async () => {
    const user = userEvent.setup();
    const { container } = render(<Sidebar nav={<span>nav</span>} />);

    const trigger = screen.getByRole("button", { name: "Toggle sidebar" });
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(container.querySelector("[data-slot=sidebar]")).toHaveAttribute(
      "data-state",
      "expanded"
    );

    await user.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(container.querySelector("[data-slot=sidebar]")).toHaveAttribute(
      "data-state",
      "collapsed"
    );
  });

  it("Should mark the panel aria-hidden when collapsed", () => {
    const { container, rerender } = render(<Sidebar nav={<span>nav</span>} collapsed={false} />);
    const panel = container.querySelector<HTMLElement>("[data-slot=sidebar-panel]");
    expect(panel).toHaveAttribute("aria-hidden", "false");
    expect(panel).not.toHaveAttribute("inert");

    rerender(<Sidebar nav={<span>nav</span>} collapsed={true} />);
    expect(panel).toHaveAttribute("aria-hidden", "true");
    expect(panel).toHaveAttribute("inert");
  });

  it("Should drive the motion panel width from the collapsed prop", async () => {
    const { container, rerender } = render(
      <UIProvider reducedMotion="always">
        <Sidebar nav={<span>nav</span>} panelWidth={260} collapsed={false} />
      </UIProvider>
    );
    const panel = container.querySelector<HTMLElement>("[data-slot=sidebar-panel]");
    expect(panel).not.toBeNull();

    await waitFor(() => expect(panel?.style.width).toBe("260px"));

    rerender(
      <UIProvider reducedMotion="always">
        <Sidebar nav={<span>nav</span>} panelWidth={260} collapsed={true} />
      </UIProvider>
    );

    await waitFor(() => expect(panel?.style.width).toBe("0px"));
  });

  it("Should auto-collapse the panel when the viewport drops below the drawer breakpoint", () => {
    installMatchMedia(q => q.includes("max-width"));
    const { container } = render(
      <Sidebar nav={<span>nav</span>} collapsed={false} collapseBreakpoint={800} />
    );
    const sidebar = container.querySelector<HTMLElement>("[data-slot=sidebar]");
    expect(sidebar).toHaveAttribute("data-state", "collapsed");
    expect(sidebar).toHaveAttribute("data-narrow", "true");
    expect(sidebar).toHaveAttribute("data-viewport", "drawer");
  });

  it("Should render the 220 px panel width when only the md breakpoint matches", async () => {
    installMatchMedia(query => {
      const match = query.match(/max-width:\s*(\d+)px/);
      if (!match) return false;
      const px = Number(match[1]);
      return px === SIDEBAR_PANEL_WIDTH_MD_BREAKPOINT - 1;
    });
    const { container } = render(
      <UIProvider reducedMotion="always">
        <Sidebar nav={<span>nav</span>} collapsed={false} />
      </UIProvider>
    );
    const sidebar = container.querySelector<HTMLElement>("[data-slot=sidebar]");
    const panel = container.querySelector<HTMLElement>("[data-slot=sidebar-panel]");
    expect(sidebar).toHaveAttribute("data-viewport", "md");
    await waitFor(() => expect(panel?.style.width).toBe(`${SIDEBAR_PANEL_WIDTH_MD}px`));
  });

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

  it("Should open a narrow-viewport panel without mutating desktop collapse state", async () => {
    const onCollapse = vi.fn();
    const user = userEvent.setup();
    installMatchMedia(q => q.includes("max-width"));

    const { container } = render(
      <UIProvider reducedMotion="always">
        <Sidebar nav={<button type="button">nav action</button>} onCollapse={onCollapse} />
      </UIProvider>
    );

    const trigger = screen.getByRole("button", { name: "Open sidebar navigation" });
    const panel = container.querySelector<HTMLElement>("[data-slot=sidebar-panel]");
    expect(panel).toHaveAttribute("aria-hidden", "true");

    await user.click(trigger);

    expect(trigger).toHaveAttribute("aria-label", "Close sidebar navigation");
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(container.querySelector("[data-slot=sidebar]")).toHaveAttribute(
      "data-state",
      "expanded"
    );
    expect(panel).toHaveAttribute("aria-hidden", "false");
    await waitFor(() => expect(panel?.style.width).toBe("244px"));
    expect(onCollapse).not.toHaveBeenCalled();
  });
});

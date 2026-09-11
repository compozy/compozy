// Suite: desktop pager
// Invariant: visible controls preserve ordered desktop navigation and disclose every hidden range.
// Boundary IN: DesktopPager control projection, callbacks, accessibility, and keyboard navigation.
// Boundary OUT: overview rendering, shell positioning measurements, persistence, and browser layout.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DesktopPager, type DesktopPagerItem } from "../desktop-pager";

const DESKTOPS: DesktopPagerItem[] = [
  { id: "control", name: "Control" },
  { id: "build", name: "Build" },
  { id: "review", name: "Review" },
  { id: "research", name: "Research" },
];

const MANY_DESKTOPS: DesktopPagerItem[] = [
  ...DESKTOPS,
  { id: "qa", name: "QA" },
  { id: "docs", name: "Docs" },
  { id: "release", name: "Release" },
  { id: "incidents", name: "Incidents" },
  { id: "archive", name: "Archive" },
];

function renderPager(
  desktops = DESKTOPS,
  activeDesktopId = "build",
  compact = false,
  canSwitchDesktop = true
) {
  const onSelectDesktop = vi.fn();
  const onOpenOverview = vi.fn();
  render(
    <DesktopPager
      desktops={desktops}
      activeDesktopId={activeDesktopId}
      compact={compact}
      canSwitchDesktop={canSwitchDesktop}
      onSelectDesktop={onSelectDesktop}
      onOpenOverview={onOpenOverview}
    />
  );
  return { onSelectDesktop, onOpenOverview };
}

describe("DesktopPager", () => {
  it("Should render all desktops through seven and identify the active position", () => {
    renderPager(MANY_DESKTOPS.slice(0, 7));

    const active = screen.getByRole("button", { name: "Desktop 2 of 7: Build" });
    expect(active).toHaveAttribute("aria-current", "page");
    expect(active).toHaveAttribute("tabindex", "0");
    expect(screen.getAllByRole("button")).toHaveLength(7);
    expect(screen.queryByRole("button", { name: /earlier desktops/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /later desktops/ })).not.toBeInTheDocument();
  });

  it("Should expose adaptive earlier and later ranges to the overview callback", async () => {
    const user = userEvent.setup();
    const { onOpenOverview } = renderPager(MANY_DESKTOPS, "qa");

    await user.click(screen.getByRole("button", { name: "Show 2 earlier desktops" }));
    expect(onOpenOverview).toHaveBeenLastCalledWith({
      direction: "earlier",
      hiddenDesktopIds: ["control", "build"],
      anchorDesktopId: "qa",
    });

    await user.click(screen.getByRole("button", { name: "Show 2 later desktops" }));
    expect(onOpenOverview).toHaveBeenLastCalledWith({
      direction: "later",
      hiddenDesktopIds: ["incidents", "archive"],
      anchorDesktopId: "qa",
    });
    expect(screen.getByRole("button", { name: "Desktop 5 of 9: QA" })).toHaveAttribute(
      "aria-current",
      "page"
    );
  });

  it("Should start overflow at eight desktops", () => {
    renderPager(MANY_DESKTOPS.slice(0, 8), "qa");

    expect(screen.getAllByRole("button")).toHaveLength(7);
    expect(screen.getByRole("button", { name: "Show 2 earlier desktops" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Show 1 later desktop" })).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Desktop 1 of 8: Control" })
    ).not.toBeInTheDocument();
  });

  it("Should navigate without wrapping and activate the focused desktop", async () => {
    const user = userEvent.setup();
    const { onSelectDesktop } = renderPager();
    const active = screen.getByRole("button", { name: "Desktop 2 of 4: Build" });
    await user.click(active);

    await user.keyboard("{ArrowRight}{End}{ArrowRight}");
    const last = screen.getByRole("button", { name: "Desktop 4 of 4: Research" });
    expect(last).toHaveFocus();

    await user.keyboard(" ");
    expect(onSelectDesktop).toHaveBeenCalledWith("research");

    await user.keyboard("{Home}{ArrowLeft}{Enter}");
    const first = screen.getByRole("button", { name: "Desktop 1 of 4: Control" });
    expect(first).toHaveFocus();
    expect(onSelectDesktop).toHaveBeenCalledWith("control");
  });

  it("Should preserve the floating projection and bounded overflow in compact mode", async () => {
    const user = userEvent.setup();
    const { onOpenOverview } = renderPager(MANY_DESKTOPS, "qa", true);

    expect(screen.getAllByRole("button")).toHaveLength(7);
    expect(screen.getByRole("button", { name: "Desktop 5 of 9: QA" })).toHaveAttribute(
      "aria-current",
      "page"
    );

    await user.click(screen.getByRole("button", { name: "Show 2 earlier desktops" }));
    expect(onOpenOverview).toHaveBeenLastCalledWith({
      direction: "earlier",
      hiddenDesktopIds: ["control", "build"],
      anchorDesktopId: "qa",
    });

    await user.click(screen.getByRole("button", { name: "Show 2 later desktops" }));
    expect(onOpenOverview).toHaveBeenLastCalledWith({
      direction: "later",
      hiddenDesktopIds: ["incidents", "archive"],
      anchorDesktopId: "qa",
    });
  });

  it("Should keep overview disclosure available while desktop switching is unavailable", async () => {
    const user = userEvent.setup();
    const { onOpenOverview, onSelectDesktop } = renderPager(MANY_DESKTOPS, "qa", false, false);

    const desktop = screen.getByRole("button", { name: "Desktop 4 of 9: Research" });
    expect(desktop).toHaveAttribute("aria-disabled", "true");
    await user.click(desktop);
    expect(onSelectDesktop).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Show 2 earlier desktops" }));
    expect(onOpenOverview).toHaveBeenCalledTimes(1);
  });

  // T9 real-device delta: a lone desktop is a position indicator, not a
  // switcher — at the touch dock it rendered as an orphaned pill beside a
  // 50vw dead zone, so compact renders nothing for a single desktop.
  it("Should render nothing in compact mode with a single desktop", () => {
    const { container } = render(
      <DesktopPager
        desktops={[{ id: "only", name: "Only" }]}
        activeDesktopId="only"
        compact
        onSelectDesktop={vi.fn()}
        onOpenOverview={vi.fn()}
      />
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("Should keep the single-desktop position pill in the floating dock", () => {
    render(
      <DesktopPager
        desktops={[{ id: "only", name: "Only" }]}
        activeDesktopId="only"
        onSelectDesktop={vi.fn()}
        onOpenOverview={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "Desktop 1 of 1: Only" })).toBeInTheDocument();
  });

  it("Should render every desktop in compact mode once switching exists", () => {
    renderPager(DESKTOPS.slice(0, 2), "build", true);

    expect(screen.getByRole("button", { name: "Desktop 1 of 2: Control" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Desktop 2 of 2: Build" })).toHaveAttribute(
      "aria-current",
      "page"
    );
    expect(screen.getByRole("navigation", { name: "Desktops" })).toHaveAttribute(
      "data-presentation",
      "compact"
    );
  });
});

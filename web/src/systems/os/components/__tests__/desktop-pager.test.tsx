// Suite: desktop pager
// Invariant: visible controls preserve ordered desktop navigation, disclose every hidden range, and
// mark off-screen desktops that need you.
// Boundary IN: DesktopPager control projection, callbacks, accessibility, keyboard navigation, and the
// desktop needs-you projection.
// Boundary OUT: overview rendering, shell positioning measurements, persistence, and browser layout.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { desktopIdsNeedingYou } from "../../lib/desktop-pager-attention";
import type { OsAttentionRow } from "../../lib/attention-model";
import type { OsWindow } from "../../lib/os-types";
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

function renderPager(desktops = DESKTOPS, activeDesktopId = "build", canSwitchDesktop = true) {
  const onSelectDesktop = vi.fn();
  const onOpenOverview = vi.fn();
  render(
    <DesktopPager
      desktops={desktops}
      activeDesktopId={activeDesktopId}
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

  it("Should mark an off-screen desktop that needs you and clear the mark once it is active", () => {
    const needsYou = DESKTOPS.map(desktop =>
      desktop.id === "research" || desktop.id === "build" ? { ...desktop, needsYou: true } : desktop
    );
    renderPager(needsYou, "build");

    const offScreen = screen.getByRole("button", {
      name: "Desktop 4 of 4: Research — needs you",
    });
    expect(offScreen).toHaveAttribute("data-needs-you", "true");
    const active = screen.getByRole("button", { name: "Desktop 2 of 4: Build" });
    expect(active).not.toHaveAttribute("data-needs-you");
  });

  it("Should only select with dots, leaving the active dot inert", async () => {
    const user = userEvent.setup();
    const { onSelectDesktop, onOpenOverview } = renderPager();

    await user.click(screen.getByRole("button", { name: "Desktop 2 of 4: Build" }));
    expect(onSelectDesktop).not.toHaveBeenCalled();
    expect(onOpenOverview).not.toHaveBeenCalled();
  });

  it("Should keep overview disclosure available while desktop switching is unavailable", async () => {
    const user = userEvent.setup();
    const { onOpenOverview, onSelectDesktop } = renderPager(MANY_DESKTOPS, "qa", false);

    const desktop = screen.getByRole("button", { name: "Desktop 4 of 9: Research" });
    expect(desktop).toHaveAttribute("aria-disabled", "true");
    await user.click(desktop);
    expect(onSelectDesktop).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Show 2 earlier desktops" }));
    expect(onOpenOverview).toHaveBeenCalledTimes(1);
  });
});

describe("desktopIdsNeedingYou", () => {
  function sessionWindow(id: string, sessionId: string, desktopId: string): OsWindow {
    return {
      id,
      app: "session",
      instanceKey: sessionId,
      desktopId,
    } as OsWindow;
  }

  function sessionRow(id: string, stale = false): OsAttentionRow {
    return {
      kind: "session",
      id,
      title: id,
      agentName: "codex",
      workspaceId: "ws",
      workspaceLabel: "ws",
      badge: "needs-attention",
      reason: "Needs input",
      changedAt: "2026-09-29T00:00:00Z",
      muted: false,
      stale,
    } as OsAttentionRow;
  }

  it("Should return the desktops whose session windows need you, ignoring stale rows", () => {
    const windows = {
      a: sessionWindow("a", "sess-a", "desktop:one"),
      b: sessionWindow("b", "sess-b", "desktop:two"),
      c: sessionWindow("c", "sess-c", "desktop:three"),
      tasks: {
        id: "tasks",
        app: "tasks",
        instanceKey: "sess-a",
        desktopId: "desktop:four",
      } as OsWindow,
    };

    const ids = desktopIdsNeedingYou(windows, [sessionRow("sess-a"), sessionRow("sess-c", true)]);

    expect([...ids]).toEqual(["desktop:one"]);
  });
});

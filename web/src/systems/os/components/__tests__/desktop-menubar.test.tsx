// Suite: DesktopMenubar scope-control wiring
// Invariant: while scope resolution is pending the globe control is
// aria-disabled, matching the runtime-workspace query lock at the root.
// Owning layer: desktop-menubar.tsx. Canonical suite: this file.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { OsAttentionModel } from "../../hooks/use-os-attention";
import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import { paletteRegistryFixture } from "../../mocks/cmd-palette-fixtures";
import { DesktopMenubar } from "../desktop-menubar";

vi.mock("../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: { hydration: "live" }) => unknown) =>
    selector({ hydration: "live" }),
}));

vi.mock("../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userOpen: vi.fn() } }),
}));

const MENUBAR_ACTIONS_STATE = vi.hoisted(() => ({ touchViewport: false }));

vi.mock("../../hooks/use-menubar-actions", () => ({
  useMenubarActions: () => ({
    menusVisible: false,
    touchViewport: MENUBAR_ACTIONS_STATE.touchViewport,
    canOpenApps: false,
    windowCommands: {},
    openApp: vi.fn(),
    openUpdates: vi.fn(),
    newAgent: vi.fn(),
  }),
}));

vi.mock("../../hooks/use-attention-jump", () => ({
  useAttentionJump: () => vi.fn(),
}));

const ATTENTION: OsAttentionModel = {
  badges: {},
  notificationCount: 0,
  sections: { needsYou: [], finished: [] },
  sessions: [],
  attentionSessionsDisconnected: false,
  sessionsDisconnected: false,
  tasksDisconnected: false,
  loopRequestsDisconnected: false,
  loading: false,
};

describe("DesktopMenubar scope control", () => {
  it("Should aria-disable the scope control while scope resolution is pending [RA0289]", () => {
    const queryClient = new QueryClient();
    render(
      <QueryClientProvider client={queryClient}>
        <UIProvider reducedMotion="always">
          <CmdPaletteRegistryProvider registry={paletteRegistryFixture([])}>
            <DesktopMenubar
              workspaces={[]}
              activeWorkspace={undefined}
              scope="workspace"
              scopePending
              onSelectWorkspace={vi.fn()}
              onAddWorkspace={vi.fn()}
              onRunCommand={vi.fn()}
              activeOverlay={null}
              onOverlayOpenChange={vi.fn()}
              attention={ATTENTION}
              updateAvailable={false}
            />
          </CmdPaletteRegistryProvider>
        </UIProvider>
      </QueryClientProvider>
    );

    expect(screen.getByTestId("os-global-scope-toggle")).toHaveAttribute("aria-disabled", "true");
  });

  // F3 real-device delta: at 390px the shrink-0 profile text row forced the
  // identity segment under the trailing action cluster. It is the
  // lowest-priority slot, so the touch tier drops it — absent, not cramped.
  it("Should drop the profile switcher from the bar at the touch tier", () => {
    MENUBAR_ACTIONS_STATE.touchViewport = true;
    try {
      const { container } = render(
        <UIProvider reducedMotion="always">
          <CmdPaletteRegistryProvider registry={paletteRegistryFixture([])}>
            <DesktopMenubar
              workspaces={[]}
              activeWorkspace={undefined}
              scope="global"
              onSelectWorkspace={vi.fn()}
              onAddWorkspace={vi.fn()}
              onRunCommand={vi.fn()}
              activeOverlay={null}
              onOverlayOpenChange={vi.fn()}
              attention={ATTENTION}
              updateAvailable={false}
              profileSwitcher={<div data-testid="profile-switcher-slot" />}
            />
          </CmdPaletteRegistryProvider>
        </UIProvider>
      );

      expect(screen.queryByTestId("profile-switcher-slot")).not.toBeInTheDocument();
      expect(container.querySelector('[data-slot="os-menubar-settings"]')).not.toBeNull();
    } finally {
      MENUBAR_ACTIONS_STATE.touchViewport = false;
    }
  });

  it("Should keep the profile switcher slot on the desktop tier", () => {
    render(
      <UIProvider reducedMotion="always">
        <CmdPaletteRegistryProvider registry={paletteRegistryFixture([])}>
          <DesktopMenubar
            workspaces={[]}
            activeWorkspace={undefined}
            scope="global"
            onSelectWorkspace={vi.fn()}
            onAddWorkspace={vi.fn()}
            onRunCommand={vi.fn()}
            activeOverlay={null}
            onOverlayOpenChange={vi.fn()}
            attention={ATTENTION}
            updateAvailable={false}
            profileSwitcher={<div data-testid="profile-switcher-slot" />}
          />
        </CmdPaletteRegistryProvider>
      </UIProvider>
    );

    expect(screen.getByTestId("profile-switcher-slot")).toBeInTheDocument();
  });
});

// Suite: DesktopMenubar scope-control wiring
// Invariant: while scope resolution is pending the globe control is
// aria-disabled, matching the runtime-workspace query lock at the root.
// Owning layer: desktop-menubar.tsx. Canonical suite: this file.
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

vi.mock("../../hooks/use-menubar-actions", () => ({
  useMenubarActions: () => ({
    menusVisible: false,
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
    render(
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
    );

    expect(screen.getByTestId("os-global-scope-toggle")).toHaveAttribute("aria-disabled", "true");
  });
});

// Suite: DesktopMenubar scope-control and attention-row navigation
// Invariants: while scope resolution is pending the globe control is
// aria-disabled, matching the runtime-workspace query lock at the root; and an
// attention row opens the surface that owns its subject.
// Owning layer: desktop-menubar.tsx. Canonical suite: this file.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { OsAttentionModel } from "../../hooks/use-os-attention";
import type { OsCallAttentionRow } from "../../lib/attention-model";
import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import { paletteRegistryFixture } from "../../mocks/cmd-palette-fixtures";
import { DesktopMenubar } from "../desktop-menubar";

vi.mock("../../hooks/use-desktop", () => ({
  useDesktop: (selector: (state: { hydration: "live" }) => unknown) =>
    selector({ hydration: "live" }),
}));

const { userOpen } = vi.hoisted(() => ({ userOpen: vi.fn() }));

vi.mock("../../hooks/use-os-shell", () => ({
  useOsShell: () => ({ coordinator: { userOpen } }),
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
  callsDisconnected: false,
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

describe("DesktopMenubar attention navigation", () => {
  function callRow(overrides: Partial<OsCallAttentionRow> = {}): OsCallAttentionRow {
    return {
      kind: "call",
      id: "call_bad",
      cause: "invalid-result",
      title: "reviewer · the answer never matched the contract",
      callId: "call_bad",
      rootSessionId: "ses_root",
      changedAt: "2026-08-20T18:40:00Z",
      count: 1,
      stale: false,
      ...overrides,
    };
  }

  function renderBell(row: OsCallAttentionRow) {
    userOpen.mockClear();
    render(
      <UIProvider reducedMotion="always">
        <CmdPaletteRegistryProvider registry={paletteRegistryFixture([])}>
          <DesktopMenubar
            workspaces={[]}
            activeWorkspace={undefined}
            scope="workspace"
            onSelectWorkspace={vi.fn()}
            onAddWorkspace={vi.fn()}
            onRunCommand={vi.fn()}
            activeOverlay="bell"
            onOverlayOpenChange={vi.fn()}
            attention={{ ...ATTENTION, sections: { needsYou: [row], finished: [] } }}
            updateAvailable={false}
          />
        </CmdPaletteRegistryProvider>
      </UIProvider>
    );
  }

  it("Should open the exact call from a singleton delegation row", async () => {
    // Without a `call` branch this row fell through to the tasks open and
    // navigated to `/tasks/call_bad`, a route that cannot resolve.
    const user = userEvent.setup();
    renderBell(callRow());

    await user.click(screen.getByTestId("os-attention-call-call_bad"));

    expect(userOpen).toHaveBeenCalledTimes(1);
    expect(userOpen.mock.calls[0]![0]).toMatchObject({
      app: "agents",
      route: { pathname: "/agents/calls/call_bad" },
    });
  });

  it("Should open Activity from a coalesced tree row, whose id is not a call id", async () => {
    // `tree:<root>` would have produced `/tasks/tree%3Ases_root`.
    const user = userEvent.setup();
    renderBell(callRow({ id: "tree:ses_root", count: 3 }));

    await user.click(screen.getByTestId("os-attention-call-tree:ses_root"));

    expect(userOpen.mock.calls[0]![0]).toMatchObject({
      app: "agents",
      route: {
        pathname: "/agents/activity",
        search: { root: "ses_root", call: "call_bad" },
      },
    });
  });
});

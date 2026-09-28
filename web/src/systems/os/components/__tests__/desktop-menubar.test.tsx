// Suite: DesktopMenubar scope-control wiring
// Invariant: while scope resolution is pending the globe control is
// aria-disabled, matching the runtime-workspace query lock at the root.
// The command-palette control carries the live palette chord as
// aria-keyshortcuts once the keymap is known, and nothing before.
// Owning layer: desktop-menubar.tsx. Canonical suite: this file.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import type { OsAttentionModel } from "../../hooks/use-os-attention";
import { CmdPaletteRegistryProvider } from "../../contexts/cmd-palette-registry-context";
import { paletteRegistryFixture, resolvedPaletteCommand } from "../../mocks/cmd-palette-fixtures";
import type { ResolvedPaletteCommand } from "../../lib/cmd-palette-types";
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

function renderMenubar({
  commands = [],
  scopePending = false,
}: {
  commands?: readonly ResolvedPaletteCommand[];
  scopePending?: boolean;
} = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { enabled: false, retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="always">
        <CmdPaletteRegistryProvider registry={paletteRegistryFixture(commands)}>
          <DesktopMenubar
            workspaces={[]}
            activeWorkspace={undefined}
            scope="workspace"
            scopePending={scopePending}
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
}

describe("DesktopMenubar scope control", () => {
  it("Should aria-disable the scope control while scope resolution is pending [RA0289]", () => {
    renderMenubar({ scopePending: true });

    expect(screen.getByTestId("os-global-scope-toggle")).toHaveAttribute("aria-disabled", "true");
  });
});

describe("DesktopMenubar command palette control", () => {
  it("Should expose the live palette chord as aria-keyshortcuts", () => {
    const { container } = renderMenubar({
      commands: [
        resolvedPaletteCommand({
          id: "palette.open",
          title: "Command palette",
          bindings: ["meta+KeyK"],
          chords: ["⌘K"],
        }),
      ],
    });

    expect(container.querySelector('[data-slot="os-menubar-command"]')).toHaveAttribute(
      "aria-keyshortcuts",
      "Meta+K"
    );
  });

  it("Should omit aria-keyshortcuts until the palette keymap is known", () => {
    const { container } = renderMenubar();

    expect(container.querySelector('[data-slot="os-menubar-command"]')).not.toHaveAttribute(
      "aria-keyshortcuts"
    );
  });
});

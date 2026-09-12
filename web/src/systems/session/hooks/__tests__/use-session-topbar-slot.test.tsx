// Suite: session document topbar publisher
// Invariant: one session publisher exposes self-title, state mark, runtime meta, lifecycle controls,
// sidebar toggle, and inspector toggle before overflow.
// Boundary IN: useSessionTopbarSlot and the real Topbar slot consumer.
// Boundary OUT: daemon mutations and window manager behavior.

import { fireEvent, screen, waitFor } from "@testing-library/react";
import { type ReactNode, useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";
import type { SessionPayload } from "../../types";

import { SessionRenameDialog } from "../../components/session-rename-dialog";
import { primarySessionFixture } from "../../mocks/fixtures";
import { useSessionTopbarSlot } from "../use-session-topbar-slot";
import { SessionTransportChip } from "../../components/session-transport-chip";
import { SESSION_TRANSPORT_LIVE } from "../../lib/session-transport";
import {
  SessionTransportContext,
  type SessionTransportState,
} from "../../lib/session-transcript-thread-context-value";

function SessionPublisher({
  onStop,
  onRename = vi.fn(),
  session = primarySessionFixture,
  inspectorOpen = false,
  onInspectorToggle = vi.fn(),
  sidebarOpen = false,
  onSidebarToggle = vi.fn(),
  transportChip,
}: {
  onStop: () => void;
  onRename?: () => void;
  session?: SessionPayload;
  inspectorOpen?: boolean;
  onInspectorToggle?: () => void;
  sidebarOpen?: boolean;
  onSidebarToggle?: () => void;
  transportChip?: ReactNode;
}) {
  useSessionTopbarSlot({
    session,
    transportChip,
    isDeleting: false,
    isRenaming: false,
    isStopping: false,
    isResuming: false,
    isUnarchiving: false,
    isClearing: false,
    canClear: true,
    inspectorOpen,
    sidebarOpen,
    onInspectorToggle,
    onSidebarToggle,
    onDelete: vi.fn(),
    onRename,
    onStop,
    onResume: vi.fn(),
    onUnarchive: vi.fn(),
    onClear: vi.fn(),
  });
  return null;
}

describe("useSessionTopbarSlot", () => {
  it("Should publish the complete document head through one slot owner", () => {
    const onStop = vi.fn();
    renderWithTopbar(<SessionPublisher onStop={onStop} />);

    expect(
      screen.getByRole("heading", { level: 1, name: primarySessionFixture.name })
    ).toBeInTheDocument();
    expect(document.querySelector('[data-slot="topbar-glyph"]')).toHaveAttribute(
      "data-presentation",
      "state"
    );
    expect(screen.getByTestId("session-status-agent")).toHaveTextContent(
      primarySessionFixture.agent_name
    );
    expect(screen.getByText("Session badge: running")).toHaveClass("sr-only");
    expect(document.querySelector('[data-slot="topbar-crumbs"]')).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    expect(onStop).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("session-inspector-toggle")).toHaveAttribute("aria-pressed", "false");
  });

  it.each(["system", "coordinator", "spawned"] as const)(
    "Should keep %s session lifecycle actions out of the topbar",
    sessionType => {
      renderWithTopbar(
        <SessionPublisher
          onStop={vi.fn()}
          session={{ ...primarySessionFixture, type: sessionType }}
        />
      );

      expect(screen.getByRole("button", { name: "Open sessions sidebar" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Open context sidebar" })).toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "Stop session" })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: "More actions" })).not.toBeInTheDocument();
    }
  );

  it("Should place the inspector toggle immediately before overflow and report pressed state", () => {
    const onInspectorToggle = vi.fn();
    renderWithTopbar(
      <SessionPublisher onStop={vi.fn()} inspectorOpen onInspectorToggle={onInspectorToggle} />
    );

    const toggle = screen.getByTestId("session-inspector-toggle");
    const overflow = screen.getByTestId("session-topbar-overflow");
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(toggle).toHaveAttribute("aria-label", "Close context sidebar");
    expect(
      toggle.compareDocumentPosition(overflow) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();

    fireEvent.click(toggle);
    expect(onInspectorToggle).toHaveBeenCalledTimes(1);
  });

  it.each([
    { sidebarOpen: false, pressed: "false", label: "Open sessions sidebar" },
    { sidebarOpen: true, pressed: "true", label: "Close sessions sidebar" },
  ])("Should expose and operate the sessions sidebar toggle", ({ sidebarOpen, pressed, label }) => {
    const onSidebarToggle = vi.fn();
    renderWithTopbar(
      <SessionPublisher
        onStop={vi.fn()}
        sidebarOpen={sidebarOpen}
        onSidebarToggle={onSidebarToggle}
      />
    );

    const toggle = screen.getByTestId("session-sidebar-toggle");
    expect(toggle).toHaveAttribute("aria-pressed", pressed);
    expect(toggle).toHaveAttribute("aria-label", label);
    fireEvent.click(toggle);
    expect(onSidebarToggle).toHaveBeenCalledTimes(1);
  });

  it("Should keep an idle attachable session to one immediate action plus overflow", () => {
    const onStop = vi.fn();

    function IdlePublisher() {
      useSessionTopbarSlot({
        session: { ...primarySessionFixture, badge: "idle" },
        isDeleting: false,
        isRenaming: false,
        isStopping: false,
        isResuming: false,
        isUnarchiving: false,
        isClearing: false,
        canClear: true,
        inspectorOpen: false,
        sidebarOpen: false,
        onSidebarToggle: vi.fn(),
        onInspectorToggle: vi.fn(),
        onDelete: vi.fn(),
        onRename: vi.fn(),
        onStop,
        onResume: vi.fn(),
        onUnarchive: vi.fn(),
        onClear: vi.fn(),
      });
      return null;
    }

    renderWithTopbar(<IdlePublisher />);

    expect(screen.getByRole("button", { name: "Attach session" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Stop session" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open context sidebar" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Stop session" }));
    expect(onStop).toHaveBeenCalledTimes(1);
  });

  it("Should offer unarchive without attach or resume controls for an archived session", () => {
    const onUnarchive = vi.fn();

    function ArchivedPublisher() {
      useSessionTopbarSlot({
        session: {
          ...primarySessionFixture,
          state: "stopped",
          badge: "stopped",
          attachable: true,
          archived_at: "2026-08-04T12:00:00Z",
        },
        isDeleting: false,
        isRenaming: false,
        isStopping: false,
        isResuming: false,
        isUnarchiving: false,
        isClearing: false,
        canClear: true,
        inspectorOpen: false,
        sidebarOpen: false,
        onSidebarToggle: vi.fn(),
        onInspectorToggle: vi.fn(),
        onDelete: vi.fn(),
        onRename: vi.fn(),
        onStop: vi.fn(),
        onResume: vi.fn(),
        onUnarchive,
        onClear: vi.fn(),
      });
      return null;
    }

    renderWithTopbar(<ArchivedPublisher />);

    fireEvent.click(screen.getByRole("button", { name: "Unarchive session" }));
    expect(onUnarchive).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: "Attach session" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Resume session" })).toBeNull();
  });

  it("Should close overflow before opening rename so one Escape dismisses the dialog", async () => {
    function RenameDialogPublisher() {
      const [dialogOpen, setDialogOpen] = useState(false);
      return (
        <>
          <SessionPublisher onStop={vi.fn()} onRename={() => setDialogOpen(true)} />
          <SessionRenameDialog
            open={dialogOpen}
            onOpenChange={setDialogOpen}
            session={primarySessionFixture}
            isRenaming={false}
            onConfirm={vi.fn()}
          />
        </>
      );
    }

    renderWithTopbar(<RenameDialogPublisher />);

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Rename session" }));

    await screen.findByRole("heading", { name: "Rename session" });
    fireEvent.keyDown(document, { key: "Escape" });

    await waitFor(() => {
      expect(screen.queryByRole("heading", { name: "Rename session" })).toBeNull();
    });
  });

  // Invariant (task_06 VC-01..05, integration): the transport chip the window
  // publishes into the OS head reads the window's own transport — the slot
  // consumer renders outside the session runtime provider, so the publisher
  // must carry the snapshot with the node. Owning layer: the topbar slot
  // publisher; canonical suite: this file (real Topbar slot consumer).
  it("Should publish the transport chip with the window's own transport into the head", () => {
    // The chip's grace has elapsed: the stream was last live five minutes ago.
    const lostAt = Date.now() - 5 * 60_000;
    const transport = (overrides: Partial<SessionTransportState>): SessionTransportState => ({
      ...SESSION_TRANSPORT_LIVE,
      lastLiveAt: lostAt,
      retry: vi.fn(),
      ...overrides,
    });
    const publisher = (state: SessionTransportState, windowLive = true) => (
      <SessionTransportContext.Provider value={state}>
        <SessionPublisher
          onStop={vi.fn()}
          transportChip={<SessionTransportChip windowLive={windowLive} />}
        />
      </SessionTransportContext.Provider>
    );
    const view = renderWithTopbar(
      publisher(transport({ degradedAt: lostAt, phase: "waiting-reconnect", reconnectAttempt: 3 }))
    );
    const chip = screen.getByTestId("session-transport-chip");
    expect(chip).toHaveTextContent(/Reconnecting/);
    expect(screen.getByTestId("session-transport-chip-count")).toHaveTextContent("3");

    view.rerender(
      publisher(
        transport({
          degradedAt: lostAt,
          failure: { at: lostAt + 30_000, attempts: 6 },
          phase: "failed",
        })
      )
    );
    expect(screen.getByTestId("session-transport-chip")).toHaveTextContent(/Disconnected/);

    // A background window is paused: its own last-live instant, never another window's.
    view.rerender(publisher(transport({ phase: "disabled" }), false));
    expect(screen.getByTestId("session-transport-chip")).toHaveTextContent(/Paused/);

    view.rerender(publisher(transport({})));
    expect(screen.queryByTestId("session-transport-chip")).not.toBeInTheDocument();
  });
});

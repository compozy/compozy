// Suite: session document topbar publisher
// Invariant: one session publisher exposes self-title, state mark, overflow and the List | Kanban
// toggle (present even without the overflow), then the PR chip first in the status zone (no agent,
// provider or time there), then lifecycle controls and the sidebar and inspector toggles; the chip
// menu offers only the actions the session allows.
// Boundary IN: useSessionTopbarSlot and the real Topbar slot consumer.
// Boundary OUT: daemon mutations and window manager behavior.

import { fireEvent, screen, waitFor, within } from "@testing-library/react";
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
  onContinue,
  onFork,
  onSendPrompt,
  onRefresh,
}: {
  onSendPrompt?: (text: string) => void;
  onRefresh?: () => void;
  onContinue?: () => void;
  onFork?: () => void;
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
    onUnarchive: vi.fn(),
    onClear: vi.fn(),
    onContinue,
    onFork,
    onSendPrompt,
    pullRequestRefresh: onRefresh ? { refreshing: false, refresh: onRefresh } : undefined,
  });
  return null;
}

const FAILING_PR: NonNullable<SessionPayload["pull_request"]> = {
  provider: "github",
  number: 932,
  url: "https://github.com/compozy/compozy/pull/932",
  state: "open",
  draft: true,
  head_sha: "8f3c2a1d40",
  checks: {
    state: "failing",
    passed: 12,
    failed: 2,
    pending: 0,
    failed_names: ["lint", "test-web"],
  },
  review_decision: "review_required",
  mergeable: "mergeable",
  ready_to_merge: false,
};

const withPullRequest = (patch: Partial<SessionPayload> = {}): SessionPayload => ({
  ...primarySessionFixture,
  branch: "feat/forge-gql",
  pull_request: FAILING_PR,
  pull_request_lookup: {
    state: "ok",
    cause: "",
    observed_at: "2026-10-23T18:01:40Z",
    stale: false,
  },
  ...patch,
});

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
    expect(screen.queryByTestId("session-status-agent")).not.toBeInTheDocument();
    expect(screen.getByText("Session status: Working")).toHaveClass("sr-only");
    expect(document.querySelector('[data-slot="topbar-crumbs"]')).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    expect(onStop).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("session-inspector-toggle")).toHaveAttribute("aria-pressed", "false");
  });

  it("Should lead the status zone with the PR chip and drop agent, provider and time", () => {
    renderWithTopbar(<SessionPublisher onStop={vi.fn()} session={withPullRequest()} />);

    const chip = screen.getByTestId("session-pull-request-chip");
    expect(chip).toHaveAccessibleName("Pull request #932, Draft");
    expect(chip).toHaveTextContent("#932");
    expect(chip).toHaveTextContent("2 failing");
    for (const id of [
      "session-status-agent-chip",
      "session-status-provider",
      "session-status-time",
    ]) {
      expect(screen.queryByTestId(id)).not.toBeInTheDocument();
    }

    const status = document.querySelector('[data-slot="topbar-status"]')!;
    const actions = document.querySelector('[data-slot="topbar-actions"]')!;
    expect(status).toContainElement(chip);
    expect(status.querySelector("button, a")).toBe(chip);
    const order = [
      screen.getByRole("button", { name: "Stop session" }),
      screen.getByTestId("session-sidebar-toggle"),
      screen.getByTestId("session-inspector-toggle"),
    ];
    for (const control of order) expect(actions).toContainElement(control);
    for (let index = 1; index < order.length; index += 1) {
      expect(
        order[index - 1]!.compareDocumentPosition(order[index]!) & Node.DOCUMENT_POSITION_FOLLOWING
      ).toBeTruthy();
    }
  });

  it("Should render no chip for a session without a branch", () => {
    renderWithTopbar(
      <SessionPublisher
        onStop={vi.fn()}
        session={{ ...primarySessionFixture, pull_request: null, pull_request_lookup: null }}
      />
    );

    expect(screen.queryByTestId("session-pull-request-chip")).not.toBeInTheDocument();
  });

  it("Should ask the agent to fix failing checks from the chip menu's primary item", async () => {
    const onSendPrompt = vi.fn();
    const onRefresh = vi.fn();
    renderWithTopbar(
      <SessionPublisher
        onStop={vi.fn()}
        onRefresh={onRefresh}
        onSendPrompt={onSendPrompt}
        session={withPullRequest()}
      />
    );

    fireEvent.click(screen.getByTestId("session-pull-request-chip"));
    const menu = await screen.findByTestId("session-pull-request-menu");
    const items = within(menu).getAllByRole("menuitem");
    expect(items.map(item => item.textContent)).toEqual([
      "Fix failing checks",
      "Open PR",
      "Refresh",
    ]);

    fireEvent.click(items[0]!);
    expect(onSendPrompt).toHaveBeenCalledTimes(1);
    expect(onSendPrompt.mock.calls[0]?.[0]).toEqual(expect.stringContaining("#932"));
    expect(onSendPrompt.mock.calls[0]?.[0]).toEqual(expect.stringContaining("lint"));
  });

  it("Should offer a non-controllable session only Open PR and Refresh (UT-062)", async () => {
    const onRefresh = vi.fn();
    renderWithTopbar(
      <SessionPublisher
        onStop={vi.fn()}
        onRefresh={onRefresh}
        onSendPrompt={vi.fn()}
        session={withPullRequest({ type: "spawned" })}
      />
    );

    fireEvent.click(screen.getByTestId("session-pull-request-chip"));
    const menu = await screen.findByTestId("session-pull-request-menu");
    const items = within(menu).getAllByRole("menuitem");
    expect(items.map(item => item.textContent)).toEqual(["Open PR", "Refresh"]);
    fireEvent.click(items[1]!);
    expect(onRefresh).toHaveBeenCalledTimes(1);
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

  it("Should keep overflow beside the title, ahead of the inspector toggle, and report pressed state", () => {
    const onInspectorToggle = vi.fn();
    renderWithTopbar(
      <SessionPublisher onStop={vi.fn()} inspectorOpen onInspectorToggle={onInspectorToggle} />
    );

    const toggle = screen.getByTestId("session-inspector-toggle");
    const overflow = screen.getByTestId("session-topbar-overflow");
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(toggle).toHaveAttribute("aria-label", "Close context sidebar");
    // Shell-rail head: title + ⋯ lead, the panel toggles trail (prototype `.head`).
    expect(
      overflow.compareDocumentPosition(toggle) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();

    fireEvent.click(toggle);
    expect(onInspectorToggle).toHaveBeenCalledTimes(1);
  });

  it("Should follow the overflow with the List | Kanban toggle, kept for sessions without one", () => {
    const { unmount } = renderWithTopbar(<SessionPublisher onStop={vi.fn()} />);
    const overflow = screen.getByTestId("session-topbar-overflow");
    const viewToggle = screen.getByRole("group", { name: "View" });
    expect(
      overflow.compareDocumentPosition(viewToggle) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: "List" })).toHaveAttribute("aria-pressed", "true");
    unmount();

    renderWithTopbar(
      <SessionPublisher onStop={vi.fn()} session={{ ...primarySessionFixture, type: "spawned" }} />
    );
    expect(screen.queryByTestId("session-topbar-overflow")).toBeNull();
    expect(screen.getByRole("group", { name: "View" })).toBeInTheDocument();
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

  it("Should keep an idle session to Stop as its one immediate action plus overflow", () => {
    const onStop = vi.fn();

    function IdlePublisher() {
      useSessionTopbarSlot({
        session: { ...primarySessionFixture, badge: "idle" },
        isDeleting: false,
        isRenaming: false,
        isStopping: false,
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
        onUnarchive: vi.fn(),
        onClear: vi.fn(),
      });
      return null;
    }

    renderWithTopbar(<IdlePublisher />);

    expect(screen.getByRole("button", { name: "Open context sidebar" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    expect(onStop).toHaveBeenCalledTimes(1);
    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    expect(screen.queryByRole("menuitem", { name: "Stop session" })).not.toBeInTheDocument();
  });

  // Invariant: an idle session whose agent process the daemon released
  // (`active` + `runtime.status: "unbound"`) reads exactly like any idle
  // session — the Idle state and the same lifecycle controls, never a stopped
  // session's presentation; the next prompt rebinds it.
  it.each(["ready", "unbound"] as const)(
    "Should present an idle session with a %s runtime with the same idle controls",
    status => {
      renderWithTopbar(
        <SessionPublisher
          onStop={vi.fn()}
          session={{
            ...primarySessionFixture,
            badge: "idle",
            state: "active",
            attachable: true,
            runtime: { ...primarySessionFixture.runtime, status },
          }}
        />
      );

      expect(screen.getByText("Session status: Idle")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Stop session" })).toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "More actions" }));
      expect(screen.queryByRole("menuitem", { name: "Stop session" })).not.toBeInTheDocument();
    }
  );

  it("Should offer unarchive as the only primary action for an archived session", () => {
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
        onUnarchive,
        onClear: vi.fn(),
      });
      return null;
    }

    renderWithTopbar(<ArchivedPublisher />);

    fireEvent.click(screen.getByRole("button", { name: "Unarchive session" }));
    expect(onUnarchive).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: "Stop session" })).toBeNull();
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

  it("Should offer Continue after Rename and open it once the overflow has closed", async () => {
    const onContinue = vi.fn();
    renderWithTopbar(<SessionPublisher onStop={vi.fn()} onContinue={onContinue} />);

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const items = screen.getAllByRole("menuitem").map(item => item.textContent);
    expect(items.indexOf("Continue with another agent…")).toBe(items.indexOf("Rename session") + 1);
    fireEvent.click(screen.getByTestId("continue-menu-item"));

    await waitFor(() => expect(onContinue).toHaveBeenCalledOnce());
    expect(screen.queryByTestId("session-topbar-overflow-menu")).toBeNull();
  });

  it("Should not offer Continue or Fork for an archived session", () => {
    renderWithTopbar(
      <SessionPublisher
        onStop={vi.fn()}
        onContinue={vi.fn()}
        onFork={vi.fn()}
        session={{ ...primarySessionFixture, archived_at: "2026-09-01T00:00:00Z" }}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    expect(screen.queryByTestId("continue-menu-item")).toBeNull();
    expect(screen.queryByTestId("fork-menu-item")).toBeNull();
  });

  // UT-074: Fork session… follows Continue (S2) and opens the whole-session
  // Fork dialog once the overflow has closed, so focus lands in the dialog.
  it("Should offer Fork after Continue and open it once the overflow has closed", async () => {
    const onFork = vi.fn();
    renderWithTopbar(<SessionPublisher onStop={vi.fn()} onContinue={vi.fn()} onFork={onFork} />);

    fireEvent.click(screen.getByRole("button", { name: "More actions" }));
    const items = screen.getAllByRole("menuitem").map(item => item.textContent);
    expect(items.indexOf("Fork session…")).toBe(items.indexOf("Continue with another agent…") + 1);
    fireEvent.click(screen.getByTestId("fork-menu-item"));

    await waitFor(() => expect(onFork).toHaveBeenCalledOnce());
    expect(screen.queryByTestId("session-topbar-overflow-menu")).toBeNull();
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

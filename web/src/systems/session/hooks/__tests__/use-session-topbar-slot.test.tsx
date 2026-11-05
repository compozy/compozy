// Suite: session document topbar publisher
// Invariant: one session publisher exposes self-title, state mark and the List | Kanban toggle
// (no per-session `…` menu: those verbs live in the session list row menus), then the PR chip
// first in the status zone (no agent, provider or time there), then lifecycle controls and the
// sidebar and inspector toggles; the chip menu offers only the actions the session allows.
// Shown in the board drawer (#1052) the head drops the rail toggle and the view toggle: the
// board is the list there, and the drawer carries its own controls. An always-on agent's main
// session (#1303) names the agent, keeps its `…` menu, and drops the linked chip, Stop and the
// context sidebar toggle; every other session keeps them.
// Boundary IN: useSessionTopbarSlot and the real Topbar slot consumer.
// Boundary OUT: daemon mutations and window manager behavior.

import { act, fireEvent, screen, waitFor, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { renderWithTopbar } from "@/test/render-with-topbar";
import type { AgentInstancePayload } from "@/systems/agent";
import { agentInstanceFixture } from "@/systems/agent/mocks/instance-fixtures";
import type { SessionPayload } from "../../types";

import { primarySessionFixture } from "../../mocks/fixtures";
import { SESSION_LINKS_DOZEN, SESSION_LINK_SETS } from "../../mocks/session-links-fixtures";
import { leadLink, linkCount, type SessionLinkedWorkItem } from "../../lib/session-linked-work";
import { withSessionProcessSummary } from "../../mocks/process-fixtures";
import { SessionWindowPresentationContext } from "../../contexts/session-window-presentation-context";
import { closeSessionLinkDialog, sessionLinkDialogStore } from "../use-session-link-dialog";
import type { SessionLinkedWorkModel } from "../use-session-linked-work";
import { useSessionTopbarSlot } from "../use-session-topbar-slot";
import { SessionTransportChip } from "../../components/session-transport-chip";
import { SESSION_TRANSPORT_LIVE } from "../../lib/session-transport";
import {
  SessionTransportContext,
  type SessionTransportState,
} from "../../lib/session-transcript-thread-context-value";

const agentInstances = vi.hoisted(() => ({ rows: [] as AgentInstancePayload[] }));

vi.mock("@/systems/agent", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/agent")>()),
  useAgentInstances: () => agentInstances.rows,
}));

// The always-on `…` menu navigates to the agent's pages; no route renders here.
vi.mock("@tanstack/react-router", async importOriginal => ({
  ...(await importOriginal<typeof import("@tanstack/react-router")>()),
  useNavigate: () => vi.fn(),
}));

const NO_LINKED_WORK: SessionLinkedWorkModel = {
  items: [],
  lead: null,
  count: linkCount([]),
  refreshing: false,
  clipped: false,
  setClipped: () => undefined,
  refresh: () => undefined,
  unlink: () => undefined,
};

const LINKED_WORK = { linkedWork: NO_LINKED_WORK } as const;

function SessionPublisher({
  onStop,
  session = primarySessionFixture,
  inspectorOpen = false,
  onInspectorToggle = vi.fn(),
  sidebarOpen = false,
  onSidebarToggle = vi.fn(),
  transportChip,
  onSendPrompt,
  onRefresh,
  hasLiveGoal = false,
  linkedWork = NO_LINKED_WORK,
  onOpenSession,
}: {
  linkedWork?: SessionLinkedWorkModel;
  onOpenSession?: (sessionId: string) => void;
  onSendPrompt?: (text: string) => void;
  onRefresh?: () => void;
  hasLiveGoal?: boolean;
  onStop: (processes?: string) => void;
  session?: SessionPayload;
  inspectorOpen?: boolean;
  onInspectorToggle?: () => void;
  sidebarOpen?: boolean;
  onSidebarToggle?: () => void;
  transportChip?: ReactNode;
}) {
  useSessionTopbarSlot({
    ...LINKED_WORK,
    linkedWork,
    onOpenSession,
    session,
    transportChip,
    hasLiveGoal,
    isStopping: false,
    isUnarchiving: false,
    inspectorOpen,
    sidebarOpen,
    onInspectorToggle,
    onSidebarToggle,
    onStop,
    onUnarchive: vi.fn(),
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
  // Invariant: stopping a Goal session explains its pause and requires confirmation before mutation.
  // Owner: session lifecycle header; canonical suite: use-session-topbar-slot.
  it("Should confirm session stop and explain that its Goal needs an explicit resume", () => {
    const onStop = vi.fn();
    renderWithTopbar(<SessionPublisher onStop={onStop} hasLiveGoal />);
    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    const dialog = screen.getByRole("dialog", { name: "Stop this session?" });
    expect(dialog).toHaveTextContent("goal will be paused");
    expect(dialog).toHaveTextContent("Sending a message reopens the session");
    expect(dialog).toHaveTextContent("The goal stays paused until you choose Resume goal");
    expect(onStop).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Stop session" }));
    expect(onStop).toHaveBeenCalledTimes(1);
  });

  // Invariant: an external lifecycle change invalidates a pending stop confirmation.
  // Owner: session lifecycle header; canonical suite: use-session-topbar-slot.
  it("Should require a new confirmation after a stopped session becomes active again", () => {
    const onStop = vi.fn();
    const { rerender } = renderWithTopbar(<SessionPublisher onStop={onStop} hasLiveGoal />);
    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    expect(screen.getByRole("dialog", { name: "Stop this session?" })).toBeInTheDocument();

    rerender(
      <SessionPublisher
        onStop={onStop}
        hasLiveGoal
        session={{ ...primarySessionFixture, state: "stopped" }}
      />
    );
    expect(screen.queryByRole("dialog", { name: "Stop this session?" })).not.toBeInTheDocument();
    rerender(<SessionPublisher onStop={onStop} hasLiveGoal />);
    expect(screen.queryByRole("dialog", { name: "Stop this session?" })).not.toBeInTheDocument();
    expect(onStop).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    const dialog = screen.getByRole("dialog", { name: "Stop this session?" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Stop session" }));
    expect(onStop).toHaveBeenCalledTimes(1);
  });

  // Invariant: an unread Goal never bypasses session-stop confirmation.
  // Owner: session lifecycle header; canonical suite: use-session-topbar-slot.
  it("Should explain conditional Goal pause while its presence is unknown", () => {
    const onStop = vi.fn();
    const UnknownGoalPublisher = () => {
      useSessionTopbarSlot({
        ...LINKED_WORK,
        session: primarySessionFixture,
        isStopping: false,
        isUnarchiving: false,
        inspectorOpen: false,
        sidebarOpen: false,
        onInspectorToggle: vi.fn(),
        onSidebarToggle: vi.fn(),
        onStop,
        onUnarchive: vi.fn(),
        hasLiveGoal: undefined,
      });
      return null;
    };
    renderWithTopbar(<UnknownGoalPublisher />);
    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    const dialog = screen.getByRole("dialog", { name: "Stop this session?" });
    expect(dialog).toHaveTextContent("Any active goal will be paused");
    expect(dialog).toHaveTextContent("Sending a message reopens the session.");
    expect(dialog).not.toHaveTextContent("Resume goal");
    expect(onStop).not.toHaveBeenCalled();
    fireEvent.click(within(dialog).getByRole("button", { name: "Keep working" }));
    expect(onStop).not.toHaveBeenCalled();
  });

  // Invariant: a stop asks what happens to the processes the session left running, even
  // without a Goal, and answers keep or terminate; at zero it stays a one-click stop.
  // Owner: session lifecycle header; canonical suite: use-session-topbar-slot.
  it("Should ask whether a stop also ends the processes the session left running", () => {
    const onStop = vi.fn();
    const session = withSessionProcessSummary(primarySessionFixture);
    const { rerender } = renderWithTopbar(<SessionPublisher onStop={onStop} session={session} />);
    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    const dialog = screen.getByRole("dialog", { name: "Stop this session?" });
    expect(dialog).toHaveTextContent("This session started 3 processes that are still running.");
    expect(dialog).toHaveTextContent(
      "Listening on localhost:5173, localhost:8080 and localhost:9090."
    );
    fireEvent.click(within(dialog).getByRole("button", { name: "Keep processes running" }));
    expect(onStop).toHaveBeenLastCalledWith("keep");

    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    fireEvent.click(
      within(screen.getByRole("dialog", { name: "Stop this session?" })).getByRole("button", {
        name: "Stop session and 3 processes",
      })
    );
    expect(onStop).toHaveBeenLastCalledWith("terminate");

    rerender(<SessionPublisher onStop={onStop} session={primarySessionFixture} />);
    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    expect(screen.queryByRole("dialog", { name: "Stop this session?" })).not.toBeInTheDocument();
    expect(onStop).toHaveBeenCalledTimes(3);
    expect(onStop).toHaveBeenLastCalledWith();
  });

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
    expect(items.map(item => item.getAttribute("data-action"))).toEqual(["fix", "open", "refresh"]);
    expect(items[0]).toHaveTextContent("Fix failing checks");

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
    expect(items.map(item => item.getAttribute("data-action"))).toEqual(["open", "refresh"]);
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

  it("Should keep the view toggle beside the title, ahead of the inspector toggle, and report pressed state", () => {
    const onInspectorToggle = vi.fn();
    renderWithTopbar(
      <SessionPublisher onStop={vi.fn()} inspectorOpen onInspectorToggle={onInspectorToggle} />
    );

    const toggle = screen.getByTestId("session-inspector-toggle");
    const viewToggle = screen.getByRole("group", { name: "View" });
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(toggle).toHaveAttribute("aria-label", "Close context sidebar");
    // Shell-rail head: title + view toggle lead, the panel toggles trail (prototype `.head`).
    expect(
      viewToggle.compareDocumentPosition(toggle) & Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();

    fireEvent.click(toggle);
    expect(onInspectorToggle).toHaveBeenCalledTimes(1);
  });

  // #952: the per-session `…` menu left the head; its verbs live in the session
  // list row menus, so no session type gets one and the view toggle always shows.
  it.each(["user", "spawned"] as const)(
    "Should publish the List | Kanban toggle without a session overflow menu for a %s session",
    type => {
      renderWithTopbar(
        <SessionPublisher onStop={vi.fn()} session={{ ...primarySessionFixture, type }} />
      );
      expect(screen.getByRole("group", { name: "View" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "List" })).toHaveAttribute("aria-pressed", "true");
      expect(screen.queryByRole("button", { name: "More actions" })).toBeNull();
    }
  );

  it("Should drop the rail and view toggles when the session shows in the board drawer", () => {
    renderWithTopbar(
      <SessionWindowPresentationContext value="board-drawer">
        <SessionPublisher onStop={vi.fn()} />
      </SessionWindowPresentationContext>
    );
    expect(screen.queryByTestId("session-sidebar-toggle")).toBeNull();
    expect(screen.queryByRole("group", { name: "View" })).toBeNull();
    expect(screen.getByTestId("session-inspector-toggle")).toBeInTheDocument();
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

  it("Should keep an idle session to Stop as its one immediate action", () => {
    const onStop = vi.fn();

    function IdlePublisher() {
      useSessionTopbarSlot({
        ...LINKED_WORK,
        hasLiveGoal: false,
        session: { ...primarySessionFixture, badge: "idle" },
        isStopping: false,
        isUnarchiving: false,
        inspectorOpen: false,
        sidebarOpen: false,
        onSidebarToggle: vi.fn(),
        onInspectorToggle: vi.fn(),
        onStop,
        onUnarchive: vi.fn(),
      });
      return null;
    }

    renderWithTopbar(<IdlePublisher />);

    expect(screen.getByRole("button", { name: "Open context sidebar" })).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Stop session" }));
    expect(onStop).toHaveBeenCalledTimes(1);
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
    }
  );

  it("Should offer unarchive as the only primary action for an archived session", () => {
    const onUnarchive = vi.fn();

    function ArchivedPublisher() {
      useSessionTopbarSlot({
        ...LINKED_WORK,
        session: {
          ...primarySessionFixture,
          state: "stopped",
          badge: "stopped",
          attachable: true,
          archived_at: "2026-08-04T12:00:00Z",
        },
        isStopping: false,
        isUnarchiving: false,
        inspectorOpen: false,
        sidebarOpen: false,
        onSidebarToggle: vi.fn(),
        onInspectorToggle: vi.fn(),
        onStop: vi.fn(),
        onUnarchive,
      });
      return null;
    }

    renderWithTopbar(<ArchivedPublisher />);

    fireEvent.click(screen.getByRole("button", { name: "Unarchive session" }));
    expect(onUnarchive).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole("button", { name: "Stop session" })).toBeNull();
  });

  // Invariant (#1202): the head offers Mark as seen only for a Needs you the daemon can
  // clear; a session waiting on an answer keeps its badge without the control.
  it("Should offer Mark as seen in the head only for a dismissible Needs you", () => {
    const { rerender } = renderWithTopbar(
      <SessionPublisher
        onStop={vi.fn()}
        session={{
          ...primarySessionFixture,
          badge: "needs-attention",
          attention: "ended_early",
          attention_dismissible: true,
        }}
      />
    );
    expect(screen.getByRole("button", { name: "Mark as seen" })).toBeEnabled();

    rerender(
      <SessionPublisher
        onStop={vi.fn()}
        session={{
          ...primarySessionFixture,
          badge: "waiting-for-input",
          attention_dismissible: false,
        }}
      />
    );
    expect(screen.queryByRole("button", { name: "Mark as seen" })).toBeNull();
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

  // Invariant: the header shows a lone item's chip, or one "N linked PRs" label for several, and
  // keeps the whole list one step away at every width; only the session's own direct links can be
  // unlinked from it.
  // Owner: session document head; canonical suite: use-session-topbar-slot.
  describe("always-on main session (#1303)", () => {
    afterEach(() => {
      agentInstances.rows = [];
    });

    const linkedWork: SessionLinkedWorkModel = {
      ...NO_LINKED_WORK,
      items: SESSION_LINK_SETS.addedBy,
      lead: leadLink(SESSION_LINK_SETS.addedBy),
      count: linkCount(SESSION_LINK_SETS.addedBy),
      clipped: true,
    };
    const renderHead = () =>
      renderWithTopbar(<SessionPublisher linkedWork={linkedWork} onStop={vi.fn()} />);

    it("Should keep the linked chip, Stop and both panel toggles on an ordinary session", () => {
      agentInstances.rows = [agentInstanceFixture({ main_session_id: "sess-other" })];
      renderHead();
      expect(screen.getByTestId("session-links-summary")).toHaveTextContent(/^4 linked$/);
      expect(screen.getByRole("button", { name: "Stop session" })).toBeInTheDocument();
      expect(screen.getByTestId("session-sidebar-toggle")).toBeInTheDocument();
      expect(screen.getByTestId("session-inspector-toggle")).toBeInTheDocument();
      expect(screen.queryByText("Always-on")).toBeNull();
    });

    it("Should drop the linked chip, Stop and the context toggle on the agent's main", () => {
      const instance = agentInstanceFixture({ main_session_id: primarySessionFixture.id });
      agentInstances.rows = [instance];
      renderHead();
      expect(screen.getByRole("heading", { level: 1, name: instance.label })).toBeInTheDocument();
      expect(screen.getByText("Always-on")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: `${instance.label} actions` })).toBeInTheDocument();
      expect(screen.queryByTestId("session-links-summary")).toBeNull();
      expect(screen.queryByText(/linked/)).toBeNull();
      expect(screen.queryByRole("button", { name: "Stop session" })).toBeNull();
      expect(screen.queryByTestId("session-inspector-toggle")).toBeNull();
      expect(screen.getByTestId("session-sidebar-toggle")).toBeInTheDocument();
    });
  });

  describe("linked work", () => {
    afterEach(() => vi.unstubAllGlobals());

    const model = (
      items: readonly SessionLinkedWorkItem[],
      overrides: Partial<SessionLinkedWorkModel> = {}
    ): SessionLinkedWorkModel => ({
      ...NO_LINKED_WORK,
      items,
      lead: leadLink(items),
      count: linkCount(items),
      ...overrides,
    });

    const renderLinks = (linkedWork: SessionLinkedWorkModel, session?: SessionPayload) =>
      renderWithTopbar(
        <SessionPublisher linkedWork={linkedWork} onStop={vi.fn()} session={session} />
      );

    it.each([
      ["one", SESSION_LINK_SETS.onePr, null],
      ["two", SESSION_LINK_SETS.mixed.slice(0, 2), "2 linked PRs"],
      ["eight", SESSION_LINKS_DOZEN.filter(item => item.pull_request).slice(0, 8), "8 linked PRs"],
    ])("Should label %s linked pull requests in the head", (_name, items, label) => {
      renderLinks(model(items));
      if (label === null) {
        expect(screen.getByTestId("session-pull-request-chip")).toHaveTextContent(
          `#${items[0]!.pull_request!.number}`
        );
        expect(screen.queryByTestId("session-links-summary")).not.toBeInTheDocument();
        return;
      }
      expect(screen.getByTestId("session-links-summary")).toHaveTextContent(label);
      expect(screen.queryByTestId("session-pull-request-chip")).not.toBeInTheDocument();
    });

    it("Should open every linked item, each with its status, from the count", async () => {
      renderLinks(model(SESSION_LINK_SETS.mixed));
      fireEvent.click(screen.getByTestId("session-links-summary"));
      const list = await screen.findByRole("dialog");
      expect(list).toHaveTextContent("Linked work");
      expect(list).toHaveTextContent(
        `This session runs in ${primarySessionFixture.workspace_path}. Linking never moves it.`
      );
      const rows = [...list.querySelectorAll('[data-slot="session-link-row"]')];
      expect(rows.map(row => row.textContent)).toEqual([
        expect.stringContaining("Checks failing"),
        expect.stringContaining("5 checks pending"),
        expect.stringContaining("Merged"),
        expect.stringContaining("Closed"),
      ]);
    });

    it("Should name what needs you on the count, and nothing when nothing does", () => {
      const view = renderLinks(model(SESSION_LINK_SETS.mixed));
      const attention = screen.getByRole("button", { name: "4 linked PRs, 1 needs you" });
      expect(attention.querySelector('[data-slot="pill-dot"]')).not.toBeNull();
      view.rerender(
        <SessionPublisher
          linkedWork={model(SESSION_LINK_SETS.mixed.slice(1))}
          onStop={vi.fn()}
          session={undefined}
        />
      );
      const calm = screen.getByRole("button", { name: "3 linked PRs" });
      expect(calm.querySelector('[data-slot="pill-dot"]')).toBeNull();
    });

    it("Should close the list when the link dialog takes over", async () => {
      renderLinks(model(SESSION_LINK_SETS.mixed));
      fireEvent.click(screen.getByTestId("session-links-summary"));
      fireEvent.click(await screen.findByRole("button", { name: "Link…" }));
      await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
      expect(sessionLinkDialogStore.getSnapshot().context.sessionId).toBe(primarySessionFixture.id);
      closeSessionLinkDialog();
    });

    it("Should keep the open list when an unlink leaves one item", async () => {
      const view = renderLinks(model(SESSION_LINK_SETS.mixed));
      fireEvent.click(screen.getByTestId("session-links-summary"));
      await screen.findByRole("dialog");
      view.rerender(
        <SessionPublisher
          linkedWork={model(SESSION_LINK_SETS.onePr)}
          onStop={vi.fn()}
          session={undefined}
        />
      );
      const list = await screen.findByRole("dialog");
      expect(list.querySelectorAll('[data-slot="session-link-row"]')).toHaveLength(1);
    });

    it("Should say N linked when not every item is a pull request", () => {
      renderLinks(model(SESSION_LINK_SETS.addedBy));
      expect(screen.getByTestId("session-links-summary")).toHaveTextContent(/^4 linked$/);
    });

    it("Should put the link actions in the chip menu of a single item", async () => {
      const unlink = vi.fn();
      renderLinks(model(SESSION_LINK_SETS.onePr, { unlink }));
      expect(screen.queryByTestId("session-links-summary")).not.toBeInTheDocument();
      fireEvent.click(screen.getByTestId("session-pull-request-chip"));
      const menu = await screen.findByTestId("session-pull-request-menu");
      expect(within(menu).getByText("Link another…")).toBeInTheDocument();
      fireEvent.click(within(menu).getByText("Unlink from this session"));
      expect(unlink).toHaveBeenCalledWith(SESSION_LINK_SETS.onePr[0]);
    });

    it("Should keep the session's own chip and actions when nothing is linked", async () => {
      renderLinks(NO_LINKED_WORK, withPullRequest());
      fireEvent.click(screen.getByTestId("session-pull-request-chip"));
      const menu = await screen.findByTestId("session-pull-request-menu");
      expect(within(menu).getByText("Fix failing checks")).toBeInTheDocument();
      expect(within(menu).queryByText("Unlink from this session")).not.toBeInTheDocument();
    });

    it("Should not offer the session's delivery actions on another branch's item", async () => {
      const onRefresh = vi.fn();
      const refresh = vi.fn();
      renderWithTopbar(
        <SessionPublisher
          linkedWork={model(SESSION_LINK_SETS.onePr, { refresh })}
          onRefresh={onRefresh}
          onStop={vi.fn()}
          session={{ ...withPullRequest(), branch: "feat/somewhere-else" }}
        />
      );
      fireEvent.click(screen.getByTestId("session-pull-request-chip"));
      const menu = await screen.findByTestId("session-pull-request-menu");
      fireEvent.click(within(menu).getByText("Refresh"));
      expect(refresh).toHaveBeenCalledTimes(1);
      expect(onRefresh).not.toHaveBeenCalled();
    });

    const needsWork = (item: SessionLinkedWorkItem): SessionLinkedWorkItem => ({
      ...item,
      pull_request: {
        ...item.pull_request!,
        ...FAILING_PR,
        number: item.pull_request!.number,
        draft: false,
        review_decision: "changes_requested",
      },
    });
    const AGENT_PROMPTS = ["Fix failing checks", "Address review comments"];

    it.each([
      ["another branch", needsWork(SESSION_LINK_SETS.onePr[0]!), "feat/somewhere-else"],
      [
        "a child session, even on this branch",
        needsWork(SESSION_LINK_SETS.rolledUp[0]!),
        SESSION_LINK_SETS.rolledUp[0]!.branch,
      ],
    ])("Should offer no agent prompts on the chip of %s", async (_name, item, branch) => {
      const onSendPrompt = vi.fn();
      renderWithTopbar(
        <SessionPublisher
          linkedWork={model([item])}
          onSendPrompt={onSendPrompt}
          onStop={vi.fn()}
          session={{ ...withPullRequest(), branch }}
        />
      );
      fireEvent.click(screen.getByTestId("session-pull-request-chip"));
      const menu = await screen.findByTestId("session-pull-request-menu");
      expect(within(menu).getByText("Open PR")).toBeInTheDocument();
      for (const prompt of AGENT_PROMPTS) {
        expect(within(menu).queryByText(prompt)).not.toBeInTheDocument();
      }
    });

    it("Should keep agent prompts on the chip of the session's own branch", async () => {
      const item = needsWork(SESSION_LINK_SETS.onePr[0]!);
      const onSendPrompt = vi.fn();
      renderWithTopbar(
        <SessionPublisher
          linkedWork={model([item])}
          onSendPrompt={onSendPrompt}
          onStop={vi.fn()}
          session={{ ...withPullRequest(), branch: item.branch }}
        />
      );
      fireEvent.click(screen.getByTestId("session-pull-request-chip"));
      const menu = await screen.findByTestId("session-pull-request-menu");
      fireEvent.click(within(menu).getByText("Fix failing checks"));
      expect(onSendPrompt).toHaveBeenCalledTimes(1);
    });

    it("Should offer agent prompts in the list only on the session's own branch", async () => {
      const [own, other] = SESSION_LINK_SETS.mixed.slice(0, 2).map(item => needsWork(item));
      renderWithTopbar(
        <SessionPublisher
          linkedWork={model([own!, other!])}
          onSendPrompt={vi.fn()}
          onStop={vi.fn()}
          session={{ ...withPullRequest(), branch: own!.branch }}
        />
      );
      fireEvent.click(screen.getByTestId("session-links-summary"));
      const list = await screen.findByRole("dialog");
      fireEvent.click(
        within(list).getByRole("button", { name: `Actions for ${own!.pull_request!.title}` })
      );
      const ownMenu = await screen.findByTestId("session-pull-request-menu");
      expect(within(ownMenu).getByText("Fix failing checks")).toBeInTheDocument();
      fireEvent.keyDown(ownMenu, { key: "Escape" });
      await waitFor(() =>
        expect(screen.queryByTestId("session-pull-request-menu")).not.toBeInTheDocument()
      );
      fireEvent.click(
        within(list).getByRole("button", { name: `Actions for ${other!.pull_request!.title}` })
      );
      const otherMenu = await screen.findByTestId("session-pull-request-menu");
      expect(within(otherMenu).getByText("Open PR")).toBeInTheDocument();
      expect(within(otherMenu).queryByText("Fix failing checks")).not.toBeInTheDocument();
    });

    it("Should open a lone rolled-up item's child session from the chip menu", async () => {
      const onOpenSession = vi.fn();
      const child = SESSION_LINK_SETS.rolledUp[0]!;
      renderWithTopbar(
        <SessionPublisher
          linkedWork={model([child])}
          onOpenSession={onOpenSession}
          onStop={vi.fn()}
        />
      );
      expect(screen.queryByTestId("session-links-summary")).not.toBeInTheDocument();
      fireEvent.click(screen.getByTestId("session-pull-request-chip"));
      const menu = await screen.findByTestId("session-pull-request-menu");
      fireEvent.click(within(menu).getByText("Open session"));
      expect(onOpenSession).toHaveBeenCalledWith(child.source_session_id);
    });

    it("Should not offer Open session on the session's own direct item", async () => {
      renderLinks(model(SESSION_LINK_SETS.onePr));
      fireEvent.click(screen.getByTestId("session-pull-request-chip"));
      const menu = await screen.findByTestId("session-pull-request-menu");
      expect(within(menu).queryByText("Open session")).not.toBeInTheDocument();
    });

    it("Should leave Unlink off a rolled-up row, whose target is the child session", async () => {
      const onOpenSession = vi.fn();
      renderWithTopbar(
        <SessionPublisher
          linkedWork={model(SESSION_LINK_SETS.rolledUp)}
          onOpenSession={onOpenSession}
          onStop={vi.fn()}
        />
      );
      fireEvent.click(screen.getByTestId("session-links-summary"));
      const list = await screen.findByRole("dialog");
      expect(list).toHaveTextContent("From Login redirect fix");
      fireEvent.click(
        within(list).getByRole("button", {
          name: "Open Fix the login redirect loop in its session",
        })
      );
      expect(onOpenSession).toHaveBeenCalledWith("s2");
      fireEvent.click(
        within(list).getByRole("button", { name: "Actions for Fix the login redirect loop" })
      );
      const menu = await screen.findByTestId("session-pull-request-menu");
      expect(within(menu).getByText("Open PR")).toBeInTheDocument();
      expect(within(menu).queryByText("Unlink from this session")).not.toBeInTheDocument();
      expect(within(menu).queryByText("Fix failing checks")).not.toBeInTheDocument();
    });

    it("Should unlink a direct row from its menu", async () => {
      const unlink = vi.fn();
      renderLinks(model(SESSION_LINK_SETS.mixed, { unlink }));
      fireEvent.click(screen.getByTestId("session-links-summary"));
      const list = await screen.findByRole("dialog");
      fireEvent.click(
        within(list).getByRole("button", { name: "Actions for Drop the old session cache" })
      );
      const menu = await screen.findByTestId("session-pull-request-menu");
      fireEvent.click(within(menu).getByText("Unlink from this session"));
      expect(unlink.mock.calls[0]?.[0]).toMatchObject({ pr_number: 939 });
    });

    it("Should keep the list reachable from the actions once the head clips the status", async () => {
      let report: IntersectionObserverCallback = () => undefined;
      vi.stubGlobal(
        "IntersectionObserver",
        class {
          constructor(callback: IntersectionObserverCallback) {
            report = callback;
          }
          observe() {}
          disconnect() {}
        }
      );
      const items = SESSION_LINK_SETS.mixed;
      const setClipped = vi.fn();
      const view = renderLinks(model(items, { setClipped }));
      expect(screen.queryByTestId("session-links-narrow")).not.toBeInTheDocument();
      act(() =>
        report(
          [{ intersectionRatio: 0.4 } as IntersectionObserverEntry],
          {} as IntersectionObserver
        )
      );
      expect(setClipped).toHaveBeenCalledWith(true);
      view.rerender(
        <SessionPublisher linkedWork={model(items, { clipped: true })} onStop={vi.fn()} />
      );
      const entry = screen.getByRole("button", { name: "4 linked to this session" });
      expect(entry.closest('[data-slot="topbar-actions"]')).not.toBeNull();
      fireEvent.click(entry);
      expect(await screen.findByRole("dialog")).toHaveTextContent("Linked work");
    });
  });
});

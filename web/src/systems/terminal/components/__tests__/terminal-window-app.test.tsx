import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";

import { destroyTerminalInstances } from "@compozy/ui";

import {
  CONTESTED_TERMINAL,
  DEV_SERVER_TERMINAL,
  exitedTerminal,
  MAKE_GATE_TERMINAL,
  PASSWORD_REQUEST,
  PSQL_TERMINAL,
  SSH_STAGING_TERMINAL,
  TERMINAL_FIXTURES,
  TERMINAL_FIXTURES_AT_CAP,
  TERMINAL_FIXTURE_PROFILE,
  TERMINAL_FIXTURE_VIEWER,
} from "../../mocks/terminal-fixtures";
import { TerminalWindowApp, type TerminalWindowAppProps } from "../terminal-window-app";
import { TERMINAL_CLIENT_OP, TERMINAL_SERVER_OP } from "../../lib/terminal-wire";
import {
  recordingSocketFactory,
  renderTerminalWindow,
  silentSocketFactory,
  stubEngineLoader,
  stubTerminalTicketFetch,
  stubWindowActions,
} from "./terminal-window-harness";

/**
 * Canonical suite for the terminal window (UT-083, UT-118).
 *
 * Invariant: every S1 state renders with its distinguishing behaviour, and the
 * two contention behaviours hold — displacing another person confirms by name
 * before any write, and tab overflow at the cap collapses rather than shrinking
 * tabs past legibility.
 */

const TERMINAL_LIMIT = 8;

function attachedPayload(overrides: Record<string, unknown> = {}) {
  return {
    seq: 0,
    truncated: false,
    lease: "human_owned",
    mode: "pty",
    cols: 96,
    rows: 28,
    ...overrides,
  };
}

function humanOwnerPayload(actorId: string) {
  return {
    lease: "human_owned",
    actor_kind: "human",
    actor_id: actorId,
    reason: "takeover",
  };
}

async function waitForTerminalRenderer(terminalId: string) {
  await act(async () => {
    await Promise.resolve();
  });
  await waitFor(() => {
    const view = screen
      .getByTestId(`terminal-pane-${terminalId}`)
      .querySelector('[data-slot="terminal-view"]');
    expect(view).toHaveAttribute("data-renderer");
  });
}

let restoreFetch: (() => void) | null = null;

afterEach(() => {
  restoreFetch?.();
  restoreFetch = null;
  destroyTerminalInstances(() => true);
});

function renderWindow(overrides: Partial<TerminalWindowAppProps> = {}) {
  restoreFetch = stubTerminalTicketFetch();
  const actions = overrides.actions ?? stubWindowActions();
  const view = renderTerminalWindow(
    <TerminalWindowApp
      actions={actions}
      engineLoader={stubEngineLoader}
      inputRequests={[]}
      interactiveAvailable
      journal={<div data-testid="journal-slot">journal</div>}
      limit={TERMINAL_LIMIT}
      profile={TERMINAL_FIXTURE_PROFILE}
      socketFactory={silentSocketFactory}
      terminals={TERMINAL_FIXTURES}
      viewerId={TERMINAL_FIXTURE_VIEWER}
      workspaceId="ws-atlas"
      {...overrides}
    />
  );
  return { ...view, actions };
}

describe("TerminalWindowApp — S1 states", () => {
  it("Should render a controlled terminal with release rather than take control", async () => {
    renderWindow();

    await waitFor(() =>
      expect(screen.getByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`)).toBeInTheDocument()
    );
    expect(screen.getByTestId("terminal-lease-badge")).toHaveTextContent("You're in control");
    expect(screen.getByTestId("terminal-release-control")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-take-control")).not.toBeInTheDocument();
  });

  it("Should render a watched terminal read-only with one take-control action", async () => {
    renderWindow({ terminals: [PSQL_TERMINAL] });

    await waitFor(() =>
      expect(screen.getByTestId(`terminal-pane-${PSQL_TERMINAL.id}`)).toBeInTheDocument()
    );
    expect(screen.getByTestId("terminal-lease-badge")).toHaveTextContent(
      "Claude Code is in control"
    );
    expect(screen.getByTestId("terminal-take-control")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-release-control")).not.toBeInTheDocument();
    // Watching is read-only, and the grid says so through its own label.
    await waitFor(() =>
      expect(
        screen.getByRole("log", { name: `${PSQL_TERMINAL.title} — watching` })
      ).toBeInTheDocument()
    );
  });

  it("Should render a pipe terminal as a log with no interactive affordance", async () => {
    renderWindow({
      terminals: [MAKE_GATE_TERMINAL],
      pipeOutput: {
        [MAKE_GATE_TERMINAL.id]: {
          firstLineNumber: 412,
          lines: ["ok   internal/store  4.021s", "web:test: 227 passed (14.2s)"],
        },
      },
    });

    expect(screen.getByTestId(`terminal-pipe-pane-${MAKE_GATE_TERMINAL.id}`)).toBeInTheDocument();
    expect(screen.getByText("412")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-pipe-chip")).toHaveTextContent("read-only log");
    // Absent, not disabled.
    expect(screen.queryByTestId("terminal-take-control")).not.toBeInTheDocument();
    expect(screen.queryByTestId("terminal-release-control")).not.toBeInTheDocument();
    expect(screen.queryByTestId("terminal-stop")).not.toBeInTheDocument();
  });

  it("Should offer opening the first terminal when the project has none", async () => {
    const actions = stubWindowActions();
    renderWindow({ actions, terminals: [] });

    expect(screen.getByTestId("terminal-empty")).toBeInTheDocument();
    await userEvent.click(screen.getByTestId("terminal-empty-open"));
    expect(actions.onOpenTerminal).toHaveBeenCalledOnce();
  });

  it("Should offer the way out of the project cap instead of a dead control", async () => {
    const actions = stubWindowActions();
    renderWindow({ actions, terminals: TERMINAL_FIXTURES_AT_CAP });

    await userEvent.click(screen.getByTestId("terminal-open"));

    expect(actions.onOpenTerminal).not.toHaveBeenCalled();
    const dialog = await screen.findByTestId("terminal-limit-dialog");
    expect(dialog).toHaveTextContent("This project is at its terminal limit");
    expect(dialog).toHaveTextContent(
      `${TERMINAL_FIXTURES_AT_CAP.length} of ${TERMINAL_LIMIT} terminals are open`
    );
    expect(dialog).toHaveTextContent("terminal.max_per_workspace 8");
    // Every terminal you could close is listed — the list is the choice.
    for (const terminal of TERMINAL_FIXTURES_AT_CAP) {
      expect(screen.getByTestId(`terminal-limit-row-${terminal.id}`)).toBeInTheDocument();
    }
    // A finished terminal is preselected: closing it costs nothing.
    const finished = TERMINAL_FIXTURES_AT_CAP.find(terminal => terminal.state === "exited");
    if (!finished) throw new Error("fixture must include a finished terminal");
    expect(screen.getByTestId(`terminal-limit-row-${finished.id}`)).toHaveAttribute(
      "aria-pressed",
      "true"
    );

    await userEvent.click(screen.getByTestId("terminal-limit-close"));
    expect(actions.onCloseTerminal).toHaveBeenCalledWith(finished.id);
  });

  it("Should say what the connection is doing, and never fake output for it", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });

    // Waiting is stated in one line over an empty grid.
    const connecting = await screen.findByTestId("terminal-connecting");
    expect(connecting).toHaveTextContent("Connecting…");
    expect(connecting).toHaveAttribute("data-status", "connecting");

    await socket.deliver(TERMINAL_SERVER_OP.attached, attachedPayload());
    await waitFor(() => expect(screen.queryByTestId("terminal-connecting")).toBeNull());

    const passesBeforeDrop = socket.connectionCount();
    await socket.drop();
    const reconnecting = await screen.findByTestId("terminal-connecting");
    expect(reconnecting).toHaveTextContent("Reconnecting…");
    expect(reconnecting).toHaveAttribute("data-status", "reconnecting");
    expect(screen.getByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`)).toBeInTheDocument();
    await socket.readyForConnectionCount(passesBeforeDrop + 1);
    await socket.deliver(TERMINAL_SERVER_OP.attached, attachedPayload());

    // A gap rebuilds the screen from the server, and says so while it does.
    await socket.deliver(TERMINAL_SERVER_OP.gap, {
      from_seq: 100,
      to_seq: 49_252,
      dropped_bytes: 49_152,
    });
    await waitFor(() =>
      expect(screen.getByTestId("terminal-connecting")).toHaveTextContent("Catching up…")
    );
  });

  it("Should offer a retry only where retrying is the remedy", async () => {
    const socket = recordingSocketFactory();
    const { rerender } = renderWindow({
      socketFactory: socket.factory,
      terminals: [DEV_SERVER_TERMINAL],
    });
    await screen.findByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`);

    for (const code of ["ticket_expired", "ticket_invalid"]) {
      await socket.deliver(TERMINAL_SERVER_OP.error, { code, message: `refused: ${code}` });
      const notice = await screen.findByTestId(`terminal-notice-${code}`);
      // A pass can be minted again, so reconnecting is a real remedy.
      expect(within(notice).getByRole("button", { name: "Reconnect" })).toBeInTheDocument();
    }

    await socket.deliver(TERMINAL_SERVER_OP.error, {
      code: "subscriber_limit_reached",
      message: "16 viewers already attached",
    });
    const full = await screen.findByTestId("terminal-notice-subscriber_limit_reached");
    // Nobody left, so retrying immediately would only fail again.
    expect(full).toHaveTextContent("subscriber_limit_reached");
    expect(within(full).queryByRole("button", { name: "Reconnect" })).not.toBeInTheDocument();
    rerender(<div />);
  });

  it("Should keep watching while a slow viewer is told it fell behind", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });
    await screen.findByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`);

    await socket.deliver(TERMINAL_SERVER_OP.error, {
      code: "slow_consumer",
      message: "viewer queue was full for 10s",
    });

    const notice = await screen.findByTestId("terminal-notice-slow_consumer");
    expect(within(notice).getByRole("button", { name: "Reconnect" })).toBeInTheDocument();
    // The grid stays mounted: falling behind is not the same as disconnecting.
    expect(screen.getByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`)).toBeInTheDocument();
  });

  it("Should state a cause the daemon could not verify rather than invent a code", async () => {
    const unknownEnd = exitedTerminal({ cause: "unknown", at: "2026-08-25T12:41:00Z" });
    renderWindow({ terminals: [unknownEnd] });

    const bar = await screen.findByTestId("terminal-exit-bar");
    expect(bar).toHaveTextContent("Ended");
    expect(bar).toHaveTextContent("cause unknown");
    expect(bar).toHaveTextContent("CompozyOS couldn't see how this ended");
    // No exit code is shown, because none exists.
    expect(bar).not.toHaveTextContent(/exit \d/);
  });

  it("Should keep the cap dialog usable after the terminal it named is gone", async () => {
    const actions = stubWindowActions();
    const { rerender } = renderWindow({ actions, terminals: TERMINAL_FIXTURES_AT_CAP });
    const removed = TERMINAL_FIXTURES_AT_CAP[0];

    await userEvent.click(screen.getByTestId("terminal-open"));
    await userEvent.click(await screen.findByTestId(`terminal-limit-row-${removed.id}`));
    await userEvent.click(screen.getByTestId("terminal-limit-close"));

    // The chosen terminal is gone, and the project fills up again.
    const remaining = TERMINAL_FIXTURES_AT_CAP.filter(t => t.id !== removed.id);
    rerender(
      <TerminalWindowApp
        actions={actions}
        engineLoader={stubEngineLoader}
        inputRequests={[]}
        interactiveAvailable
        journal={<div data-testid="journal-slot">journal</div>}
        limit={remaining.length}
        profile={TERMINAL_FIXTURE_PROFILE}
        socketFactory={silentSocketFactory}
        terminals={remaining}
        viewerId={TERMINAL_FIXTURE_VIEWER}
        workspaceId="ws-atlas"
      />
    );
    await userEvent.click(screen.getByTestId("terminal-open"));

    // Reopening falls back to a terminal that still exists, so the way out of
    // the cap is still a way out.
    const close = await screen.findByTestId("terminal-limit-close");
    expect(close).toBeEnabled();
    await userEvent.click(close);
    expect(actions.onCloseTerminal).toHaveBeenCalledTimes(2);
    expect(actions.onCloseTerminal).toHaveBeenLastCalledWith(
      remaining.find(t => t.state === "exited")?.id ?? remaining[0].id
    );
  });

  it("Should state an execute-only platform instead of offering a screen", () => {
    renderWindow({ interactiveAvailable: false });

    expect(screen.getByTestId("terminal-execute-only")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-tabs")).not.toBeInTheDocument();
    expect(screen.queryByTestId("terminal-open")).not.toBeInTheDocument();
  });

  it("Should keep an exited terminal readable with its outcome", async () => {
    renderWindow({ terminals: [SSH_STAGING_TERMINAL] });

    await waitFor(() =>
      expect(screen.getByTestId(`terminal-pane-${SSH_STAGING_TERMINAL.id}`)).toBeInTheDocument()
    );
    expect(screen.getByTestId(`terminal-tab-${SSH_STAGING_TERMINAL.id}`)).toHaveTextContent(
      "exit 0"
    );
  });

  it("Should pause commands while the journal cannot record, without stopping output", async () => {
    renderWindow({ auditBlockedIds: new Set([DEV_SERVER_TERMINAL.id]) });

    await waitFor(() => expect(screen.getByTestId("terminal-audit-blocked")).toBeInTheDocument());
    expect(screen.getByTestId("terminal-audit-blocked")).toHaveTextContent(
      "New commands are paused."
    );
    // Watching continues: the grid is still mounted behind the bar.
    expect(screen.getByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`)).toBeInTheDocument();
  });

  it("Should mark the tab of a terminal that is waiting on an answer", async () => {
    renderWindow({ inputRequests: [PASSWORD_REQUEST] });

    await waitForTerminalRenderer(DEV_SERVER_TERMINAL.id);

    expect(
      screen.getByTestId(`terminal-tab-attention-${PASSWORD_REQUEST.terminal_id}`)
    ).toBeInTheDocument();
  });

  it("Should show a recording as running, with the way to stop it", async () => {
    renderWindow({ recordings: { [DEV_SERVER_TERMINAL.id]: { elapsed: "02:14" } } });

    await waitForTerminalRenderer(DEV_SERVER_TERMINAL.id);

    expect(screen.getByTestId("terminal-recording-chip")).toHaveTextContent("rec 02:14");
    expect(screen.getByTestId("terminal-stop-recording")).toBeInTheDocument();
  });

  it("Should open the journal from its pinned tab", async () => {
    renderWindow();

    await userEvent.click(screen.getByTestId("terminal-tab-journal"));

    expect(screen.getByTestId("journal-slot")).toBeInTheDocument();
  });
});

describe("TerminalWindowApp — contention", () => {
  it("Should take control of an agent's terminal immediately, without asking", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [PSQL_TERMINAL] });

    await userEvent.click(screen.getByTestId("terminal-take-control"));

    // Exactly one TAKEOVER, unforced: displacing an agent never asks.
    expect(socket.sentWithOp(TERMINAL_CLIENT_OP.takeover)).toEqual([
      { op: TERMINAL_CLIENT_OP.takeover, payload: { force: false } },
    ]);
    expect(screen.queryByTestId("terminal-takeover-dialog")).not.toBeInTheDocument();
  });

  it("Should confirm by name before displacing another person", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [CONTESTED_TERMINAL] });

    await userEvent.click(screen.getByTestId("terminal-take-control"));

    // Nothing reaches the wire until the confirmation lands.
    expect(socket.sentWithOp(TERMINAL_CLIENT_OP.takeover)).toEqual([]);
    const dialog = await screen.findByTestId("terminal-takeover-dialog");
    expect(dialog).toHaveTextContent("Take control from marina?");
    expect(dialog).toHaveTextContent(CONTESTED_TERMINAL.title);

    await userEvent.click(screen.getByTestId("terminal-takeover-confirm"));

    expect(socket.sentWithOp(TERMINAL_CLIENT_OP.takeover)).toEqual([
      { op: TERMINAL_CLIENT_OP.takeover, payload: { force: true } },
    ]);
  });

  it("Should send nothing when the displacement is cancelled", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [CONTESTED_TERMINAL] });

    await userEvent.click(screen.getByTestId("terminal-take-control"));
    await screen.findByTestId("terminal-takeover-dialog");
    await userEvent.keyboard("{Escape}");

    await waitFor(() =>
      expect(screen.queryByTestId("terminal-takeover-dialog")).not.toBeInTheDocument()
    );
    expect(socket.sentWithOp(TERMINAL_CLIENT_OP.takeover)).toEqual([]);
  });

  it("Should read control from the daemon's own frames, not from the catalog", async () => {
    const socket = recordingSocketFactory();
    // The catalog says the agent holds it; that is the starting point only.
    renderWindow({ socketFactory: socket.factory, terminals: [PSQL_TERMINAL] });

    await waitFor(() => expect(screen.getByTestId("terminal-take-control")).toBeInTheDocument());
    await socket.ready();
    await socket.deliver(TERMINAL_SERVER_OP.attached, attachedPayload({ lease: "agent_owned" }));
    // The daemon then hands the lease to this viewer.
    await socket.deliver(TERMINAL_SERVER_OP.owner, humanOwnerPayload(TERMINAL_FIXTURE_VIEWER));

    await waitFor(() => expect(screen.getByTestId("terminal-release-control")).toBeInTheDocument());
    expect(screen.queryByTestId("terminal-take-control")).not.toBeInTheDocument();
  });

  it("Should give control back by detaching rather than by claiming locally", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });

    const release = await screen.findByTestId("terminal-release-control");
    await userEvent.click(release);

    expect(socket.sentWithOp(TERMINAL_CLIENT_OP.detach)).toEqual([
      { op: TERMINAL_CLIENT_OP.detach, payload: {} },
    ]);
    // The chip does not move until the daemon says the lease moved.
    expect(screen.getByTestId("terminal-release-control")).toBeInTheDocument();
  });

  it("Should announce a change of control without announcing the whole head", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [PSQL_TERMINAL] });

    // Only the sentence is live. Marking the chip, the glyph and the viewer
    // count live would repeat all of it on every render.
    const label = await screen.findByTestId("terminal-lease-label");
    expect(label).toHaveAttribute("aria-live", "polite");
    expect(screen.getByTestId("terminal-lease-badge")).not.toHaveAttribute("aria-live");

    await socket.ready();
    await socket.deliver(TERMINAL_SERVER_OP.owner, humanOwnerPayload(TERMINAL_FIXTURE_VIEWER));

    await waitFor(() =>
      expect(screen.getByTestId("terminal-lease-label")).toHaveTextContent("You're in control")
    );
  });

  it("Should offer the journal, not a retry, once the terminal is gone", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });
    await screen.findByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`);

    await socket.ready();
    await socket.deliver(TERMINAL_SERVER_OP.error, {
      code: "terminal_not_found",
      message: "no terminal term-4f21c9a03b7e",
    });

    // Nothing to reconnect to; everything it ran is still recorded.
    const notice = await screen.findByTestId("terminal-notice-terminal_not_found");
    expect(notice).not.toHaveTextContent("Reconnect");
    await userEvent.click(screen.getByTestId("terminal-notice-view-journal"));
    expect(screen.getByTestId("journal-slot")).toBeInTheDocument();
  });

  it("Should show the daemon's own words for a refusal it has no copy for", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });
    await screen.findByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`);

    await socket.ready();
    await socket.deliver(TERMINAL_SERVER_OP.error, {
      code: "some_future_refusal",
      message: "The daemon refused for a reason this client predates.",
    });

    expect(await screen.findByTestId("terminal-notice-some_future_refusal")).toHaveTextContent(
      "The daemon refused for a reason this client predates."
    );
  });

  it("Should state the grid the daemon settled on", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });

    await waitForTerminalRenderer(DEV_SERVER_TERMINAL.id);
    await socket.ready();
    await socket.deliver(
      TERMINAL_SERVER_OP.attached,
      attachedPayload({
        // The smallest controlling viewer decides, so this is frequently not the
        // size this window would have chosen.
        cols: 80,
        rows: 24,
      })
    );
    await waitFor(() =>
      expect(screen.getByTestId("terminal-grid-chip")).toHaveTextContent("80×24")
    );
  });

  it("Should collapse the surplus tabs at the cap instead of shrinking them", async () => {
    renderWindow({ terminals: TERMINAL_FIXTURES_AT_CAP });

    const overflow = screen.getByTestId("terminal-tab-overflow");
    expect(overflow).toHaveAttribute("aria-label", "3 more terminals");
    // Every remaining tab keeps its own row in the strip rather than shrinking.
    expect(screen.getAllByRole("tab").length).toBeLessThan(TERMINAL_FIXTURES_AT_CAP.length + 1);

    await userEvent.click(overflow);

    expect(await screen.findByText("e2e suite")).toBeInTheDocument();
  });

  it("Should keep the terminal you are looking at on the strip", async () => {
    renderWindow({ terminals: TERMINAL_FIXTURES_AT_CAP });
    const hidden = TERMINAL_FIXTURES_AT_CAP[TERMINAL_FIXTURES_AT_CAP.length - 1];

    await userEvent.click(screen.getByTestId("terminal-tab-overflow"));
    await userEvent.click(await screen.findByText(hidden.title));

    // Selecting from the surplus promotes it: a pane no visible tab claims,
    // with no tab marked selected, is not a tablist.
    const tab = await screen.findByTestId(`terminal-tab-select-${hidden.id}`);
    expect(tab).toHaveAttribute("aria-selected", "true");
    expect(tab).toHaveAttribute("tabindex", "0");
  });

  it("Should move selection and focus together across the tab strip", async () => {
    renderWindow();
    const first = TERMINAL_FIXTURES[0];
    const second = TERMINAL_FIXTURES[1];

    const tab = screen.getByTestId(`terminal-tab-select-${first.id}`);
    tab.focus();
    await userEvent.keyboard("{ArrowRight}");

    // Focus travels with the selection: the tab left behind becomes
    // unreachable by Tab the moment it stops being selected.
    await waitFor(() =>
      expect(screen.getByTestId(`terminal-tab-select-${second.id}`)).toHaveFocus()
    );
    expect(screen.getByTestId(`terminal-tab-select-${second.id}`)).toHaveAttribute(
      "aria-selected",
      "true"
    );
    expect(screen.getByTestId(`terminal-tab-select-${first.id}`)).toHaveAttribute("tabindex", "-1");
  });
});

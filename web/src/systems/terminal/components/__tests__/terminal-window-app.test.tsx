import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { destroyTerminalInstances } from "@compozy/ui";

import {
  ANSWERED_PASSWORD_REQUEST,
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
 * Canonical suite for the terminal window (UT-083, UT-118, UT-122).
 *
 * Invariant: one terminal per OS window — every S1 state renders with its
 * distinguishing behaviour, the id-less route resolves itself (adopt the newest
 * unwindowed running terminal, else create once, cap surfaced instead of a dead
 * create), and the two contention behaviours hold — displacing another person
 * confirms by name before any write, and an unchanged browser identity keeps
 * one writable attachment. A pending question stays on screen for a watcher or
 * aggregate read, with the write row absent; resolved rows from the host
 * projection, including "by you", stay on the same stack.
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
    expect(screen.getByTestId("terminal-lease-badge")).toHaveAttribute("data-lease", "me");
    expect(screen.getByTestId("terminal-lease-badge")).toHaveTextContent("You're in control");
    expect(screen.getByTestId("terminal-release-control")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-take-control")).not.toBeInTheDocument();
  });

  it("Should render a watched terminal read-only with one take-control action", async () => {
    renderWindow({ terminals: [PSQL_TERMINAL] });

    await waitFor(() =>
      expect(screen.getByTestId(`terminal-pane-${PSQL_TERMINAL.id}`)).toBeInTheDocument()
    );
    expect(screen.getByTestId("terminal-lease-badge")).toHaveAttribute("data-lease", "agent");
    expect(screen.getByTestId("terminal-lease-badge")).toHaveTextContent(
      "Claude Code is in control"
    );
    expect(screen.getByTestId("terminal-take-control")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-release-control")).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole("log", { name: PSQL_TERMINAL.title })).toBeInTheDocument()
    );
    expect(screen.queryByRole("log", { name: `${PSQL_TERMINAL.title} — watching` })).toBeNull();
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
    // Wait and Close are the pipe head verbs; Signal lives in overflow.
    expect(screen.getByTestId("terminal-wait")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-close")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-signal")).not.toBeInTheDocument();
    await userEvent.click(screen.getByTestId("terminal-pipe-overflow"));
    expect(await screen.findByTestId("terminal-signal")).toBeInTheDocument();
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

    await userEvent.click(screen.getByTestId("terminal-new"));

    expect(actions.onOpenTerminal).not.toHaveBeenCalled();
    const dialog = await screen.findByTestId("terminal-limit-dialog");
    expect(dialog).toHaveTextContent("This project is at its terminal limit");
    expect(dialog).toHaveTextContent(
      `${TERMINAL_FIXTURES_AT_CAP.length} of ${TERMINAL_LIMIT} terminals are open`
    );
    expect(dialog).toHaveTextContent("terminal.max_per_workspace 8");
    // Every running terminal you could close is listed — the list is the choice.
    for (const terminal of TERMINAL_FIXTURES_AT_CAP) {
      expect(screen.getByTestId(`terminal-limit-row-${terminal.id}`)).toBeInTheDocument();
    }
    expect(
      screen.getByTestId(`terminal-limit-row-${TERMINAL_FIXTURES_AT_CAP[0].id}`)
    ).toHaveAttribute("aria-pressed", "true");

    await userEvent.click(screen.getByTestId("terminal-limit-close"));
    expect(actions.onCloseTerminal).toHaveBeenCalledWith(TERMINAL_FIXTURES_AT_CAP[0].id);
  });

  it("Should say what the connection is doing, and never fake output for it", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });

    // Waiting is stated in one line over an invisible grid — socket open is not attached.
    const connecting = await screen.findByTestId("terminal-connecting");
    expect(connecting).toHaveTextContent("Connecting…");
    expect(connecting).toHaveAttribute("data-status", "connecting");
    expect(screen.getByRole("log", { name: DEV_SERVER_TERMINAL.title })).toHaveClass("invisible");

    await socket.deliver(TERMINAL_SERVER_OP.attached, attachedPayload());
    await waitFor(() => expect(screen.queryByTestId("terminal-connecting")).toBeNull());
    expect(screen.getByRole("log", { name: DEV_SERVER_TERMINAL.title })).not.toHaveClass(
      "invisible"
    );

    const passesBeforeDrop = socket.connectionCount();
    await socket.drop();
    const reconnecting = await screen.findByTestId("terminal-connecting");
    expect(reconnecting).toHaveTextContent("Reconnecting…");
    expect(reconnecting).toHaveAttribute("data-status", "reconnecting");
    expect(screen.getByRole("log", { name: DEV_SERVER_TERMINAL.title })).not.toHaveClass(
      "invisible"
    );
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

  it("Should keep one writable attachment when the browser identity is unchanged", async () => {
    const socket = recordingSocketFactory();
    renderWindow({
      socketFactory: socket.factory,
      terminals: [DEV_SERVER_TERMINAL],
      viewerToken: "attachment-token",
    });

    await socket.readyForConnectionCount(1);
    await socket.deliver(TERMINAL_SERVER_OP.attached, attachedPayload());
    await socket.deliver(TERMINAL_SERVER_OP.owner, humanOwnerPayload(TERMINAL_FIXTURE_VIEWER));

    await waitFor(() =>
      expect(screen.getByRole("log", { name: DEV_SERVER_TERMINAL.title })).toBeInTheDocument()
    );
    expect(socket.connectionCount()).toBe(1);
  });

  it("Should offer a retry only where retrying is the remedy", async () => {
    const socket = recordingSocketFactory();
    const { rerender } = renderWindow({
      socketFactory: socket.factory,
      terminals: [DEV_SERVER_TERMINAL],
    });
    await screen.findByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`);

    for (const code of ["ticket_expired", "ticket_invalid"]) {
      await socket.deliver(TERMINAL_SERVER_OP.error, {
        error: { code, message: `refused: ${code}` },
      });
      const notice = await screen.findByTestId(`terminal-notice-${code}`);
      // A pass can be minted again, so reconnecting is a real remedy.
      expect(within(notice).getByRole("button", { name: "Reconnect" })).toBeInTheDocument();
    }

    await socket.deliver(TERMINAL_SERVER_OP.error, {
      error: {
        code: "subscriber_limit_reached",
        message: "16 viewers already attached",
      },
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
      error: { code: "slow_consumer", message: "viewer queue was full for 10s" },
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

    await userEvent.click(screen.getByTestId("terminal-new"));
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
    await userEvent.click(screen.getByTestId("terminal-new"));

    // Reopening falls back to a terminal that still exists, so the way out of
    // the cap is still a way out.
    const close = await screen.findByTestId("terminal-limit-close");
    expect(close).toBeEnabled();
    await userEvent.click(close);
    expect(actions.onCloseTerminal).toHaveBeenCalledTimes(2);
    expect(actions.onCloseTerminal).toHaveBeenLastCalledWith(remaining[0].id);
  });

  it("Should state an execute-only platform instead of offering a screen", async () => {
    renderWindow({ interactiveAvailable: false, terminals: [] });

    expect(screen.getByTestId("terminal-execute-only")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-new")).not.toBeInTheDocument();
    expect(screen.getByTestId("terminal-execute-only")).not.toHaveTextContent("On this platform");

    await userEvent.click(screen.getByRole("button", { name: "View journal" }));
    expect(screen.queryByTestId("terminal-execute-only")).not.toBeInTheDocument();
    expect(screen.getByTestId("journal-slot")).toBeVisible();

    await userEvent.click(screen.getByTestId("terminal-journal-back"));
    expect(screen.getByTestId("terminal-execute-only")).toBeInTheDocument();
    expect(screen.getByTestId("journal-slot")).not.toBeVisible();
  });

  it("Should publish identity into the OS head instead of drawing a second row", async () => {
    renderWindow({ hostChrome: true });
    await waitForTerminalRenderer(DEV_SERVER_TERMINAL.id);

    expect(screen.queryByTestId("terminal-header")).not.toBeInTheDocument();
    expect(screen.queryByTestId("terminal-journal-head")).not.toBeInTheDocument();
    expect(screen.getByTestId("terminal-window")).toBeInTheDocument();
  });

  it("Should keep an exited terminal readable with its outcome", async () => {
    renderWindow({ terminals: [SSH_STAGING_TERMINAL] });

    await waitFor(() =>
      expect(screen.getByTestId(`terminal-pane-${SSH_STAGING_TERMINAL.id}`)).toBeInTheDocument()
    );
    const bar = screen.getByTestId("terminal-exit-bar");
    expect(bar).toHaveTextContent("Succeeded");
    expect(bar).toHaveTextContent("exit 0");
  });

  it("Should pause commands while the journal cannot record, without stopping output", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });
    await screen.findByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`);

    await socket.deliver(TERMINAL_SERVER_OP.error, {
      error: { code: "journal_unavailable", message: "journal_unavailable" },
    });

    const notice = await screen.findByTestId("terminal-notice-journal_unavailable");
    expect(notice).toHaveTextContent("New commands are paused.");
    // Watching continues: the grid is still mounted behind the stream warning.
    expect(screen.getByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`)).toBeInTheDocument();
  });

  it("Should let a destination-profile watcher answer through an atomic handoff", async () => {
    const request = { ...PASSWORD_REQUEST, requested_at: new Date().toISOString() };
    renderWindow({ inputRequests: [request], terminals: [PSQL_TERMINAL] });

    await waitFor(() =>
      expect(screen.getByTestId(`terminal-input-request-${request.id}`)).toBeInTheDocument()
    );
    expect(screen.getByText(request.reason)).toBeInTheDocument();
    expect(screen.getByTestId(`terminal-input-request-field-${request.id}`)).toBeInTheDocument();
    expect(screen.getByTestId(`terminal-input-request-send-${request.id}`)).toHaveTextContent(
      "Take control & send"
    );
  });

  it("Should keep the question on an aggregate read without offering a write row", async () => {
    const request = { ...PASSWORD_REQUEST, terminal_id: DEV_SERVER_TERMINAL.id };
    renderWindow({
      inputRequests: [request],
      readOnly: true,
      terminals: [DEV_SERVER_TERMINAL],
    });

    await waitFor(() =>
      expect(screen.getByTestId(`terminal-input-request-${request.id}`)).toBeInTheDocument()
    );
    expect(screen.getByText(request.reason)).toBeInTheDocument();
    expect(
      screen.queryByTestId(`terminal-input-request-field-${request.id}`)
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId(`terminal-input-request-send-${request.id}`)
    ).not.toBeInTheDocument();
  });

  it("Should send directly only with a writable lease on a destination profile", async () => {
    const request = {
      ...PASSWORD_REQUEST,
      requested_at: new Date().toISOString(),
      terminal_id: DEV_SERVER_TERMINAL.id,
    };
    renderWindow({
      inputRequestTitles: new Map([[DEV_SERVER_TERMINAL.id, DEV_SERVER_TERMINAL.title]]),
      inputRequests: [request],
      terminals: [DEV_SERVER_TERMINAL],
    });

    await waitFor(() =>
      expect(screen.getByTestId(`terminal-input-request-send-${request.id}`)).toHaveTextContent(
        "Send"
      )
    );
    expect(screen.getByTestId("terminal-input-request-stack")).toBeInTheDocument();
  });

  it("Should show a resolved answer as by you when the host passes the projection", async () => {
    renderWindow({
      resolvedInputRequests: [ANSWERED_PASSWORD_REQUEST],
      terminals: [PSQL_TERMINAL],
    });

    await waitFor(() =>
      expect(screen.getByTestId("terminal-input-resolved-answered")).toBeInTheDocument()
    );
    expect(screen.getByText("Answered by you")).toBeInTheDocument();
    expect(screen.getByTestId("terminal-input-request-stack")).toBeInTheDocument();
  });

  it("Should show a recording as running, with the way to stop it", async () => {
    const view = renderWindow({
      recordings: { [DEV_SERVER_TERMINAL.id]: { elapsed: "02:14" } },
    });

    await waitForTerminalRenderer(DEV_SERVER_TERMINAL.id);

    expect(screen.getByTestId("terminal-recording-chip")).toHaveTextContent("rec 02:14");
    expect(screen.getByRole("button", { name: "Stop recording" })).toBeEnabled();

    view.rerender(
      <TerminalWindowApp
        actions={view.actions}
        engineLoader={stubEngineLoader}
        inputRequests={[]}
        interactiveAvailable
        journal={<div data-testid="journal-slot">journal</div>}
        limit={TERMINAL_LIMIT}
        profile={TERMINAL_FIXTURE_PROFILE}
        recordings={{}}
        socketFactory={silentSocketFactory}
        terminals={TERMINAL_FIXTURES}
        viewerId={TERMINAL_FIXTURE_VIEWER}
        workspaceId="ws-atlas"
      />
    );
    expect(screen.queryByTestId("terminal-recording-chip")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Stop recording" })).not.toBeInTheDocument();

    view.rerender(
      <TerminalWindowApp
        actions={view.actions}
        engineLoader={stubEngineLoader}
        inputRequests={[]}
        interactiveAvailable
        journal={<div data-testid="journal-slot">journal</div>}
        limit={TERMINAL_LIMIT}
        profile={TERMINAL_FIXTURE_PROFILE}
        recordings={{ [DEV_SERVER_TERMINAL.id]: { elapsed: "05:00" } }}
        socketFactory={silentSocketFactory}
        terminals={TERMINAL_FIXTURES}
        viewerId={TERMINAL_FIXTURE_VIEWER}
        workspaceId="ws-atlas"
      />
    );
    expect(screen.getByTestId("terminal-recording-chip")).toHaveTextContent("rec 05:00");
    expect(screen.getByRole("button", { name: "Stop recording" })).toBeEnabled();
  });

  it("Should open the journal from the head and return from its back affordance", async () => {
    const onViewJournal = vi.fn();
    const onLeaveJournal = vi.fn();
    renderWindow({ onLeaveJournal, onViewJournal });

    await userEvent.click(screen.getByTestId("terminal-journal-toggle"));

    expect(screen.getByTestId("journal-slot")).toBeVisible();
    expect(onViewJournal).toHaveBeenCalledOnce();

    await userEvent.click(screen.getByTestId("terminal-journal-back"));
    expect(screen.getByTestId("journal-slot")).not.toBeVisible();
    expect(onLeaveJournal).toHaveBeenCalledOnce();
  });
});

describe("TerminalWindowApp — contention", () => {
  it("Should take control of an agent's terminal immediately, without asking", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [PSQL_TERMINAL] });

    await socket.ready();
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

    await socket.ready();
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

  it("Should give control back explicitly rather than by claiming locally", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });

    await socket.ready();
    const release = await screen.findByTestId("terminal-release-control");
    await userEvent.click(release);

    expect(socket.sentWithOp(TERMINAL_CLIENT_OP.release)).toEqual([
      { op: TERMINAL_CLIENT_OP.release, payload: {} },
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
      error: { code: "terminal_not_found", message: "no terminal term-4f21c9a03b7e" },
    });

    const gone = await screen.findByTestId("terminal-not-found");
    expect(gone).toHaveTextContent("terminal_not_found");
    expect(gone).not.toHaveTextContent("Reconnect");
    expect(gone).not.toHaveTextContent("It may have been closed");
    expect(screen.queryByTestId("terminal-notice-terminal_not_found")).not.toBeInTheDocument();
    await userEvent.click(within(gone).getByRole("button", { name: "View journal" }));
    expect(screen.getByTestId("journal-slot")).toBeVisible();
  });

  it("Should state a reclaimed terminal from the stream with its idle period", async () => {
    const socket = recordingSocketFactory();
    renderWindow({
      detachedTtl: "24h",
      socketFactory: socket.factory,
      terminals: [DEV_SERVER_TERMINAL],
    });
    await screen.findByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`);

    await socket.ready();
    await socket.deliver(TERMINAL_SERVER_OP.error, {
      error: { code: "terminal_expired", message: "terminal expired" },
    });

    const expired = await screen.findByTestId("terminal-expired");
    expect(expired).toHaveTextContent("reclaimed after 24h without viewers");
    expect(expired).not.toHaveTextContent("Nobody was watching");
    expect(within(expired).getByRole("button", { name: "View journal" })).toBeInTheDocument();
    expect(
      within(expired).getByRole("button", { name: "Open a new terminal" })
    ).toBeInTheDocument();
    expect(screen.getByTestId("terminal-new")).toBeInTheDocument();
  });

  it.each([
    { code: "terminal_expired", testId: "terminal-expired" },
    { code: "terminal_not_found", testId: "terminal-not-found" },
  ] as const)("Should gate the $code create CTA at the project cap", async ({ code, testId }) => {
    const actions = stubWindowActions();
    const socket = recordingSocketFactory();
    const current = TERMINAL_FIXTURES_AT_CAP[0];
    if (!current) throw new Error("cap fixture must include a current terminal");
    renderWindow({
      actions,
      requestedTerminalId: current.id,
      socketFactory: socket.factory,
      terminals: TERMINAL_FIXTURES_AT_CAP,
    });
    await screen.findByTestId(`terminal-pane-${current.id}`);

    await socket.ready();
    await socket.deliver(TERMINAL_SERVER_OP.error, {
      error: { code, message: code },
    });

    const state = await screen.findByTestId(testId);
    await userEvent.click(within(state).getByRole("button", { name: "Open a new terminal" }));

    expect(actions.onOpenTerminal).not.toHaveBeenCalled();
    expect(await screen.findByTestId("terminal-limit-dialog")).toBeInTheDocument();
  });

  it("Should say the routed terminal is gone with its history still reachable", () => {
    renderWindow({
      requestedTerminalId: "term-missing",
      terminals: TERMINAL_FIXTURES,
    });

    const gone = screen.getByTestId("terminal-not-found");
    expect(gone).toHaveTextContent("terminal_not_found");
    expect(within(gone).getByRole("button", { name: "View journal" })).toBeInTheDocument();
    expect(within(gone).getByRole("button", { name: "Open a new terminal" })).toBeInTheDocument();
  });

  it("Should show the daemon's own words for a refusal it has no copy for", async () => {
    const socket = recordingSocketFactory();
    renderWindow({ socketFactory: socket.factory, terminals: [DEV_SERVER_TERMINAL] });
    await screen.findByTestId(`terminal-pane-${DEV_SERVER_TERMINAL.id}`);

    await socket.ready();
    await socket.deliver(TERMINAL_SERVER_OP.error, {
      error: {
        code: "some_future_refusal",
        message: "The daemon refused for a reason this client predates.",
      },
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
    // The bar states the settled size whenever the daemon has said one; the
    // presence frame also keeps the viewers chip truthful in the same pass.
    await socket.deliver(TERMINAL_SERVER_OP.presence, { viewers: 2 });
    await waitFor(() =>
      expect(screen.getByTestId("terminal-size-vote")).toHaveTextContent("80×24")
    );
  });

  it("Should not claim a cap number until settings.max_per_workspace is known", async () => {
    const actions = stubWindowActions();
    renderWindow({
      actions,
      limit: undefined,
      terminals: TERMINAL_FIXTURES_AT_CAP,
    });

    expect(screen.queryByTestId("terminal-cap-count")).not.toBeInTheDocument();
    await userEvent.click(screen.getByTestId("terminal-new"));
    expect(actions.onOpenTerminal).toHaveBeenCalledOnce();
    expect(screen.queryByTestId("terminal-limit-dialog")).not.toBeInTheDocument();
  });

  it("Should count only the destination profile for the cap trail and the limit dialog", async () => {
    const actions = stubWindowActions();
    const personal = {
      ...DEV_SERVER_TERMINAL,
      id: "term-personal-other",
      profile_name: "personal",
      title: "personal notes",
    };
    renderWindow({
      actions,
      terminals: [...TERMINAL_FIXTURES_AT_CAP, personal],
    });

    expect(screen.getByTestId("terminal-cap-count")).toHaveTextContent(
      `${TERMINAL_FIXTURES_AT_CAP.length} of ${TERMINAL_LIMIT}`
    );

    await userEvent.click(screen.getByTestId("terminal-new"));
    expect(actions.onOpenTerminal).not.toHaveBeenCalled();
    const dialog = await screen.findByTestId("terminal-limit-dialog");
    expect(dialog).toHaveTextContent(
      `${TERMINAL_FIXTURES_AT_CAP.length} of ${TERMINAL_LIMIT} terminals are open`
    );
    expect(screen.queryByTestId(`terminal-limit-row-${personal.id}`)).not.toBeInTheDocument();
    for (const terminal of TERMINAL_FIXTURES_AT_CAP) {
      expect(screen.getByTestId(`terminal-limit-row-${terminal.id}`)).toBeInTheDocument();
    }
  });

  it("Should keep the journal mounted while the terminal is the surface again", async () => {
    renderWindow();

    await userEvent.click(screen.getByTestId("terminal-journal-toggle"));
    expect(screen.getByTestId("journal-slot")).toBeVisible();

    await userEvent.click(screen.getByTestId("terminal-journal-back"));
    expect(screen.getByTestId("journal-slot")).toBeInTheDocument();
    expect(screen.getByTestId("journal-slot")).not.toBeVisible();
  });
});

describe("TerminalWindowApp — id-less route resolver and close", () => {
  it("Should adopt the newest running terminal no other window shows", async () => {
    const actions = stubWindowActions();
    const onSelectTerminal = vi.fn();
    renderWindow({
      actions,
      onSelectTerminal,
      requestedTerminalId: null,
      terminals: TERMINAL_FIXTURES,
      windowedTerminalIds: new Set([DEV_SERVER_TERMINAL.id]),
    });

    // PSQL is the latest running fixture without a window; SSH has exited.
    await waitFor(() => expect(onSelectTerminal).toHaveBeenCalledExactlyOnceWith(PSQL_TERMINAL.id));
    expect(actions.onOpenTerminal).not.toHaveBeenCalled();
  });

  it("Should open a fresh terminal when every running one already has a window", async () => {
    const actions = stubWindowActions();
    const onSelectTerminal = vi.fn();
    const { rerender, actions: renderedActions } = renderWindow({
      actions,
      onSelectTerminal,
      requestedTerminalId: null,
      terminals: [DEV_SERVER_TERMINAL, SSH_STAGING_TERMINAL],
      windowedTerminalIds: new Set([DEV_SERVER_TERMINAL.id]),
    });

    await waitFor(() => expect(actions.onOpenTerminal).toHaveBeenCalledOnce());
    expect(onSelectTerminal).not.toHaveBeenCalled();

    // A re-render on the same arrival must not open a second terminal.
    rerender(
      <TerminalWindowApp
        actions={renderedActions}
        engineLoader={stubEngineLoader}
        inputRequests={[]}
        interactiveAvailable
        journal={<div data-testid="journal-slot">journal</div>}
        limit={TERMINAL_LIMIT}
        onSelectTerminal={onSelectTerminal}
        profile={TERMINAL_FIXTURE_PROFILE}
        requestedTerminalId={null}
        socketFactory={silentSocketFactory}
        terminals={[DEV_SERVER_TERMINAL, SSH_STAGING_TERMINAL]}
        viewerId={TERMINAL_FIXTURE_VIEWER}
        windowedTerminalIds={new Set([DEV_SERVER_TERMINAL.id])}
        workspaceId="ws-atlas"
      />
    );
    expect(actions.onOpenTerminal).toHaveBeenCalledOnce();
  });

  it("Should wait for a fresh catalog before resolving the id-less route", async () => {
    const actions = stubWindowActions();
    const onSelectTerminal = vi.fn();
    const { rerender, actions: renderedActions } = renderWindow({
      actions,
      onSelectTerminal,
      requestedTerminalId: null,
      resolveReady: false,
      terminals: [],
    });

    // A cached, pre-mount catalog must not trigger a duplicate create.
    await new Promise(resolve => setTimeout(resolve, 5));
    expect(actions.onOpenTerminal).not.toHaveBeenCalled();

    rerender(
      <TerminalWindowApp
        actions={renderedActions}
        engineLoader={stubEngineLoader}
        inputRequests={[]}
        interactiveAvailable
        journal={<div data-testid="journal-slot">journal</div>}
        limit={TERMINAL_LIMIT}
        onSelectTerminal={onSelectTerminal}
        profile={TERMINAL_FIXTURE_PROFILE}
        requestedTerminalId={null}
        resolveReady
        socketFactory={silentSocketFactory}
        terminals={[PSQL_TERMINAL]}
        viewerId={TERMINAL_FIXTURE_VIEWER}
        workspaceId="ws-atlas"
      />
    );
    // Once the catalog is fresh, the running terminal is adopted, not duplicated.
    await waitFor(() => expect(onSelectTerminal).toHaveBeenCalledExactlyOnceWith(PSQL_TERMINAL.id));
    expect(actions.onOpenTerminal).not.toHaveBeenCalled();
  });

  it("Should create instead of adopting when the arrival asked for a fresh terminal", async () => {
    const actions = stubWindowActions();
    const onSelectTerminal = vi.fn();
    renderWindow({
      actions,
      createIntent: true,
      onSelectTerminal,
      requestedTerminalId: null,
      terminals: TERMINAL_FIXTURES,
      windowedTerminalIds: new Set<string>(),
    });

    await waitFor(() => expect(actions.onOpenTerminal).toHaveBeenCalledOnce());
    expect(onSelectTerminal).not.toHaveBeenCalled();
  });

  it("Should surface the cap instead of creating when the id-less route lands full", async () => {
    const actions = stubWindowActions();
    renderWindow({
      actions,
      requestedTerminalId: null,
      terminals: TERMINAL_FIXTURES_AT_CAP,
      windowedTerminalIds: new Set(TERMINAL_FIXTURES_AT_CAP.map(terminal => terminal.id)),
    });

    expect(await screen.findByTestId("terminal-limit-dialog")).toBeInTheDocument();
    expect(actions.onOpenTerminal).not.toHaveBeenCalled();
  });

  it("Should neither adopt nor create on an aggregate profile read", async () => {
    const actions = stubWindowActions();
    const onSelectTerminal = vi.fn();
    renderWindow({
      actions,
      onSelectTerminal,
      readOnly: true,
      requestedTerminalId: null,
      terminals: [],
    });

    expect(screen.getByTestId("terminal-empty")).toBeInTheDocument();
    expect(onSelectTerminal).not.toHaveBeenCalled();
    expect(actions.onOpenTerminal).not.toHaveBeenCalled();
  });

  it("Should close the terminal from the head overflow, disabled while pending", async () => {
    const actions = stubWindowActions();
    renderWindow({ actions, terminals: [DEV_SERVER_TERMINAL] });

    await userEvent.click(screen.getByTestId("terminal-overflow"));
    await userEvent.click(await screen.findByTestId("terminal-close"));
    expect(actions.onCloseTerminal).toHaveBeenCalledExactlyOnceWith(DEV_SERVER_TERMINAL.id);

    const pending = stubWindowActions({ closePending: true });
    renderWindow({ actions: pending, terminals: [DEV_SERVER_TERMINAL] });
    const overflows = screen.getAllByTestId("terminal-overflow");
    await userEvent.click(overflows[overflows.length - 1]);
    expect(await screen.findByTestId("terminal-close")).toHaveAttribute("aria-disabled", "true");
  });
});

import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import type { TerminalSocket } from "../../adapters/terminal-socket";
import type { TerminalInfo } from "../../types";
import {
  exitedTerminal,
  DEV_SERVER_TERMINAL,
  MAKE_GATE_TERMINAL,
  PASSWORD_REQUEST,
  PSQL_TERMINAL,
  SSH_STAGING_TERMINAL,
  TERMINAL_FIXTURES,
  TERMINAL_FIXTURES_AT_CAP,
  TERMINAL_FIXTURE_PROFILE,
  TERMINAL_FIXTURE_VIEWER,
} from "../../mocks/terminal-fixtures";
import { TerminalConnectingLine } from "../terminal-connecting-line";
import { TerminalExpiredState, TerminalNotFoundState } from "../terminal-empty-states";
import { TerminalHeader } from "../terminal-header";
import { TerminalExitBar, TerminalSizeVoteBar } from "../terminal-exit-bar";
import { TerminalStreamNotice } from "../terminal-notices";
import { TerminalWindowApp, type TerminalWindowAppProps } from "../terminal-window-app";
import {
  AGENT_SCREEN,
  DEV_SERVER_SCREEN,
  EXITED_SCREEN,
  WORKER_SCREEN,
  scriptedSocketFactory,
} from "./terminal-scripted-socket";
import { TerminalVisualStage } from "./terminal-visual-stage";

/**
 * Every S1 state the surface map enumerates, one story each.
 *
 * The socket never opens: these render the chrome and the states around the
 * grid, which is what the Visual Contract binds. Live byte behaviour is covered
 * by the protocol-client suite, not by a screenshot.
 */
const meta: Meta<typeof TerminalWindowApp> = {
  title: "systems/terminal/routes/Terminal",
  component: TerminalWindowApp,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** A socket that opens and then says nothing, so no live bytes arrive. */
const silentSocketFactory = (): TerminalSocket => ({
  close: () => undefined,
  send: () => undefined,
  onopen: null,
  onmessage: null,
  onclose: null,
  onerror: null,
});

const PTY_SCREEN = { cols: 96, rows: 28, mode: "pty" } as const;

// Factories are module-scoped: an inline factory changes identity every render
// and forces the attachment hook into a full teardown + re-attach per render.
const SHARED_SOCKET = scriptedSocketFactory({
  ...PTY_SCREEN,
  output: DEV_SERVER_SCREEN,
});
const JOURNAL_UNAVAILABLE_SOCKET = scriptedSocketFactory({
  ...PTY_SCREEN,
  output: DEV_SERVER_SCREEN,
  error: { code: "journal_unavailable", message: "journal_unavailable" },
});
const AGENT_SOCKET = scriptedSocketFactory({
  ...PTY_SCREEN,
  output: AGENT_SCREEN,
});
const EXITED_SOCKET = scriptedSocketFactory({
  ...PTY_SCREEN,
  output: EXITED_SCREEN,
});
const TINY_TILE_SOCKET = scriptedSocketFactory({
  cols: 30,
  rows: 5,
  mode: "pty",
  output: WORKER_SCREEN,
});

const ACTIONS: TerminalWindowAppProps["actions"] = {
  onOpenTerminal: fn(),
  onCloseTerminal: fn(),
  onStop: fn(),
  onWait: fn(),
  onStopRecording: fn(),
  onAnswerInputRequest: fn(),
  onRejectInputRequest: fn(),
  onSendSelection: fn(),
  onCopySelection: fn(),
  onChooseSession: fn(),
  onStartSession: fn(),
  hasActiveSession: true,
  onOpenSettings: fn(),
};

function windowProps(overrides: Partial<TerminalWindowAppProps> = {}): TerminalWindowAppProps {
  return {
    actions: ACTIONS,
    inputRequests: [],
    interactiveAvailable: true,
    journal: null,
    limit: 8,
    profile: TERMINAL_FIXTURE_PROFILE,
    socketFactory: silentSocketFactory,
    terminals: TERMINAL_FIXTURES,
    viewerId: TERMINAL_FIXTURE_VIEWER,
    workspaceId: "ws-atlas",
    ...overrides,
  };
}

function stagedWindow(overrides: Partial<TerminalWindowAppProps> = {}, width?: "tile") {
  return (
    <TerminalVisualStage width={width ?? "default"}>
      <TerminalWindowApp {...windowProps(overrides)} />
    </TerminalVisualStage>
  );
}

/** VC-01 — the operator and agents share one terminal. */
export const Shared: Story = {
  name: "VC-01 · Shared terminal",
  render: () =>
    stagedWindow({
      socketFactory: SHARED_SOCKET,
    }),
};

/** VC-02 — an agent and operator can work on the same screen. */
export const AgentActive: Story = {
  name: "VC-02 · Agent active",
  render: () =>
    stagedWindow({
      terminals: [PSQL_TERMINAL, DEV_SERVER_TERMINAL],
      socketFactory: AGENT_SOCKET,
    }),
};

/** VC-04 — a project with no terminals. */
export const Empty: Story = {
  name: "VC-04 · No terminals yet",
  render: () => stagedWindow({ terminals: [] }),
};

/** VC-05 — the per-project cap, reached. */
export const AtLimit: Story = {
  name: "VC-05 · At the project limit",
  render: () => stagedWindow({ terminals: TERMINAL_FIXTURES_AT_CAP }),
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByTestId("terminal-new"));
  },
  tags: ["play-fn"],
};

/** VC-06 — a remote environment that cannot host an interactive terminal. */
export const ExecuteOnly: Story = {
  name: "VC-06 · Execute-only remote environment",
  render: () => stagedWindow({ interactiveAvailable: false, terminals: [] }),
};

/**
 * VC-07a — how a run ended, in every outcome the daemon can report.
 *
 * Only zero earns colour. A non-zero code is information, not an emergency; a
 * signal names the signal; and a cause the daemon could not verify says so in
 * words rather than inventing a code. Each bar states how much longer the
 * screen stays readable, from `[terminal].exit_retention`.
 */
const EXIT_OUTCOMES: { terminal: TerminalInfo; retentionNote: string }[] = [
  {
    terminal: exitedTerminal({ cause: "exited", code: 0, at: "2026-08-25T12:52:00Z" }),
    retentionNote: "readable for 14 more minutes",
  },
  {
    terminal: exitedTerminal({ cause: "exited", code: 1, at: "2026-08-25T12:50:00Z" }),
    retentionNote: "readable for 12 more minutes",
  },
  {
    terminal: exitedTerminal({ cause: "signaled", signal: "TERM", at: "2026-08-25T12:47:00Z" }),
    retentionNote: "readable for 9 more minutes",
  },
  {
    terminal: exitedTerminal({ cause: "unknown", at: "2026-08-25T12:41:00Z" }),
    retentionNote: "readable for 3 more minutes",
  },
];

export const Exited: Story = {
  name: "VC-07a · How the run ended",
  render: () => (
    <TerminalVisualStage>
      <div className="flex flex-col">
        {EXIT_OUTCOMES.map(outcome => (
          <TerminalExitBar
            exit={{
              cause: outcome.terminal.exit?.cause ?? "unknown",
              code: outcome.terminal.exit?.code ?? null,
              signal: outcome.terminal.exit?.signal ?? null,
            }}
            key={outcome.terminal.id}
            retentionNote={outcome.retentionNote}
            terminal={outcome.terminal}
          />
        ))}
      </div>
    </TerminalVisualStage>
  ),
};

/** An exited terminal stays on screen, with its outcome pinned under the grid. */
export const ExitedWindow: Story = {
  name: "Exited and readable",
  render: () =>
    stagedWindow({
      exitRetentionMs: 15 * 60 * 1000,
      terminals: [SSH_STAGING_TERMINAL],
      socketFactory: EXITED_SOCKET,
    }),
};

/** VC-07b — reclaimed after idle; the journal outlives the terminal. */
export const Expired: Story = {
  name: "VC-07b · Cleaned up after idle",
  render: () => (
    <TerminalVisualStage>
      {/* The duration comes from `[terminal].detached_ttl`; this is the
          documented default, supplied here the way a caller would. */}
      <TerminalExpiredState idleFor="24 hours" onOpenTerminal={fn()} onViewJournal={fn()} />
    </TerminalVisualStage>
  ),
};

/**
 * VC-16 — the smallest sane tile.
 *
 * The daemon decides the size; the pane only proposes one. At the tile's floor
 * that lands at 30 columns, which is what this screen is written for — a line
 * that outruns the grid wraps, because a real emulator wraps. The hard clamp
 * (20–2000 × 5–1000) is a daemon rule, asserted in the protocol suite rather
 * than drawn here.
 */
export const TinyTile: Story = {
  name: "VC-16 · Minimum size",
  render: () =>
    stagedWindow(
      {
        terminals: [DEV_SERVER_TERMINAL],
        socketFactory: TINY_TILE_SOCKET,
      },
      "tile"
    ),
};

/** VC-21 — the journal cannot record; watching continues. */
export const AuditBlocked: Story = {
  name: "VC-21 · Journal unavailable",
  render: () =>
    stagedWindow({
      terminals: [DEV_SERVER_TERMINAL],
      socketFactory: JOURNAL_UNAVAILABLE_SOCKET,
    }),
  play: async ({ canvas }) => {
    await canvas.findByTestId("terminal-notice-journal_unavailable");
  },
  tags: ["play-fn"],
};

/**
 * What the connection is doing, before the screen is live.
 *
 * Three different facts, one line each: the pass is being minted, the
 * connection dropped and is coming back, or the screen is being rebuilt after
 * skipped output. None of them fills the grid with placeholder content —
 * waiting must never look like output.
 */
export const ConnectionStates: Story = {
  name: "Connecting, reconnecting, catching up",
  render: () => (
    <TerminalVisualStage>
      <div className="flex flex-col bg-terminal-bg">
        <TerminalConnectingLine status="connecting" />
        <TerminalConnectingLine status="reconnecting" />
        <TerminalConnectingLine status="resyncing" />
      </div>
    </TerminalVisualStage>
  ),
};

/** The pass is being minted; the grid stays empty rather than faking output. */
export const Connecting: Story = {
  name: "Connecting",
  render: () => stagedWindow({ terminals: [DEV_SERVER_TERMINAL] }),
};

/**
 * The size is not this window's to choose.
 *
 * Several viewers can share one terminal, so the quiet bar under the surface
 * states what size the daemon settled on.
 */
export const SettledSharedSize: Story = {
  name: "Shared terminal size",
  render: () => (
    <TerminalVisualStage>
      <TerminalHeader onStop={fn()} terminal={{ ...DEV_SERVER_TERMINAL, viewers: 3 }} />
      <div className="min-h-24 flex-1 bg-terminal-bg" />
      <TerminalSizeVoteBar cols={80} rows={24} />
    </TerminalVisualStage>
  ),
};

/**
 * Every refusal the stream can report, each with the remedy it actually has.
 *
 * A pass that expired can be retried; a terminal that no longer exists cannot,
 * so it points at the journal instead. Nothing here invents a third option.
 */
export const StreamRefusals: Story = {
  name: "Attach refused",
  render: () => (
    <TerminalVisualStage>
      <div className="flex flex-col gap-2 p-3">
        <TerminalStreamNotice
          code="ticket_expired"
          message="connection pass expired"
          onReconnect={fn()}
        />
        <TerminalStreamNotice
          code="ticket_invalid"
          message="connection pass already used"
          onReconnect={fn()}
        />
        <TerminalStreamNotice
          code="subscriber_limit_reached"
          message="16 viewers already attached"
        />
        <TerminalStreamNotice
          code="terminal_not_found"
          message="no terminal term-4f21c9a03b7e"
          onViewJournal={fn()}
        />
      </div>
    </TerminalVisualStage>
  ),
};

/** Gone after a runtime restart: no exit code, because none exists. */
export const NotFoundAfterRestart: Story = {
  name: "Gone after a restart",
  render: () => (
    <TerminalVisualStage>
      <TerminalNotFoundState onOpenTerminal={fn()} onViewJournal={fn()} />
    </TerminalVisualStage>
  ),
};

/** VC-17 — after a profile switch, the previous profile's terminals are gone. */
export const SwitchedProfile: Story = {
  name: "VC-17 · Switched profile (non-normative)",
  parameters: {
    docs: {
      description: {
        story:
          "Non-normative: the boards predate profile segmentation and contain no profile state, so there is nothing to compare against. The contract is `_uiux.md` US-033.AC-2 plus the `DESIGN.md` grammar. Switching profile hides the previous profile's terminals from this window's catalog — they are hidden, not closed, and switching back shows them still running.",
      },
    },
  },
  render: () => stagedWindow({ profile: "personal", terminals: [] }),
};

/** A read-only log born from a command. */
export const PipeMode: Story = {
  name: "Read-only log",
  render: () =>
    stagedWindow({
      terminals: [MAKE_GATE_TERMINAL],
      pipeOutput: {
        [MAKE_GATE_TERMINAL.id]: {
          firstLineNumber: 412,
          lines: [
            "ok   internal/store  4.021s",
            "ok   internal/osquery  1.116s",
            "gate: web lane → turbo --filter web…",
            "web:typecheck: cache hit, replaying logs",
            "web:test: 227 passed (14.2s)",
            "gate: recording evidence → .cache/gate/…",
          ],
        },
      },
    }),
};

/** A terminal waiting on an answer pins the question under its grid. */
export const AwaitingAnswer: Story = {
  name: "Question waiting",
  render: () => stagedWindow({ terminals: [PSQL_TERMINAL], inputRequests: [PASSWORD_REQUEST] }),
};

/** Recording announces itself in the head. */
export const Recording: Story = {
  name: "Recording",
  render: () =>
    stagedWindow({
      terminals: [PSQL_TERMINAL],
      recordings: { [PSQL_TERMINAL.id]: { elapsed: "02:14" } },
    }),
};

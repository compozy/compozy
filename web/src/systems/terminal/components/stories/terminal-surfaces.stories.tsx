import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { terminalLeaseView } from "../../lib/terminal-lease";
import { buildTerminalQuote } from "../../lib/terminal-quote";
import {
  CONFIRMATION_REQUEST,
  DEV_SERVER_TERMINAL,
  JOURNAL_FIXTURES,
  PASSWORD_REQUEST,
  PSQL_TERMINAL,
  RECORDING_FIXTURE,
  TERMINAL_FIXTURE_VIEWER,
  TERMINAL_GRANT_FIXTURES,
} from "../../mocks/terminal-fixtures";
import { SessionTerminalBlock } from "../session-terminal-block";
import { TerminalApprovalDetail } from "../terminal-approval-detail";
import { TerminalGrantRow } from "../terminal-grant-row";
import { TerminalInputRequestCard, TerminalInputResolvedRow } from "../terminal-input-request";
import { TerminalJournalPanel } from "../terminal-journal-panel";
import { TerminalGapSeam, TerminalStreamNotice } from "../terminal-notices";
import { TerminalQuoteBlock, TerminalSelectionActions } from "../terminal-quote-block";
import { TerminalRecordingPlayer } from "../terminal-recording-player";
import { TerminalVisualStage } from "./terminal-visual-stage";

/**
 * The surfaces around the grid: questions, approvals, the journal, replay, and
 * the session-side block.
 */
const meta: Meta = {
  title: "systems/terminal/components/TerminalSurfaces",
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const NOOP = fn();

/**
 * ANSI, built at runtime.
 *
 * Control bytes never live in source: a raw ESC or CR is invisible in review,
 * survives copy-paste as a corrupt byte, and defeats every text tool that reads
 * the file. They are spelled by code point here and assembled when the story
 * renders.
 */
const ESC = String.fromCharCode(27);
const CRLF = String.fromCharCode(13, 10);

/** Select Graphic Rendition — `sgr("1;32")` is bold green, `sgr("0")` resets. */
function sgr(parameters: string): string {
  return `${ESC}[${parameters}m`;
}

/**
 * One minute into the request's frozen fifteen-minute lifetime, so the capture
 * reads the same on any day.
 */
const ONE_MINUTE_IN = Date.parse(PASSWORD_REQUEST.requested_at) + 60_000;

/**
 * VC-08 — a redacted question, pinned to its terminal.
 *
 * `askedBy` is supplied here because the board names the agent. The runtime does
 * not: `TerminalInputRequest` carries no requester field, so the shipped window
 * omits the name rather than borrowing the current controller's — an authorized
 * runtime-truth delta until the daemon publishes who asked.
 */
export const InputRequestRedacted: Story = {
  name: "VC-08 · Redacted question",
  render: () => (
    <TerminalVisualStage>
      <TerminalInputRequestCard
        askedBy="Claude Code"
        canAnswerDirectly
        now={ONE_MINUTE_IN}
        onAnswer={NOOP}
        onReject={NOOP}
        request={PASSWORD_REQUEST}
      />
    </TerminalVisualStage>
  ),
};

/** A plain question keeps the same anatomy without the hidden field. */
export const InputRequestPlain: Story = {
  name: "Plain question",
  render: () => (
    <TerminalVisualStage>
      <TerminalInputRequestCard
        askedBy="Claude Code"
        canAnswerDirectly
        now={Date.parse(CONFIRMATION_REQUEST.requested_at) + 60_000}
        onAnswer={NOOP}
        onReject={NOOP}
        request={CONFIRMATION_REQUEST}
      />
    </TerminalVisualStage>
  ),
};

/** A watcher is offered one gesture that takes control and sends. */
export const InputRequestWatcher: Story = {
  name: "Watcher — one gesture",
  render: () => (
    <TerminalVisualStage>
      <TerminalInputRequestCard
        askedBy="Claude Code"
        canAnswerDirectly={false}
        now={ONE_MINUTE_IN}
        onAnswer={NOOP}
        onReject={NOOP}
        request={PASSWORD_REQUEST}
      />
    </TerminalVisualStage>
  ),
};

/** VC-09 — the four resolved outcomes, each with its own copy. */
export const InputRequestResolved: Story = {
  name: "VC-09 · Four quiet outcomes",
  render: () => (
    <TerminalVisualStage>
      <TerminalInputResolvedRow
        outcome="answered"
        redactedLength={10}
        resolvedAt="2026-08-25T12:44:00Z"
      />
      <TerminalInputResolvedRow outcome="rejected" resolvedAt="2026-08-25T12:41:00Z" />
      <TerminalInputResolvedRow
        outcome="superseded"
        resolvedAt="2026-08-25T12:39:00Z"
        supersededBy="Marina"
      />
      <TerminalInputResolvedRow outcome="expired" resolvedAt="2026-08-25T12:20:00Z" />
    </TerminalVisualStage>
  ),
};

/**
 * The exec detail on its own.
 *
 * The normative VC-10 row is captured from the shipped decision surface —
 * `systems/session/components/PermissionDock` → `TerminalExec` — because the
 * title and the decisions belong to the dock. This story isolates the part the
 * terminal system owns.
 */
export const ExecApproval: Story = {
  name: "Exec approval — detail only",
  render: () => (
    <TerminalVisualStage>
      <div className="px-3.5 py-3">
        <TerminalApprovalDetail
          detail={{
            kind: "exec",
            command: "bun add @xterm/xterm @xterm/addon-fit",
            cwd: "~/dev/atlas-api",
            terminalId: DEV_SERVER_TERMINAL.id,
            risk: "ordinary",
          }}
        />
      </div>
    </TerminalVisualStage>
  ),
};

/** The irreversible detail on its own; VC-11 is captured from the dock. */
export const IrreversibleApproval: Story = {
  name: "Irreversible command — detail only",
  render: () => (
    <TerminalVisualStage>
      <div className="px-3.5 py-3">
        <TerminalApprovalDetail
          detail={{
            kind: "exec",
            command: "rm -rf /var/lib/atlas/journal-backups",
            cwd: "~/dev/atlas-api",
            terminalId: DEV_SERVER_TERMINAL.id,
            risk: "irreversible",
          }}
        />
      </div>
    </TerminalVisualStage>
  ),
};

/** The typing detail on its own; VC-12a is captured from the dock. */
export const TypingGrant: Story = {
  name: "Typing permission — detail only",
  render: () => (
    <TerminalVisualStage>
      <div className="px-3.5 py-3">
        <TerminalApprovalDetail
          detail={{ kind: "typing", terminalId: PSQL_TERMINAL.id }}
          terminalTitle={PSQL_TERMINAL.title}
        />
      </div>
    </TerminalVisualStage>
  ),
};

/**
 * VC-12b — what those permissions look like once remembered.
 *
 * The board labels these rows "Can type in psql" and "Always allowed: bun add
 * …". The daemon cannot support either: a remembered decision stores a **digest**
 * of the exact tool input, never the input, the terminal id or the command. So
 * the rows say what the decision covers and show the digest as the only thing
 * that tells two of them apart. Authorized runtime-truth delta; a readable name
 * would need the daemon to publish one (handed off to the activation tranche).
 */
export const GrantRows: Story = {
  name: "VC-12b · Remembered permissions",
  render: () => (
    <TerminalVisualStage>
      {TERMINAL_GRANT_FIXTURES.map(grant => (
        <TerminalGrantRow grant={grant} key={grant.id} onRevoke={NOOP} />
      ))}
    </TerminalVisualStage>
  ),
};

const JOURNAL_ACTIONS = {
  onClearFilter: NOOP,
  onClearFilters: NOOP,
  onLoadMore: NOOP,
  onOpenFilters: NOOP,
  onOpenNewTerminal: NOOP,
  onOpenTerminal: NOOP,
  onReplay: NOOP,
};

const ALL_PROFILE_JOURNAL_FIXTURES = JOURNAL_FIXTURES.map((entry, index) => ({
  ...entry,
  profile_name: ["work", "staging", "release"][index % 3],
}));

/** VC-13 — the populated journal, with mixed detection confidence. */
export const Journal: Story = {
  name: "VC-13 · Journal",
  render: () => (
    <TerminalVisualStage width="wide">
      <TerminalJournalPanel
        {...JOURNAL_ACTIONS}
        entries={JOURNAL_FIXTURES}
        filters={[{ key: "actor", label: "who", value: "Claude Code" }]}
        hasMore
      />
    </TerminalVisualStage>
  ),
};

/** VC-14a — nothing has run, and the way out is a terminal. */
export const JournalEmpty: Story = {
  name: "VC-14a · Empty journal",
  render: () => (
    <TerminalVisualStage width="wide">
      <TerminalJournalPanel {...JOURNAL_ACTIONS} entries={[]} filters={[]} hasMore={false} />
    </TerminalVisualStage>
  ),
};

/** VC-14b — nothing matched, which is a different fact and a different offer. */
export const JournalFilteredEmpty: Story = {
  name: "VC-14b · Filtered to nothing",
  render: () => (
    <TerminalVisualStage width="wide">
      <TerminalJournalPanel
        {...JOURNAL_ACTIONS}
        entries={[]}
        filters={[
          { key: "failed", label: "result", value: "errors" },
          { key: "actor", label: "who", value: "Marina" },
        ]}
        hasMore
      />
    </TerminalVisualStage>
  ),
};

/** VC-18 — the read-only all-profiles view labels every row's owner. */
export const JournalAllProfiles: Story = {
  name: "VC-18 · All profiles (non-normative)",
  parameters: {
    docs: {
      description: {
        story:
          "Non-normative: the boards predate profile segmentation and contain no profile state, so this has no reference to compare against. The contract is `_uiux.md` US-033.AC-3/AC-4 plus the `DESIGN.md` grammar. Every row is labelled with its owning profile, and the view offers no mutating action.",
      },
    },
  },
  render: () => (
    <TerminalVisualStage width="wide">
      <TerminalJournalPanel
        {...JOURNAL_ACTIONS}
        entries={ALL_PROFILE_JOURNAL_FIXTURES}
        filters={[]}
        hasMore
        showOwner
      />
    </TerminalVisualStage>
  ),
};

/** VC-20 — output was skipped, and a viewer that kept falling behind. */
export const SkippedContent: Story = {
  name: "VC-20 · Skipped content and falling behind",
  render: () => (
    <TerminalVisualStage>
      <div className="flex flex-col gap-3 bg-terminal-bg px-3.5 py-3 font-mono text-[12.5px] leading-[1.5] text-terminal-fg">
        <span className="block">
          <span className="text-terminal-ansi-8">12:44:01</span> GET /api/journal 200 4.1ms
        </span>
        <TerminalGapSeam gap={{ droppedBytes: 49_152, fromSeq: 100, toSeq: 49_252 }} />
        <span className="block">
          <span className="text-terminal-ansi-8">12:44:09</span> GET /api/terminals 200 1.9ms
        </span>
      </div>
      <TerminalStreamNotice
        code="slow_consumer"
        message="viewer queue was full for 10s"
        onReconnect={NOOP}
      />
    </TerminalVisualStage>
  ),
};

/**
 * VC-22 — a recording, mid-replay.
 *
 * The clock is frozen by handing the player a scheduler that never fires, then
 * seeking: the replay is genuinely playing and genuinely at 0:51, so the
 * capture reads the same on every run instead of racing the wall clock.
 */
const FROZEN_CLOCK = () => () => undefined;

export const RecordingReplay: Story = {
  name: "VC-22 · Replay",
  render: () => (
    <TerminalVisualStage>
      <div className="flex h-[360px] flex-col">
        <TerminalRecordingPlayer
          autoPlay
          onOpenJournal={NOOP}
          recordedAtLabel="12:47"
          recordingId="rec-9f21ac"
          retentionNote="kept for 30 days"
          schedule={FROZEN_CLOCK}
          source={RECORDING_FIXTURE}
          title="make gate"
        />
      </div>
    </TerminalVisualStage>
  ),
  play: async ({ canvas }) => {
    const scrubber = await canvas.findByTestId("terminal-recording-scrubber");
    const setValue = Object.getOwnPropertyDescriptor(
      window.HTMLInputElement.prototype,
      "value"
    )?.set;
    setValue?.call(scrubber, "51000");
    scrubber.dispatchEvent(new Event("input", { bubbles: true }));
    scrubber.dispatchEvent(new Event("change", { bubbles: true }));
  },
  tags: ["play-fn"],
};

/** The screen the board paints in its session block, in real ANSI. */
const SESSION_BLOCK_PREVIEW = [
  "$ bun run dev",
  `  ${sgr("1;32")}VITE v6.0.3${sgr("0")}  ready in 412 ms`,
  "",
  `  ${sgr("32")}➜${sgr("0")}  Local:   ${sgr("36")}http://localhost:5173/${sgr("0")}`,
  `${sgr("2")}12:41:03${sgr("0")} [${sgr("36")}vite${sgr("0")}] hmr update ${sgr("34")}/src/systems/terminal/terminal-pane.tsx${sgr("0")}`,
  "",
].join(CRLF);

/**
 * VC-15 — deliberate terminal use, inside the conversation.
 *
 * The lease chip and the jump action are shown here because the board shows
 * them, and the block renders both when its caller knows them. The session's
 * tool renderer does not yet: the tool envelope carries no lease and no start
 * time, and the Terminal app is registered by the activation tranche, so
 * production omits them rather than guessing. Authorized runtime-truth delta.
 */
export const SessionBlock: Story = {
  name: "VC-15 · Session terminal block",
  render: () => (
    <TerminalVisualStage width="block">
      <div className="p-4">
        <SessionTerminalBlock
          blockId="tool-call-7f21"
          lease={terminalLeaseView({
            lease: PSQL_TERMINAL.lease,
            controller: PSQL_TERMINAL.controller,
            viewerId: TERMINAL_FIXTURE_VIEWER,
            mode: PSQL_TERMINAL.mode,
            capabilities: PSQL_TERMINAL.capabilities,
          })}
          onOpenTerminal={NOOP}
          preview={SESSION_BLOCK_PREVIEW}
          terminalId={DEV_SERVER_TERMINAL.id}
          title="dev server"
        />
      </div>
    </TerminalVisualStage>
  ),
};

/** The quote a selection becomes, waiting in the composer. */
export const QuoteBlock: Story = {
  name: "Quote in the composer",
  render: () => (
    <TerminalVisualStage width="block">
      <div className="p-4">
        <TerminalQuoteBlock
          onRemove={NOOP}
          quote={buildTerminalQuote({
            terminalId: DEV_SERVER_TERMINAL.id,
            fromLine: 214,
            lines: [
              "12:41:04 [vite] Internal server error: Failed to resolve import",
              '"@compozy/ui/terminal-view" from "src/systems/terminal/terminal-pane.tsx"',
            ],
          })}
        />
      </div>
    </TerminalVisualStage>
  ),
};

/** With no session, the gesture offers a way in rather than dead-ending. */
export const SelectionWithoutSession: Story = {
  name: "Selection — no active session",
  render: () => (
    <TerminalVisualStage width="block">
      <div className="p-4">
        <TerminalSelectionActions
          hasActiveSession={false}
          onChooseSession={NOOP}
          onCopy={NOOP}
          onSendToConversation={NOOP}
          onStartSession={NOOP}
        />
      </div>
    </TerminalVisualStage>
  ),
};

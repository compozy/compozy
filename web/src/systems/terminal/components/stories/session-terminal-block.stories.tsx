import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { terminalLeaseView } from "../../lib/terminal-lease";
import {
  DEV_SERVER_TERMINAL,
  MAKE_GATE_TERMINAL,
  PSQL_TERMINAL,
  TERMINAL_FIXTURE_VIEWER,
} from "../../mocks/terminal-fixtures";
import { SessionTerminalBlock } from "../session-terminal-block";
import { TerminalVisualStage } from "./terminal-visual-stage";

const meta: Meta<typeof SessionTerminalBlock> = {
  title: "systems/terminal/components/SessionTerminalBlock",
  component: SessionTerminalBlock,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

const NOOP = fn();
const ESC = String.fromCharCode(27);
const CRLF = String.fromCharCode(13, 10);

function sgr(code: string): string {
  return `${ESC}[${code}m`;
}

const DEV_SERVER_PREVIEW = [
  "$ bun run dev",
  `  ${sgr("1;32")}VITE v6.0.3${sgr("0")}  ready in 412 ms`,
  "",
  `  ${sgr("32")}➜${sgr("0")}  Local:   ${sgr("36")}http://localhost:5173/${sgr("0")}`,
  `${sgr("2")}12:41:03${sgr("0")} [${sgr("36")}vite${sgr("0")}] hmr update ${sgr("34")}/src/systems/terminal/terminal-pane.tsx${sgr("0")}`,
  "",
].join(CRLF);

const GATE_PREVIEW = [
  `web:test: ${sgr("32")}227 passed${sgr("0")} (14.2s)`,
  "go: ok  internal/terminal  2.148s",
  `${sgr("1;32")}gate: PASS${sgr("0")} · evidence .cache/gate/7f31c0`,
].join(CRLF);

const HANDOFF_PREVIEW = [
  "Running 42 visual contracts…",
  `${sgr("32")}✓${sgr("0")} VC-01 app-window (1.8s)`,
  `${sgr("32")}✓${sgr("0")} VC-02 states (2.1s)`,
].join(CRLF);

/**
 * VC-15 — deliberate terminal use, inside the conversation.
 *
 * The lease chip, Open action, and start clock appear when the catalog already
 * has them. The tool envelope itself does not carry those fields, so production
 * omits them when the catalog row is missing rather than inventing them.
 */
export const SessionBlock: Story = {
  name: "VC-15 · Session terminal block",
  args: {},
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
          preview={DEV_SERVER_PREVIEW}
          startedLabel="12:40"
          terminalId={DEV_SERVER_TERMINAL.id}
          title="dev server"
        />
      </div>
    </TerminalVisualStage>
  ),
};

export const Finished: Story = {
  name: "VC-15b · Finished terminal block",
  args: {},
  render: () => (
    <TerminalVisualStage width="block">
      <div className="p-4">
        <SessionTerminalBlock
          blockId="tool-call-gate"
          durationLabel="2m 41s · finished 12:47"
          exit={{ at: "2026-08-25T12:47:00Z", cause: "exited", code: 0 }}
          onOpenTerminal={NOOP}
          preview={GATE_PREVIEW}
          terminalId={MAKE_GATE_TERMINAL.id}
          title="make gate"
        />
      </div>
    </TerminalVisualStage>
  ),
};

export const StillRunningHandoff: Story = {
  name: "VC-15c · Still-running handoff",
  args: {},
  render: () => (
    <TerminalVisualStage width="block">
      <div className="p-4">
        <SessionTerminalBlock
          blockId="tool-call-e2e"
          onOpenTerminal={NOOP}
          preview={HANDOFF_PREVIEW}
          stillRunning
          terminalId="term-77c1d0e94ab3"
          title="e2e suite"
        />
      </div>
    </TerminalVisualStage>
  ),
};

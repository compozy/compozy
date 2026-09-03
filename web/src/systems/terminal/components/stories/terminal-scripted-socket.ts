import type { TerminalSocket, TerminalSocketFactory } from "../../adapters/terminal-socket";
import {
  encodeTerminalServerControlFrame,
  encodeTerminalServerOutputFrame,
  TERMINAL_SERVER_OP,
} from "../../lib/terminal-wire";
import type { TerminalLeaseState, TerminalMode } from "../../types";

/**
 * A socket that plays a fixed script.
 *
 * Stories drive the real protocol client and the real emulator: frames are
 * encoded exactly as the daemon encodes them, so what a capture shows is what
 * the code actually paints, not a hand-drawn approximation of it.
 */
export interface ScriptedTerminalScreen {
  cols: number;
  rows: number;
  lease: TerminalLeaseState;
  mode: TerminalMode;
  /** Written as one OUTPUT frame from sequence zero. */
  output: string;
  /** A stream refusal after the screen is live, as the daemon encodes it. */
  error?: { code: string; message: string };
}

export function scriptedSocketFactory(screen: ScriptedTerminalScreen): TerminalSocketFactory {
  return () => {
    const socket: TerminalSocket = {
      close: () => undefined,
      send: () => undefined,
      onopen: null,
      onmessage: null,
      onclose: null,
      onerror: null,
    };
    // The client attaches on the next task, so the script is delivered after
    // its handlers are in place.
    setTimeout(() => {
      socket.onopen?.(new Event("open"));
      socket.onmessage?.({
        data: encodeTerminalServerControlFrame(TERMINAL_SERVER_OP.attached, {
          seq: "0",
          truncated: false,
          cols: screen.cols,
          rows: screen.rows,
          lease: screen.lease,
          mode: screen.mode,
        }),
      } as MessageEvent<unknown>);
      socket.onmessage?.({
        data: encodeTerminalServerOutputFrame(0n, screen.output),
      } as MessageEvent<unknown>);
      if (screen.error) {
        socket.onmessage?.({
          data: encodeTerminalServerControlFrame(TERMINAL_SERVER_OP.error, {
            error: screen.error,
          }),
        } as MessageEvent<unknown>);
      }
    }, 0);
    return socket;
  };
}

const ESC = String.fromCharCode(27);

function sgr(code: string, text: string): string {
  return `${ESC}[${code}m${text}${ESC}[0m`;
}

/** The board's `dev server` screen, as bytes. */
export const DEV_SERVER_SCREEN = [
  `${sgr("32", "$")} bun run dev\r\n`,
  `${sgr("1;32", "  VITE v6.0.3")}  ready in ${sgr("1", "412 ms")}\r\n\r\n`,
  `  ${sgr("32", "➜")}  ${sgr("1", "Local:")}   ${sgr("36", "http://localhost:5173/")}\r\n`,
  `  ${sgr("32", "➜")}  ${sgr("1", "Network:")} ${sgr("36", "http://192.168.0.14:5173/")}\r\n\r\n`,
  `${sgr("90", "12:41:03")} ${sgr("36", "[vite]")} hmr update ${sgr("34", "/src/systems/terminal/terminal-pane.tsx")}\r\n`,
  `${sgr("90", "12:41:09")} ${sgr("36", "[vite]")} page reload ${sgr("34", "src/routes/_app/terminal/index.tsx")}\r\n`,
  `${sgr("32", "$")} `,
].join("");

/** The board's watched screen, mid-gate. */
export const AGENT_SCREEN = [
  `${sgr("32", "$")} git rebase --continue\r\n`,
  `${sgr("90", "Executing: git rebase --continue")}\r\n`,
  `Successfully rebased and updated refs/heads/${sgr("36", "terminal-app")}.\r\n`,
  `${sgr("32", "$")} make gate\r\n`,
  `${sgr("90", "gate: classifying diff vs merge-base…")}\r\n`,
  `gate: go lane → ${sgr("33", "go-lint")} + ${sgr("33", "go test -race")} (scoped)\r\n`,
  `ok   internal/terminal  2.148s\r\n`,
  `ok   internal/terminal/journal  0.912s\r\n`,
  `${sgr("32", "$")} `,
].join("");

/** A small worker log, for the minimum-size tile. */
export const WORKER_SCREEN = [
  `${sgr("90", "12:58:11")} job 4812 done\r\n`,
  `${sgr("90", "12:58:14")} job 4813 done\r\n`,
  `${sgr("32", "$")} `,
].join("");

/** The exited `ssh staging` screen, still readable. */
export const EXITED_SCREEN = [
  `${sgr("32", "$")} ssh staging\r\n`,
  `Last login: Mon Aug 25 12:31:02 2026\r\n`,
  `staging % exit\r\n`,
  `logout\r\n`,
].join("");

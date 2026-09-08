/**
 * Reading one control frame from the terminal stream.
 *
 * Every frame is parsed against its schema before anything acts on it: a frame
 * this client cannot read is reported as unreadable rather than merged
 * half-understood into what a person is looking at. Keeping the parsing here
 * leaves the client with the decisions and none of the decoding.
 */

import {
  terminalAttachedFrameSchema,
  terminalErrorFrameSchema,
  terminalExitFrameSchema,
  terminalGapFrameSchema,
  terminalPresenceFrameSchema,
  terminalRedactedInputFrameSchema,
  terminalResizedFrameSchema,
  terminalTitleFrameSchema,
  type TerminalAttachedFrame,
  type TerminalErrorFrame,
  type TerminalExitFrame,
  type TerminalGapFrame,
  type TerminalPresenceFrame,
  type TerminalRedactedInputFrame,
  type TerminalResizedFrame,
} from "./terminal-wire-schema";
import { TERMINAL_SERVER_OP } from "./terminal-wire";

/** What the client does with each kind of control frame. */
export interface TerminalControlFrameHandlers {
  onAttached: (frame: TerminalAttachedFrame) => void;
  onPresence: (frame: TerminalPresenceFrame) => void;
  onRedactedInput: (frame: TerminalRedactedInputFrame) => void;
  onTitle: (title: string) => void;
  onResized: (frame: TerminalResizedFrame) => void;
  onGap: (frame: TerminalGapFrame) => void;
  onExit: (frame: TerminalExitFrame) => void;
  onError: (frame: TerminalErrorFrame) => void;
}

/** Parses one control frame and routes it. Unknown opcodes are ignored. */
export function dispatchTerminalControlFrame(
  op: number,
  payload: unknown,
  handlers: TerminalControlFrameHandlers
): void {
  switch (op) {
    case TERMINAL_SERVER_OP.attached:
      handlers.onAttached(terminalAttachedFrameSchema.parse(payload));
      return;
    case TERMINAL_SERVER_OP.presence:
      handlers.onPresence(terminalPresenceFrameSchema.parse(payload));
      return;
    case TERMINAL_SERVER_OP.redactedInput:
      handlers.onRedactedInput(terminalRedactedInputFrameSchema.parse(payload));
      return;
    case TERMINAL_SERVER_OP.title:
      handlers.onTitle(terminalTitleFrameSchema.parse(payload).title);
      return;
    case TERMINAL_SERVER_OP.resized:
      handlers.onResized(terminalResizedFrameSchema.parse(payload));
      return;
    case TERMINAL_SERVER_OP.gap:
      handlers.onGap(terminalGapFrameSchema.parse(payload));
      return;
    case TERMINAL_SERVER_OP.exit:
      handlers.onExit(terminalExitFrameSchema.parse(payload));
      return;
    case TERMINAL_SERVER_OP.error:
      handlers.onError(terminalErrorFrameSchema.parse(payload));
      return;
    default:
      return;
  }
}

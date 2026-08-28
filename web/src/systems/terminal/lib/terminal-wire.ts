import {
  TERMINAL_CLIENT_OP,
  TERMINAL_MAX_COLS,
  TERMINAL_MAX_INPUT_BYTES,
  TERMINAL_MAX_ROWS,
  TERMINAL_MIN_COLS,
  TERMINAL_MIN_ROWS,
  TERMINAL_SERVER_OP,
  TERMINAL_SUBPROTOCOL,
} from "@/generated/terminal-wire";

export {
  TERMINAL_CLIENT_OP,
  TERMINAL_MAX_COLS,
  TERMINAL_MAX_INPUT_BYTES,
  TERMINAL_MAX_ROWS,
  TERMINAL_MIN_COLS,
  TERMINAL_MIN_ROWS,
  TERMINAL_SERVER_OP,
  TERMINAL_SUBPROTOCOL,
};

export type TerminalServerOpcode = (typeof TERMINAL_SERVER_OP)[keyof typeof TERMINAL_SERVER_OP];
export type TerminalSequence = bigint;

export type TerminalControlOpcode = Exclude<TerminalServerOpcode, typeof TERMINAL_SERVER_OP.output>;

/** Frames are sent as binary; the socket only accepts owned buffers. */
export type TerminalFrameBytes = Uint8Array<ArrayBuffer>;

/** An OUTPUT frame carries raw bytes behind an absolute sequence number. */
export interface TerminalOutputFrame {
  op: typeof TERMINAL_SERVER_OP.output;
  seq: TerminalSequence;
  bytes: Uint8Array;
}

/** Every other server frame carries JSON. */
export interface TerminalControlFrame {
  op: TerminalControlOpcode;
  payload: unknown;
}

export type TerminalServerFrame = TerminalOutputFrame | TerminalControlFrame;

export class TerminalFrameError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "TerminalFrameError";
  }
}

const decoder = new TextDecoder();
const encoder = new TextEncoder();

/** Decodes one server frame. Throws rather than guessing at a malformed one. */
export function decodeTerminalServerFrame(raw: ArrayBuffer | Uint8Array): TerminalServerFrame {
  const bytes = raw instanceof Uint8Array ? raw : new Uint8Array(raw);
  if (bytes.byteLength === 0) {
    throw new TerminalFrameError("empty server frame");
  }
  const op = bytes[0];
  if (op === TERMINAL_SERVER_OP.output) {
    if (bytes.byteLength < 9) {
      throw new TerminalFrameError("short OUTPUT frame");
    }
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    const seq = view.getBigUint64(1, false);
    return { op, seq, bytes: bytes.subarray(9) };
  }
  if (!isControlOpcode(op)) {
    throw new TerminalFrameError(`unknown server opcode 0x${op.toString(16).padStart(2, "0")}`);
  }
  return { op, payload: parseControlPayload(bytes.subarray(1)) };
}

function parseControlPayload(bytes: Uint8Array): unknown {
  try {
    return JSON.parse(decoder.decode(bytes)) as unknown;
  } catch {
    throw new TerminalFrameError("server control payload is not JSON");
  }
}

function isControlOpcode(op: number): op is TerminalControlOpcode {
  return op > TERMINAL_SERVER_OP.output && op <= TERMINAL_SERVER_OP.redactedInput;
}

/**
 * Splits input into frames the daemon will accept, without breaking a character.
 *
 * A paste is one `onData` call of any size, and the daemon refuses anything
 * over 64 KiB — so the client frames it rather than dropping it. The split
 * happens on encoded bytes, then walks back off any UTF-8 continuation byte:
 * cutting mid-sequence would put half a character on the wire and the other
 * half in the next frame, which decodes to a different string than the person
 * pasted.
 */
export function encodeTerminalInputFrames(data: string): TerminalFrameBytes[] {
  const payload = encoder.encode(data);
  if (payload.byteLength === 0) return [];
  const frames: TerminalFrameBytes[] = [];
  let start = 0;
  while (start < payload.byteLength) {
    let end = Math.min(start + TERMINAL_MAX_INPUT_BYTES, payload.byteLength);
    if (end < payload.byteLength) {
      // 0b10xxxxxx is a continuation byte; back up to the sequence's first byte.
      while (end > start && (payload[end] & 0b1100_0000) === 0b1000_0000) end -= 1;
      if (end === start) end = Math.min(start + TERMINAL_MAX_INPUT_BYTES, payload.byteLength);
    }
    frames.push(prefix(TERMINAL_CLIENT_OP.input, payload.subarray(start, end)));
    start = end;
  }
  return frames;
}

/** Flow-control credit, in bytes the client has finished parsing. */
export function encodeTerminalAck(bytes: number): TerminalFrameBytes {
  const payload = new Uint8Array(4);
  new DataView(payload.buffer).setUint32(0, bytes, false);
  return prefix(TERMINAL_CLIENT_OP.ack, payload);
}

/** A vote, not a command: the daemon decides the authoritative size. */
export function encodeTerminalResize(cols: number, rows: number): TerminalFrameBytes {
  return prefix(TERMINAL_CLIENT_OP.resize, encoder.encode(JSON.stringify({ cols, rows })));
}

export function encodeTerminalSignal(signal: string): TerminalFrameBytes {
  return prefix(TERMINAL_CLIENT_OP.signal, encoder.encode(JSON.stringify({ signal })));
}

export function encodeTerminalTakeover(force: boolean): TerminalFrameBytes {
  return prefix(TERMINAL_CLIENT_OP.takeover, encoder.encode(JSON.stringify({ force })));
}

export function encodeTerminalDetach(): TerminalFrameBytes {
  return prefix(TERMINAL_CLIENT_OP.detach, encoder.encode("{}"));
}

export function encodeTerminalRelease(): TerminalFrameBytes {
  return prefix(TERMINAL_CLIENT_OP.release, encoder.encode("{}"));
}

/** Builds one daemon control frame for contract fixtures and scripted transports. */
export function encodeTerminalServerControlFrame(
  op: TerminalControlOpcode,
  payload: unknown
): TerminalFrameBytes {
  return prefix(op, encoder.encode(JSON.stringify(payload)));
}

/** Builds one daemon OUTPUT frame with its absolute sequence position. */
export function encodeTerminalServerOutputFrame(
  seq: TerminalSequence,
  text: string
): TerminalFrameBytes {
  if (seq < 0n || seq > 0xffff_ffff_ffff_ffffn) {
    throw new TerminalFrameError("server output sequence must fit an unsigned 64-bit integer");
  }
  const body = encoder.encode(text);
  const frame = new Uint8Array(body.byteLength + 9);
  frame[0] = TERMINAL_SERVER_OP.output;
  new DataView(frame.buffer).setBigUint64(1, seq, false);
  frame.set(body, 9);
  return frame;
}

/** Renders trusted REDACTED_INPUT metadata. PTY output never calls this boundary. */
export function renderTerminalRedactedInput(characters: number): string {
  return `hidden input · ${characters} characters`;
}

function prefix(op: number, payload: Uint8Array): TerminalFrameBytes {
  const frame = new Uint8Array(payload.byteLength + 1);
  frame[0] = op;
  frame.set(payload, 1);
  return frame;
}

/** Keeps a proposal inside the range the daemon would clamp it to anyway. */
export function clampTerminalDimensions(
  cols: number,
  rows: number
): { cols: number; rows: number } | null {
  // A non-finite size is not a size. Clamping one produces `NaN`, which
  // `JSON.stringify` turns into `null` — a resize frame the daemon would read as
  // a missing field rather than as the nonsense it is.
  if (!Number.isFinite(cols) || !Number.isFinite(rows)) return null;
  if (cols <= 0 || rows <= 0) return null;
  return {
    cols: Math.min(Math.max(Math.trunc(cols), TERMINAL_MIN_COLS), TERMINAL_MAX_COLS),
    rows: Math.min(Math.max(Math.trunc(rows), TERMINAL_MIN_ROWS), TERMINAL_MAX_ROWS),
  };
}

/**
 * The terminal catalog's live half.
 *
 * Only attached terminals hold a WebSocket, so this stream is what keeps the
 * tab strip, the dock badge and the list honest without one. It is workspace-
 * and profile-scoped, and a profile switch rebinds it exactly as the desktop
 * stream rebinds its own key.
 */

import { z } from "zod";

import type { TerminalInfo } from "../types";
import {
  terminalActorKindSchema,
  terminalExitSchema,
  terminalInfoSchema,
  terminalLeaseStateSchema,
  terminalModeSchema,
} from "./terminal-contract-schema";

export const TERMINAL_CATALOG_EVENTS = [
  "terminal.snapshot",
  "terminal.created",
  "terminal.closed",
  "terminal.title_changed",
  "terminal.lease_changed",
  "terminal.mode_changed",
] as const;

export const TERMINAL_STREAM_EVENTS = [
  ...TERMINAL_CATALOG_EVENTS,
  "terminal.recording_started",
  "terminal.recording_stopped",
] as const;

export type TerminalCatalogEventName = (typeof TERMINAL_CATALOG_EVENTS)[number];

export class TerminalCatalogProtocolError extends Error {
  constructor() {
    super("The daemon returned an invalid terminal catalog event.");
    this.name = "TerminalCatalogProtocolError";
  }
}

/** One catalog stream is always owned by exactly one profile. */
export function terminalCatalogStreamPath(workspaceId: string, profile: string): string {
  const query = new URLSearchParams({ profile });
  return `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/stream?${query.toString()}`;
}

const snapshotSchema = z.strictObject({ terminals: z.array(terminalInfoSchema) });
const createdSchema = z.strictObject({ terminal: terminalInfoSchema });
const closedSchema = z.strictObject({
  terminal_id: z.string(),
  exit: terminalExitSchema.nullable(),
});
const titleChangedSchema = z.strictObject({ terminal_id: z.string(), title: z.string() });
const modeChangedSchema = z.strictObject({ terminal_id: z.string(), mode: terminalModeSchema });
const leaseChangedSchema = z.discriminatedUnion("lease", [
  z.strictObject({
    terminal_id: z.string(),
    lease: z.literal(terminalLeaseStateSchema.enum.available),
    controller_kind: z.literal(""),
    controller_id: z.literal(""),
    reason: z.string(),
  }),
  z.strictObject({
    terminal_id: z.string(),
    lease: z.literal(terminalLeaseStateSchema.enum.human_owned),
    controller_kind: z.literal(terminalActorKindSchema.enum.human),
    controller_id: z.string().min(1),
    reason: z.string(),
  }),
  z.strictObject({
    terminal_id: z.string(),
    lease: z.literal(terminalLeaseStateSchema.enum.agent_owned),
    controller_kind: z.literal(terminalActorKindSchema.enum.agent),
    controller_id: z.string().min(1),
    reason: z.string(),
  }),
]);

export type TerminalCatalogEvent =
  | { name: "terminal.snapshot"; terminals: TerminalInfo[] }
  | { name: "terminal.created"; terminal: TerminalInfo }
  | { name: "terminal.closed"; terminalId: string; exit: TerminalInfo["exit"] }
  | { name: "terminal.title_changed"; terminalId: string; title: string }
  | { name: "terminal.mode_changed"; terminalId: string; mode: TerminalInfo["mode"] }
  | {
      name: "terminal.lease_changed";
      terminalId: string;
      lease: TerminalInfo["lease"];
      controller: TerminalInfo["controller"];
      reason: string | null;
    };

/**
 * Parses a catalog list frame. Recording frames are subscribed on the same
 * socket but owned by `terminal-recording-state` — they are not catalog rows.
 */
export function parseTerminalCatalogEvent(name: string, raw: unknown): TerminalCatalogEvent | null {
  switch (name) {
    case "terminal.snapshot": {
      const parsed = parseKnownCatalogEvent(snapshotSchema, raw);
      return { name, terminals: parsed.terminals };
    }
    case "terminal.created": {
      const parsed = parseKnownCatalogEvent(createdSchema, raw);
      return { name, terminal: parsed.terminal };
    }
    case "terminal.closed": {
      const parsed = parseKnownCatalogEvent(closedSchema, raw);
      return { name, terminalId: parsed.terminal_id, exit: parsed.exit };
    }
    case "terminal.title_changed": {
      const parsed = parseKnownCatalogEvent(titleChangedSchema, raw);
      return { name, terminalId: parsed.terminal_id, title: parsed.title };
    }
    case "terminal.mode_changed": {
      const parsed = parseKnownCatalogEvent(modeChangedSchema, raw);
      return { name, terminalId: parsed.terminal_id, mode: parsed.mode };
    }
    case "terminal.lease_changed": {
      const parsed = parseKnownCatalogEvent(leaseChangedSchema, raw);
      const controller =
        parsed.lease === terminalLeaseStateSchema.enum.available
          ? null
          : { kind: parsed.controller_kind, id: parsed.controller_id };
      return {
        name,
        terminalId: parsed.terminal_id,
        lease: parsed.lease,
        controller,
        reason: parsed.reason || null,
      };
    }
    default:
      return null;
  }
}

function parseKnownCatalogEvent<Result>(schema: z.ZodType<Result>, raw: unknown): Result {
  const parsed = schema.safeParse(raw);
  if (!parsed.success) throw new TerminalCatalogProtocolError();
  return parsed.data;
}

/**
 * Folds one event into the cached list.
 *
 * A snapshot replaces the list outright — the daemon sends one when the cursor
 * is older than the retained window, and merging into a stale list there would
 * keep terminals the server no longer lists. Everything else patches in place,
 * preserving the server's order.
 */
export function reconcileTerminalCatalog(
  current: TerminalInfo[] | undefined,
  event: TerminalCatalogEvent
): TerminalInfo[] {
  const terminals = current ?? [];
  switch (event.name) {
    case "terminal.snapshot":
      return event.terminals;
    case "terminal.created": {
      const index = terminals.findIndex(terminal => terminal.id === event.terminal.id);
      if (index >= 0) {
        const next = [...terminals];
        next[index] = event.terminal;
        return next;
      }
      return [...terminals, event.terminal];
    }
    case "terminal.closed":
      // An ended terminal stays listed and readable through its exit retention;
      // dropping it here would hide a screen the person can still scroll.
      return patch(terminals, event.terminalId, terminal => ({
        ...terminal,
        state: "exited",
        exit: event.exit ?? terminal.exit ?? null,
      }));
    case "terminal.title_changed":
      return patch(terminals, event.terminalId, terminal => ({
        ...terminal,
        title: event.title,
      }));
    case "terminal.mode_changed":
      return patch(terminals, event.terminalId, terminal => ({ ...terminal, mode: event.mode }));
    case "terminal.lease_changed":
      return patch(terminals, event.terminalId, terminal => ({
        ...terminal,
        lease: event.lease,
        controller: event.controller,
      }));
  }
}

/** Replaces only one owner's rows inside an all-profiles catalog snapshot. */
export function reconcileTerminalProfileSnapshot(
  current: TerminalInfo[] | undefined,
  profile: string,
  terminals: readonly TerminalInfo[]
): TerminalInfo[] {
  const replacement = terminals.filter(terminal => terminal.profile_name === profile);
  if (!current) return [...replacement];

  const next: TerminalInfo[] = [];
  let inserted = false;
  for (const terminal of current) {
    if (terminal.profile_name !== profile) {
      next.push(terminal);
      continue;
    }
    if (!inserted) {
      next.push(...replacement);
      inserted = true;
    }
  }
  if (!inserted) next.push(...replacement);
  return next;
}

function patch(
  terminals: TerminalInfo[],
  terminalId: string,
  update: (terminal: TerminalInfo) => TerminalInfo
): TerminalInfo[] {
  const index = terminals.findIndex(terminal => terminal.id === terminalId);
  if (index < 0) return terminals;
  const next = [...terminals];
  next[index] = update(terminals[index]);
  return next;
}

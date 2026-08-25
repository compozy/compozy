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

export const TERMINAL_CATALOG_EVENTS = [
  "terminal.snapshot",
  "terminal.created",
  "terminal.closed",
  "terminal.title_changed",
  "terminal.lease_changed",
  "terminal.mode_changed",
] as const;

export type TerminalCatalogEventName = (typeof TERMINAL_CATALOG_EVENTS)[number];

const actorKindSchema = z.enum(["human", "agent", "system"]);
const leaseSchema = z.enum(["agent_owned", "human_owned", "available"]);

const exitSchema = z
  .object({
    cause: z.enum(["exited", "signaled", "unknown"]),
    code: z.number().nullish(),
    signal: z.string().nullish(),
    at: z.string(),
  })
  .nullish();

/**
 * The public terminal projection. Parsed rather than trusted: a frame the
 * client cannot read is dropped, never merged half-understood into the list a
 * person is reading.
 */
const terminalInfoSchema = z.object({
  id: z.string(),
  workspace_id: z.string(),
  profile_id: z.string(),
  profile_name: z.string(),
  title: z.string(),
  shell: z.string(),
  cwd: z.string(),
  mode: z.enum(["pty", "pipe"]),
  state: z.enum(["running", "exited"]),
  controller: z.object({ kind: actorKindSchema, id: z.string() }).nullish(),
  lease: leaseSchema,
  viewers: z.number(),
  bound_run: z.object({ session_id: z.string(), run_id: z.string() }).nullish(),
  capabilities: z.object({ interactive: z.boolean() }),
  created_at: z.string(),
  exit: exitSchema,
});

const snapshotSchema = z.object({ terminals: z.array(terminalInfoSchema) });
const createdSchema = z.object({ terminal: terminalInfoSchema });
const closedSchema = z.object({ terminal_id: z.string(), exit: exitSchema });
const titleChangedSchema = z.object({ terminal_id: z.string(), title: z.string() });
const modeChangedSchema = z.object({ terminal_id: z.string(), mode: z.enum(["pty", "pipe"]) });
const leaseChangedSchema = z.object({
  terminal_id: z.string(),
  lease: leaseSchema,
  actor_kind: actorKindSchema.nullish(),
  actor_id: z.string().nullish(),
  reason: z.string().nullish(),
});

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

/** Parses one named catalog frame. Returns null for a frame it cannot read. */
export function parseTerminalCatalogEvent(name: string, raw: unknown): TerminalCatalogEvent | null {
  switch (name) {
    case "terminal.snapshot": {
      const parsed = snapshotSchema.safeParse(raw);
      return parsed.success
        ? { name, terminals: parsed.data.terminals.map(normalizeTerminal) }
        : null;
    }
    case "terminal.created": {
      const parsed = createdSchema.safeParse(raw);
      return parsed.success ? { name, terminal: normalizeTerminal(parsed.data.terminal) } : null;
    }
    case "terminal.closed": {
      const parsed = closedSchema.safeParse(raw);
      return parsed.success
        ? { name, terminalId: parsed.data.terminal_id, exit: parsed.data.exit ?? null }
        : null;
    }
    case "terminal.title_changed": {
      const parsed = titleChangedSchema.safeParse(raw);
      return parsed.success
        ? { name, terminalId: parsed.data.terminal_id, title: parsed.data.title }
        : null;
    }
    case "terminal.mode_changed": {
      const parsed = modeChangedSchema.safeParse(raw);
      return parsed.success
        ? { name, terminalId: parsed.data.terminal_id, mode: parsed.data.mode }
        : null;
    }
    case "terminal.lease_changed": {
      const parsed = leaseChangedSchema.safeParse(raw);
      if (!parsed.success) return null;
      const controller =
        parsed.data.actor_kind && parsed.data.actor_id
          ? { kind: parsed.data.actor_kind, id: parsed.data.actor_id }
          : null;
      return {
        name,
        terminalId: parsed.data.terminal_id,
        lease: parsed.data.lease,
        controller,
        reason: parsed.data.reason ?? null,
      };
    }
    default:
      return null;
  }
}

function normalizeTerminal(parsed: z.infer<typeof terminalInfoSchema>): TerminalInfo {
  return {
    ...parsed,
    controller: parsed.controller ?? null,
    bound_run: parsed.bound_run ?? null,
    exit: parsed.exit ?? null,
  };
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
  current: readonly TerminalInfo[] | undefined,
  event: TerminalCatalogEvent
): TerminalInfo[] {
  const terminals = current ? [...current] : [];
  switch (event.name) {
    case "terminal.snapshot":
      return event.terminals;
    case "terminal.created": {
      const index = terminals.findIndex(terminal => terminal.id === event.terminal.id);
      if (index >= 0) {
        terminals[index] = event.terminal;
        return terminals;
      }
      terminals.push(event.terminal);
      return terminals;
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

import {
  CMD_PALETTE_STRUCTURAL_CONTEXT_KEYS,
  type CmdPaletteContextKey,
  type CmdPaletteContextSnapshot,
} from "./cmd-palette-context";
import type {
  CmdPaletteAvailability,
  CmdPaletteStructuralCommand,
  CmdPalettePredicate,
} from "./cmd-palette-types";

/**
 * Reasons the palette is allowed to author. Everything else is verbatim from the
 * runtime (BR-8) — the UI never invents a specific it cannot prove.
 */
export const ATTACHED_SHELL_REASON = "requires an attached shell";
export const RUNTIME_UNAVAILABLE_REASON = "runtime unavailable";
export const GENERIC_UNAVAILABLE_REASON = "unavailable right now";

const AVAILABLE: CmdPaletteAvailability = { visible: true, available: true, reason: "" };

function disabled(reason: string): CmdPaletteAvailability {
  const verbatim = reason.trim();
  return {
    visible: true,
    available: false,
    reason: verbatim === "" ? GENERIC_UNAVAILABLE_REASON : verbatim,
  };
}

const HIDDEN: CmdPaletteAvailability = { visible: false, available: false, reason: "" };

/**
 * Keys the client owns. Without a snapshot these cannot be answered at all, so a
 * command gated on one reports the attached-shell reason instead of passing —
 * context is never allowed to default to "everything allowed" (US-008.EC-3).
 */
const CLIENT_CONTEXT_KEYS: ReadonlySet<string> = new Set<CmdPaletteContextKey>([
  "window.focused",
  "window.floating",
  "window.stacked",
  "desktop.windowCount",
  "shell.desktop",
  "session.focused.state",
]);

function numeric(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

/** Mirrors `internal/cmdpalette/predicate.go`; an unknown operator never passes. */
export function predicateMatches(
  predicate: CmdPalettePredicate,
  snapshot: CmdPaletteContextSnapshot | null
): boolean {
  if (snapshot === null) return false;
  const actual = (snapshot as Record<string, unknown>)[predicate.key];
  if (actual === undefined) return false;
  const operator = predicate.operator?.trim() || "equals";
  if (operator === "equals") return Object.is(actual, predicate.value);
  if (operator === "not_equals") return !Object.is(actual, predicate.value);
  if (operator === "greater_than_or_equal") {
    const left = numeric(actual);
    const right = numeric(predicate.value);
    return left !== null && right !== null && left >= right;
  }
  return false;
}

export interface AvailabilityInput {
  /** False while the catalog is stale and the daemon has not confirmed it. */
  readonly daemonReachable: boolean;
}

/**
 * How one command reads for this client, right now.
 *
 * The US-037 split lives here: a predicate on a *structural* key fails because
 * the command does not belong to this surface at all, so the row is hidden;
 * a predicate on a *volatile* key fails because of passing state, so the row
 * stays visible and carries the runtime's own reason. Hiding a merely-blocked
 * command would gaslight muscle memory; disabling an irrelevant one would fill
 * the palette with rows that can never light up.
 */
export function resolvePaletteAvailability(
  command: CmdPaletteStructuralCommand,
  snapshot: CmdPaletteContextSnapshot | null,
  { daemonReachable }: AvailabilityInput
): CmdPaletteAvailability {
  if (command.availability_exempt) return AVAILABLE;
  if (!daemonReachable) return disabled(RUNTIME_UNAVAILABLE_REASON);
  const isClientExecuted = command.action.kind !== "tool";
  if (snapshot === null && isClientExecuted) return disabled(ATTACHED_SHELL_REASON);
  for (const predicate of command.when ?? []) {
    if (predicateMatches(predicate, snapshot)) continue;
    if (CMD_PALETTE_STRUCTURAL_CONTEXT_KEYS.has(predicate.key as CmdPaletteContextKey)) {
      return HIDDEN;
    }
    if (snapshot === null && CLIENT_CONTEXT_KEYS.has(predicate.key)) {
      return disabled(ATTACHED_SHELL_REASON);
    }
    return disabled(predicate.reason ?? "");
  }
  return AVAILABLE;
}

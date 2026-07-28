import type { LoopContract, LoopContractVerification } from "../types";

/**
 * The verification criterion type that the configure sheet may toggle without a fork.
 * Every other declared type (`agent-judge` | `human` | `extension`) is structural — it
 * is LOCKED on in the sheet, removing it needs a fork (ADR-009 / design §4.7).
 */
const COMMAND_CHECK_TYPE = "command";

/** The per-check state the configure sheet edits: the enable flag + the command override. */
export interface LoopConfigCheckState {
  enabled: boolean;
  /** Project command for a `command` check; the empty string for non-command checks. */
  command: string;
}

/**
 * Static descriptor for one declared verification check, derived from the loop contract.
 * `command` checks are the only no-fork-toggleable ones and carry the command field; the
 * other typed criteria are shown LOCKED on (`locked`) — they cannot be removed here.
 */
export interface LoopConfigCheckDescriptor {
  id: string;
  type: string;
  isCommand: boolean;
  locked: boolean;
  /** The command declared in the loop definition (`check`), used as the field default. */
  declaredCommand: string;
  /** Human-readable method line (`npm test · expect exit 0`), mirrors the contract panel. */
  method: string;
}

/** The persisted `enabled_checks_json` shape: keyed by command-check id (backend-opaque JSON). */
export type EnabledChecksMap = Record<string, { enabled?: boolean; command?: string }>;

/** Compact method line for one criterion (`command · expect …`), mirrors the contract panel. */
function verificationMethod(criterion: LoopContractVerification): string {
  const parts: string[] = [];
  if (criterion.check) parts.push(criterion.check);
  if (criterion.expect) parts.push(`expect ${criterion.expect}`);
  if (criterion.agent) parts.push(`agent ${criterion.agent}`);
  if (criterion.tool) parts.push(`tool ${criterion.tool}`);
  if (criterion.rubric) parts.push(criterion.rubric);
  if (criterion.prompt) parts.push(criterion.prompt);
  return parts.join(" · ");
}

/** Builds the ordered check descriptors from a loop's declared verification criteria. */
export function buildCheckDescriptors(contract: LoopContract): LoopConfigCheckDescriptor[] {
  const verification = contract.verification ?? [];
  return verification.map(criterion => {
    const isCommand = criterion.type === COMMAND_CHECK_TYPE;
    return {
      id: criterion.id,
      type: criterion.type,
      isCommand,
      locked: !isCommand,
      declaredCommand: isCommand ? (criterion.check ?? "") : "",
      method: verificationMethod(criterion),
    };
  });
}

/** Reads the persisted `enabled_checks_json` blob into a typed map (tolerant of null/garbage). */
export function parseEnabledChecks(raw: unknown): EnabledChecksMap {
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return {};
  const map: EnabledChecksMap = {};
  for (const [id, value] of Object.entries(raw as Record<string, unknown>)) {
    if (!value || typeof value !== "object" || Array.isArray(value)) continue;
    const entry = value as { enabled?: unknown; command?: unknown };
    map[id] = {
      enabled: typeof entry.enabled === "boolean" ? entry.enabled : undefined,
      command: typeof entry.command === "string" ? entry.command : undefined,
    };
  }
  return map;
}

/**
 * Seeds the editable check state from the descriptors + the stored `enabled_checks_json`.
 * A command check defaults to enabled with its declared command; a non-command check is
 * always enabled and carries no command. Stored overrides win where present.
 */
export function initialCheckStates(
  descriptors: LoopConfigCheckDescriptor[],
  storedEnabledChecks: unknown
): Record<string, LoopConfigCheckState> {
  const stored = parseEnabledChecks(storedEnabledChecks);
  const states: Record<string, LoopConfigCheckState> = {};
  for (const descriptor of descriptors) {
    if (descriptor.locked) {
      states[descriptor.id] = { enabled: true, command: "" };
      continue;
    }
    const override = stored[descriptor.id];
    states[descriptor.id] = {
      enabled: override?.enabled ?? true,
      command: override?.command ?? descriptor.declaredCommand,
    };
  }
  return states;
}

/** The default (inherited) check state used by Reset: every check on, declared command restored. */
export function defaultCheckStates(
  descriptors: LoopConfigCheckDescriptor[]
): Record<string, LoopConfigCheckState> {
  const states: Record<string, LoopConfigCheckState> = {};
  for (const descriptor of descriptors) {
    states[descriptor.id] = {
      enabled: true,
      command: descriptor.locked ? "" : descriptor.declaredCommand,
    };
  }
  return states;
}

/**
 * Serializes the command checks into the `enabled_checks_json` blob. Only command checks
 * that DEVIATE from the definition are persisted — an enabled check whose command still
 * equals its declared value inherits (no entry), and a `command` is emitted only when it
 * diverges from `declaredCommand`. This mirrors the numeric omit-default discipline so a
 * later definition change is never silently frozen by a stale pin. Returns `null` when
 * nothing deviates, so the store stays empty.
 */
export function serializeEnabledChecks(
  descriptors: LoopConfigCheckDescriptor[],
  states: Record<string, LoopConfigCheckState>
): EnabledChecksMap | null {
  const map: EnabledChecksMap = {};
  for (const descriptor of descriptors) {
    if (!descriptor.isCommand) continue;
    const state = states[descriptor.id];
    if (!state) continue;
    const command = state.command.trim();
    // An emptied command field means "inherit the declared command" (the field placeholder
    // shows it), never a pinned empty override — only a non-empty command that differs from
    // the declared one is a real divergence (R-401).
    const commandDiverges = command !== "" && command !== descriptor.declaredCommand;
    if (state.enabled && !commandDiverges) continue;
    map[descriptor.id] = commandDiverges
      ? { enabled: state.enabled, command }
      : { enabled: state.enabled };
  }
  return Object.keys(map).length > 0 ? map : null;
}

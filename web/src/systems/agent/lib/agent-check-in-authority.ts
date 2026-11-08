/**
 * HEARTBEAT.md's authority-bearing keys as the Heartbeat page edits them: the write tools a
 * check-in may call (`check_ins.allow_writes`) and the declared `actions`. Caps stay raw-only and
 * are carried through a save unchanged.
 */
import type { FrontmatterMap, FrontmatterValue } from "./agent-frontmatter";

export interface CheckInAction {
  id: string;
  title: string;
  instructions: string;
  /** "Do it without asking me". */
  autoAccept: boolean;
}

export interface CheckInCaps {
  max_tool_calls?: number;
  max_write_calls?: number;
}

export interface CheckInAuthorityForm {
  writes: readonly string[];
  actions: readonly CheckInAction[];
  caps: CheckInCaps;
}

export const EMPTY_AUTHORITY: CheckInAuthorityForm = { writes: [], actions: [], caps: {} };

function isMap(value: FrontmatterValue | undefined): value is FrontmatterMap {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function onlyKeys(map: FrontmatterMap, keys: readonly string[]) {
  const allowed = new Set(keys);
  return Object.keys(map).every(key => allowed.has(key));
}

function readCaps(map: FrontmatterMap): CheckInCaps | null {
  const caps: CheckInCaps = {};
  for (const key of ["max_tool_calls", "max_write_calls"] as const) {
    const value = map[key];
    if (value === undefined || value === null) continue;
    if (typeof value !== "number" || !Number.isInteger(value)) return null;
    caps[key] = value;
  }
  return caps;
}

function readAction(value: FrontmatterValue): CheckInAction | null {
  if (!isMap(value) || !onlyKeys(value, ["id", "title", "instructions", "auto_accept"]))
    return null;
  const { id, title, instructions, auto_accept } = value;
  if (typeof id !== "string" || typeof title !== "string" || typeof instructions !== "string") {
    return null;
  }
  if (auto_accept !== undefined && auto_accept !== null && typeof auto_accept !== "boolean") {
    return null;
  }
  return { id, title, instructions, autoAccept: auto_accept === true };
}

/** The keys the form holds, or `null` when their shape needs the raw file. */
export function readCheckInAuthority(data: FrontmatterMap): CheckInAuthorityForm | null {
  const checkIns = data.check_ins ?? {};
  if (!isMap(checkIns)) return null;
  if (!onlyKeys(checkIns, ["allow_writes", "max_tool_calls", "max_write_calls"])) return null;
  const writes = checkIns.allow_writes ?? [];
  if (!Array.isArray(writes) || !writes.every(item => typeof item === "string")) return null;
  const caps = readCaps(checkIns);
  const actions = data.actions ?? [];
  if (!caps || !Array.isArray(actions)) return null;
  const read = (actions as readonly FrontmatterValue[]).map(readAction);
  if (read.some(action => action === null)) return null;
  return { writes: writes as readonly string[], actions: read as CheckInAction[], caps };
}

/** The frontmatter `check_ins` and `actions` the form writes; empty ones stay out of the file. */
export function composeCheckInAuthority(form: CheckInAuthorityForm) {
  return {
    check_ins: { allow_writes: [...form.writes], ...form.caps },
    actions: form.actions.map(action => ({
      id: action.id,
      title: action.title,
      instructions: action.instructions,
      auto_accept: action.autoAccept,
    })),
  };
}

export function countAuthorityChanges(a: CheckInAuthorityForm, b: CheckInAuthorityForm) {
  return [
    JSON.stringify([...new Set(a.writes)].sort()) !== JSON.stringify([...new Set(b.writes)].sort()),
    JSON.stringify([...a.actions].sort((x, y) => x.id.localeCompare(y.id))) !==
      JSON.stringify([...b.actions].sort((x, y) => x.id.localeCompare(y.id))),
    a.caps.max_tool_calls !== b.caps.max_tool_calls ||
      a.caps.max_write_calls !== b.caps.max_write_calls,
  ].filter(Boolean).length;
}

/**
 * A new action's id from its title: lowercase words joined by `-`, starting with a letter, at most
 * 40 characters, never one already taken.
 */
export function actionIdFor(title: string, taken: readonly string[]) {
  const base =
    title
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "-")
      .replace(/^[^a-z]+|-+$/g, "")
      .slice(0, 36)
      .replace(/-+$/, "") || "action";
  const used = new Set(taken);
  let id = base;
  for (let n = 2; used.has(id); n += 1) id = `${base}-${n}`;
  return id;
}

/** The daemon owns the check-in writable set. */
export function isCheckInWritableTool(tool: { check_in_writable: boolean }) {
  return tool.check_in_writable;
}

/** An action needs a title and instructions before the file can take it. */
export function checkInActionsProblem(actions: readonly CheckInAction[]) {
  return actions.some(action => !action.title.trim() || !action.instructions.trim())
    ? "Give every action a title and instructions"
    : undefined;
}

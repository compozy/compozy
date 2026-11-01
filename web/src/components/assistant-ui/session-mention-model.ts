import type { Unstable_DirectiveFormatter, Unstable_TriggerItem } from "@assistant-ui/react";

import { agentIdentityOf, humanizeAgentSlug } from "@/systems/agent/lib/agent-identity";
import type { AgentPayload } from "@/systems/agent";
import type {
  SessionFileEntry,
  SessionMentionRecent,
  SessionMentionSourceStatus,
  SessionPayload,
} from "@/systems/session";

/** Directive type of every `@` mention chip; skills keep their own type. */
export const MENTION_DIRECTIVE_TYPE = "session-mention";
export const MENTION_TRIGGER_CHARACTER = "@";
/** The picker requests and shows at most 50 file/folder rows (Business Rule 13). */
export const MENTION_FILE_ROW_LIMIT = 50;

export type SessionMentionKind = "file" | "folder" | "session" | "agent";

export interface SessionMentionTarget {
  kind: SessionMentionKind;
  /** Relative path (folders end in `/`), session id, or agent slug. */
  target: string;
}

export interface SessionMentionTrigger {
  /** UTF-16 offset of the `@` in the composer text. */
  offset: number;
  /** Text between the `@` and the caret. */
  query: string;
}

const WHITESPACE_RE = /\s/u;

/**
 * The active `@` occurrence before the caret, with the daemon's start rule
 * (Business Rule 1): `@` at offset 0 or right after whitespace, and no
 * whitespace between it and the caret. `me@example.com` never opens it.
 */
export function detectMentionTrigger(text: string, cursor: number): SessionMentionTrigger | null {
  const upToCursor = text.slice(0, Math.max(0, Math.min(cursor, text.length)));
  for (let index = upToCursor.length - 1; index >= 0; index--) {
    const char = upToCursor[index]!;
    if (WHITESPACE_RE.test(char)) return null;
    if (char !== MENTION_TRIGGER_CHARACTER) continue;
    if (index > 0 && !WHITESPACE_RE.test(upToCursor[index - 1]!)) continue;
    return { offset: index, query: upToCursor.slice(index + 1) };
  }
  return null;
}

/** The daemon trims these from an unquoted token's end (Business Rule 2). */
const TRIMMED_TRAILING_RE = /[.,;:!?)\]}'"]$/u;

/** Unquoted, these leading characters change what the daemon reads the token as. */
const RESERVED_PATH_PREFIXES = ["@", "session:"] as const;

/**
 * The token a picked path serializes to. A path the daemon would split at
 * whitespace, trim at its end, or read as something else (`@@notes.md` is no
 * token, `@session:notes.md` is a session lookup) is quoted, so the sent text
 * always names exactly the picked target (US-002.EC-1).
 */
export function pathMentionToken(path: string): string {
  const quote =
    WHITESPACE_RE.test(path) ||
    TRIMMED_TRAILING_RE.test(path) ||
    RESERVED_PATH_PREFIXES.some(prefix => path.startsWith(prefix));
  return quote ? `@"${path}"` : `@${path}`;
}

export function sessionMentionToken(sessionId: string): string {
  return `@session:${sessionId}`;
}

/**
 * What the model reads for an agent: slug and name. The daemon verifies no agent token, so it
 * stays prompt text, and nothing starts or messages the agent (US-006.EC-1).
 */
export function agentMentionToken(slug: string, label: string): string {
  return `@agent:${slug} (${label})`;
}

export function mentionDirectiveId({ kind, target }: SessionMentionTarget): string {
  return `${kind}:${target}`;
}

/** Reads a chip's kind and target back from its directive id. */
export function parseMentionDirectiveId(id: string): SessionMentionTarget | null {
  const separator = id.indexOf(":");
  if (separator <= 0) return null;
  const kind = id.slice(0, separator);
  const target = id.slice(separator + 1);
  if (target.length === 0) return null;
  if (kind !== "file" && kind !== "folder" && kind !== "session" && kind !== "agent") return null;
  return { kind, target };
}

/** File name, or folder name ending in `/`. */
export function mentionPathName(path: string): string {
  const folder = path.endsWith("/");
  const trimmed = folder ? path.slice(0, -1) : path;
  const name = trimmed.slice(trimmed.lastIndexOf("/") + 1);
  return folder ? `${name}/` : name;
}

export function mentionPathParent(path: string): string {
  const trimmed = path.endsWith("/") ? path.slice(0, -1) : path;
  const slash = trimmed.lastIndexOf("/");
  return slash < 0 ? "" : trimmed.slice(0, slash);
}

/** The label a session chip and row carry: its title, else its id. */
export function sessionMentionLabel(session: Pick<SessionPayload, "id" | "name">): string {
  return session.name?.trim() || session.id;
}

function pathItem(path: string, kind: "file" | "folder"): Unstable_TriggerItem {
  const target = kind === "folder" && !path.endsWith("/") ? `${path}/` : path;
  return {
    id: mentionDirectiveId({ kind, target }),
    type: MENTION_DIRECTIVE_TYPE,
    label: mentionPathName(target),
    description: mentionPathParent(target),
    metadata: { kind, target, token: pathMentionToken(target) },
  };
}

export function fileMentionItem(
  entry: Pick<SessionFileEntry, "path" | "kind">
): Unstable_TriggerItem {
  return pathItem(entry.path, entry.kind);
}

export function sessionMentionItem(
  session: Pick<SessionPayload, "id" | "name" | "agent_name" | "updated_at">
): Unstable_TriggerItem {
  return {
    id: mentionDirectiveId({ kind: "session", target: session.id }),
    type: MENTION_DIRECTIVE_TYPE,
    label: sessionMentionLabel(session),
    description: humanizeAgentSlug(session.agent_name),
    metadata: {
      kind: "session",
      target: session.id,
      token: sessionMentionToken(session.id),
      titled: Boolean(session.name?.trim()),
      updatedAt: session.updated_at,
    },
  };
}

export function agentMentionItem(
  agent: Pick<AgentPayload, "name" | "label" | "resolved_appearance" | "description">
): Unstable_TriggerItem {
  const { name, label } = agentIdentityOf(agent);
  return {
    id: mentionDirectiveId({ kind: "agent", target: name }),
    type: MENTION_DIRECTIVE_TYPE,
    label,
    description: agent.description ?? "",
    metadata: { kind: "agent", target: name, token: agentMentionToken(name, label) },
  };
}

/** The picked item's `@` token — what the composer text and the daemon see. */
export function mentionItemToken(item: Unstable_TriggerItem): string {
  return typeof item.metadata?.token === "string" ? item.metadata.token : item.label;
}

/**
 * Directive formatter for the `@` trigger: `serialize` emits the token. `parse`
 * never turns typed or pasted `@text` into a chip — only the daemon verifies a
 * mention, so the editor never claims one it cannot know (US-002.EC-3).
 */
export const mentionDirectiveFormatter: Unstable_DirectiveFormatter = {
  serialize: mentionItemToken,
  parse: text => [{ kind: "text", text }],
};

export interface MentionMenuRow {
  item: Unstable_TriggerItem;
  flatIndex: number;
}

export interface MentionMenuGroup {
  id: "files" | "sessions" | "agents";
  label: string;
  status: SessionMentionSourceStatus;
  rows: MentionMenuRow[];
  /** Empty / error / loading copy shown in place of rows; `null` when rows show. */
  message: string | null;
  truncated: boolean;
}

export interface MentionMenuModel {
  groups: MentionMenuGroup[];
  /**
   * Every keyboard-reachable row in display order — the trigger adapter's
   * list. Empty while the shown rows are stale, so Enter/Tab can never commit
   * a row that answers an earlier query.
   */
  items: Unstable_TriggerItem[];
  /** True while any group is still resolving its first answer. */
  loading: boolean;
  /** True when rows answering an earlier query are on screen (US-001.EC-6). */
  stale: boolean;
  /** Rows on screen across both groups, selectable or not. */
  rowCount: number;
}

export interface MentionMenuSources {
  query: string;
  files: {
    entries: readonly Pick<SessionFileEntry, "path" | "kind">[];
    status: SessionMentionSourceStatus;
    truncated: boolean;
    /** The entries answer an earlier query than `query`. */
    stale?: boolean;
  };
  sessions: {
    sessions: readonly Pick<SessionPayload, "id" | "name" | "agent_name" | "updated_at">[];
    status: SessionMentionSourceStatus;
    /** The sessions answer an earlier query than `query`. */
    stale?: boolean;
  };
  recents: readonly SessionMentionRecent[];
  agents?: {
    agents: readonly Pick<AgentPayload, "name" | "label" | "resolved_appearance" | "description">[];
    status: SessionMentionSourceStatus;
  };
}

function fileItems(sources: MentionMenuSources): Unstable_TriggerItem[] {
  const seen = new Set<string>();
  const items: Unstable_TriggerItem[] = [];
  const push = (item: Unstable_TriggerItem) => {
    if (seen.has(item.id) || items.length >= MENTION_FILE_ROW_LIMIT) return;
    seen.add(item.id);
    items.push(item);
  };
  // A bare `@` leads with this workspace's recent mentions (US-001.AC-3).
  if (sources.query.trim() === "") {
    for (const recent of sources.recents) push(pathItem(recent.path, recent.kind));
  }
  for (const entry of sources.files.entries) push(fileMentionItem(entry));
  return items;
}

function groupMessage(
  status: SessionMentionSourceStatus,
  rowCount: number,
  copy: { loading: string; empty: string; error: string }
): string | null {
  if (status === "error") return copy.error;
  if (rowCount > 0) return null;
  return status === "loading" ? copy.loading : copy.empty;
}

/**
 * The picker's groups in fixed order — Files, Sessions, then Agents — each owning
 * its empty, error and loading row, so one failing source never hides the
 * other (US-001.EC-3, US-003.EC-3). Rows keep the previous answer while a new
 * one loads; a group in error offers no rows.
 */
export function buildMentionMenuModel(sources: MentionMenuSources): MentionMenuModel {
  const files = sources.files.status === "error" ? [] : fileItems(sources);
  const sessions =
    sources.sessions.status === "error" ? [] : sources.sessions.sessions.map(sessionMentionItem);
  const agents =
    !sources.agents || sources.agents.status === "error"
      ? []
      : sources.agents.agents.map(agentMentionItem);
  const items = [...files, ...sessions, ...agents];
  const groups: MentionMenuGroup[] = [
    {
      id: "files",
      label: "Files",
      status: sources.files.status,
      rows: files.map((item, index) => ({ item, flatIndex: index })),
      message: groupMessage(sources.files.status, files.length, {
        loading: "Loading files…",
        empty: "No matching files",
        error: "Couldn't search files",
      }),
      truncated: sources.files.status === "ready" && sources.files.truncated && files.length > 0,
    },
    {
      id: "sessions",
      label: "Sessions",
      status: sources.sessions.status,
      rows: sessions.map((item, index) => ({ item, flatIndex: files.length + index })),
      message: groupMessage(sources.sessions.status, sessions.length, {
        loading: "Loading sessions…",
        empty: "No matching sessions",
        error: "Couldn't load sessions",
      }),
      truncated: false,
    },
  ];
  if (sources.agents) {
    groups.push({
      id: "agents",
      label: "Agents",
      status: sources.agents.status,
      rows: agents.map((item, index) => ({
        item,
        flatIndex: files.length + sessions.length + index,
      })),
      message: groupMessage(sources.agents.status, agents.length, {
        loading: "Loading agents…",
        empty: "No matching agents",
        error: "Couldn't load agents",
      }),
      truncated: false,
    });
  }
  // Rows answering an earlier query stay visible, undimmed and clickable, but
  // the keyboard reaches none until the current query's rows land: an Enter
  // racing the debounce or the request must not pick a row the typed text no
  // longer names.
  const stale =
    (sources.files.stale === true && files.length > 0) ||
    (sources.sessions.stale === true && sessions.length > 0);
  return {
    groups,
    items: stale ? [] : items,
    loading: groups.some(group => group.status === "loading" && group.rows.length === 0),
    stale,
    rowCount: items.length,
  };
}

export interface MatchSegment {
  text: string;
  match: boolean;
}

/** Splits `text` around its first case-insensitive occurrence of `query`. */
export function emphasizeMatch(text: string, query: string): MatchSegment[] {
  const needle = query.trim().toLowerCase();
  const index = needle ? text.toLowerCase().indexOf(needle) : -1;
  if (index < 0) return [{ text, match: false }];
  return [
    { text: text.slice(0, index), match: false },
    { text: text.slice(index, index + needle.length), match: true },
    { text: text.slice(index + needle.length), match: false },
  ].filter(segment => segment.text.length > 0);
}

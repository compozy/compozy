import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import type { SessionPayload } from "../types";
import { isSubagentSession } from "./session-hierarchy";
import { sessionComputerOf, sessionRowKey, sessionRowKeyFor } from "./session-row-ref";

/**
 * Subagent sessions are excluded from the sidebar page (ADR-005), so a
 * subagent row that must appear — the one the operator is viewing, or a search
 * match — can arrive without its ancestors. These rules decide which ancestors
 * the list loads (through the session detail read) and where the rows land, so
 * a subagent always nests and never becomes a root (S9).
 *
 * - Reveal: the viewed subagent nests under its nearest ancestor on the page;
 *   intermediate subagent sessions between them are loaded as context rows.
 *   When no ancestor is on the page, nothing is revealed.
 * - Search: a matched subagent whose parent did not match is nested under that
 *   parent, loaded as a non-matching context row; the walk stops at the first
 *   ancestor that is on the page or is not itself a subagent (that ancestor
 *   leads its thread).
 * - Deleted ancestor: when a hop's detail read answers not found, the rows
 *   below it stay visible; the outermost one leads its thread and the sidebar
 *   labels it `Subagent of a deleted session` (COPY.md), the one case where a
 *   subagent row is a root.
 */
export interface SubagentContextInput {
  sessions: readonly SessionPayload[];
  /** Ancestors already read through the session detail query, by id. */
  loaded: ReadonlyMap<string, SessionPayload>;
  /** Ancestors whose detail read answered not found: the session is gone. */
  deleted?: ReadonlySet<string>;
  revealed?: SessionPayload | null;
  searching: boolean;
}

type AncestorWalk =
  /** Every ancestor up to `anchorId` (on the page) is known; `chain` is nearest-first. */
  | { kind: "anchored"; anchorId: string; chain: SessionPayload[] }
  /** Search only: the chain's last row is a non-subagent ancestor that leads the thread. */
  | { kind: "context-root"; chain: SessionPayload[] }
  /**
   * An ancestor is deleted: the chain's outermost row (or the target) leads its
   * thread and reads `Subagent of a deleted session`, instead of vanishing.
   */
  | { kind: "orphaned"; chain: SessionPayload[] }
  | { kind: "missing"; id: string; key: string }
  | { kind: "none" };

function parentIdOf(session: SessionPayload): string {
  return session.lineage?.parent_session_id?.trim() ?? "";
}

function walkAncestors(
  start: SessionPayload,
  onPage: ReadonlySet<string>,
  { loaded, deleted, searching }: SubagentContextInput
): AncestorWalk {
  const chain: SessionPayload[] = [];
  const computerId = sessionComputerOf(start);
  const visited = new Set([sessionRowKeyFor(start)]);
  let current = start;
  for (;;) {
    const parentId = parentIdOf(current);
    const key = sessionRowKey({ computerId, sessionId: parentId });
    if (parentId === "" || visited.has(key)) return { kind: "none" };
    if (onPage.has(key)) return { kind: "anchored", anchorId: key, chain };
    if (deleted?.has(key)) return { kind: "orphaned", chain };
    const parent = loaded.get(key);
    if (!parent) return { kind: "missing", id: parentId, key };
    visited.add(key);
    chain.push(parent);
    if (!isSubagentSession(parent)) {
      return searching ? { kind: "context-root", chain } : { kind: "none" };
    }
    current = parent;
  }
}

function subagentTargets({ sessions, revealed, searching }: SubagentContextInput) {
  const onPage = new Set(sessions.map(session => sessionRowKeyFor(session)));
  const matches = searching ? sessions.filter(isSubagentSession) : [];
  const viewed =
    revealed && isSubagentSession(revealed) && !onPage.has(sessionRowKeyFor(revealed))
      ? [revealed]
      : [];
  return { onPage, targets: [...matches, ...viewed] };
}

export interface SubagentContextRequest {
  sessionId: string;
  /** Lineage stays inside one workspace: the ancestor is read where its descendant lives. */
  workspaceId: string;
  computerId?: ComputerId;
}

/** Ancestors still to read (one per unresolved target) before every target can nest. */
export function subagentContextRequests(input: SubagentContextInput): SubagentContextRequest[] {
  const { onPage, targets } = subagentTargets(input);
  const requests = new Map<string, SubagentContextRequest>();
  for (const target of targets) {
    const walk = walkAncestors(target, onPage, input);
    if (walk.kind === "missing" && !requests.has(walk.key)) {
      const computerId = sessionComputerOf(target);
      requests.set(walk.key, {
        sessionId: walk.id,
        workspaceId: target.workspace_id ?? "",
        ...(isRemoteComputerId(computerId) ? { computerId } : {}),
      });
    }
  }
  return [...requests.values()];
}

/**
 * The page with each target's context inserted: ancestors (outermost first)
 * and then the target, right after the on-page anchor, or — for a search
 * context root — in the match's own position. A target whose chain is not yet
 * loaded, or that has no ancestor to nest under, is left out (never a root).
 */
export function withSubagentContext(input: SubagentContextInput): readonly SessionPayload[] {
  const { onPage, targets } = subagentTargets(input);
  if (targets.length === 0) return input.sessions;
  const placed = new Set<string>();
  const after = new Map<string, SessionPayload[]>();
  const replace = new Map<string, SessionPayload[]>();
  const dropped = new Set<string>();
  // A viewed subagent whose ancestry is gone has no anchor: it leads the list.
  const orphans: SessionPayload[] = [];
  for (const target of targets) {
    const targetKey = sessionRowKeyFor(target);
    const walk = walkAncestors(target, onPage, input);
    const fresh = (rows: SessionPayload[]) =>
      rows.filter(row => !placed.has(sessionRowKeyFor(row)));
    if (walk.kind === "anchored" || walk.kind === "context-root") {
      const rows = fresh([...walk.chain].reverse());
      for (const row of rows) placed.add(sessionRowKeyFor(row));
      const group = onPage.has(targetKey) ? rows : [...rows, target];
      if (walk.kind === "anchored") {
        after.set(walk.anchorId, [...(after.get(walk.anchorId) ?? []), ...group]);
      } else if (onPage.has(targetKey)) {
        replace.set(targetKey, [...group, target]);
      } else {
        // A viewed subagent never leads a thread off the page.
        dropped.add(targetKey);
      }
    } else if (walk.kind === "orphaned") {
      const rows = fresh([...walk.chain].reverse());
      for (const row of rows) placed.add(sessionRowKeyFor(row));
      if (onPage.has(targetKey)) replace.set(targetKey, [...rows, target]);
      else orphans.push(...rows, target);
    } else if (onPage.has(targetKey) && parentIdOf(target) !== "") {
      // Its chain is still loading, or it has nowhere to nest: hold it back.
      dropped.add(targetKey);
    }
  }
  return [
    ...orphans,
    ...input.sessions.flatMap(session => {
      const key = sessionRowKeyFor(session);
      if (dropped.has(key)) return [];
      const head = replace.get(key) ?? [session];
      return [...head, ...(after.get(key) ?? [])];
    }),
  ];
}

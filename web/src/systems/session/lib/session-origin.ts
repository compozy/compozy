import type { SessionPayload } from "../types";

/** Lineage kinds that carry a visible origin; spawn/provenance/recovery stay silent. */
export type SessionOriginKind = "continue" | "fork";

export interface SessionOriginParent {
  title: string;
}

/**
 * One label source for the status-line pill and the transcript divider, so the
 * two read the same words for the same child.
 */
export interface SessionOriginView {
  kind: SessionOriginKind;
  parentSessionId: string;
  /** `Continued from` / `Forked from`. */
  verb: string;
  /** Pill subject: the source's agent for a continue, the source's title for a fork. */
  pillSubject: string;
  /** Divider subject: the source's title (the agent or id once the source is gone). */
  dividerSubject: string;
  /** The source is readable on this client; the pill and divider may open it. */
  linkable: boolean;
}

export function sessionOriginKind(session: SessionPayload): SessionOriginKind | null {
  const kind = session.lineage?.kind;
  const parent = session.lineage?.parent_session_id?.trim();
  if (!parent) return null;
  return kind === "continue" || kind === "fork" ? kind : null;
}

/**
 * `parent` is the source session as this client can read it; `null` means the
 * read settled without one (deleted or not visible) and the origin renders as
 * plain text. While the read is in flight pass `undefined`: the words are the
 * same, only the link waits.
 */
export function sessionOriginView(
  session: SessionPayload,
  parent: SessionOriginParent | null | undefined
): SessionOriginView | null {
  const kind = sessionOriginKind(session);
  const parentSessionId = session.lineage?.parent_session_id?.trim() ?? "";
  if (kind === null || parentSessionId === "") return null;
  const originAgent = session.lineage?.origin_agent_name?.trim() ?? "";
  const parentTitle = parent?.title.trim() ?? "";
  const fallbackSubject = kind === "continue" && originAgent !== "" ? originAgent : parentSessionId;
  const titleSubject = parentTitle || fallbackSubject;
  return {
    kind,
    parentSessionId,
    verb: kind === "continue" ? "Continued from" : "Forked from",
    pillSubject: kind === "continue" && originAgent !== "" ? originAgent : titleSubject,
    dividerSubject: titleSubject,
    linkable: parent != null,
  };
}

/** The spawn role delegated subagent sessions carry in their lineage. */
export const SUBAGENT_SPAWN_ROLE = "subagent";

/**
 * The parent of a delegated subagent session (S4), or `null` for every other
 * session — plain `session_spawn` children stay silent.
 */
export function subagentParentSessionId(session: SessionPayload): string | null {
  const lineage = session.lineage;
  if (lineage?.kind !== "spawn" || lineage.spawn_role?.trim() !== SUBAGENT_SPAWN_ROLE) return null;
  return lineage.parent_session_id?.trim() || null;
}

/** Inspector Origin row: `continue · from claude` / `fork · through msg_…` / `fork`. */
export function sessionOriginLedgerValue(session: SessionPayload): string | null {
  const kind = sessionOriginKind(session);
  if (kind === "continue") {
    const agent = session.lineage?.origin_agent_name?.trim();
    return agent ? `continue · from ${agent}` : "continue";
  }
  if (kind === "fork") {
    const message = session.lineage?.origin_message_id?.trim();
    return message ? `fork · through ${message}` : "fork";
  }
  return null;
}

export interface SessionSeedView {
  label: string;
  /** The redacted native-clone error, disclosed on hover/expand only. */
  detail: string | null;
}

/** Inspector Seed line from the child's read model; the client never infers it. */
export function sessionSeedView(session: SessionPayload): SessionSeedView | null {
  const derivation = session.derivation;
  if (!derivation || sessionOriginKind(session) === null) return null;
  if (derivation.seed !== "native_fork") {
    // The clone was attempted at fork time and refused: the carried context runs.
    const attemptError = derivation.native_fork_error?.trim() ?? "";
    return attemptError
      ? { label: "native clone · failed — carried context used", detail: attemptError }
      : { label: "replay", detail: null };
  }
  switch (derivation.native_state) {
    case "loaded":
      return { label: "native clone · loaded", detail: null };
    case "failed":
      return {
        label: "native clone · failed — carried context used",
        detail: derivation.native_fork_error?.trim() || null,
      };
    default:
      return { label: "native clone · pending", detail: null };
  }
}

const ROUTE_NOT_FOUND = "route_not_found";

/** A bind-time `route_not_found` on the child's runtime, as the daemon wrote it. */
export function sessionRouteFailure(session: SessionPayload): string | null {
  const failure = session.runtime.failure?.trim() ?? "";
  return failure.startsWith(ROUTE_NOT_FOUND) ? failure : null;
}

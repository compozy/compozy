export interface SessionSkillInvocationDirective {
  commandId: string;
  token: string;
  start: number;
  end: number;
  label?: string;
  source?: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function invocationFrom(value: unknown): SessionSkillInvocationDirective | null {
  if (!isRecord(value)) return null;
  const commandId = typeof value.command_id === "string" ? value.command_id : "";
  const token = typeof value.token === "string" ? value.token : "";
  const start = typeof value.start === "number" ? value.start : -1;
  const end = typeof value.end === "number" ? value.end : -1;
  if (
    commandId.length === 0 ||
    !token.startsWith("/") ||
    !Number.isInteger(start) ||
    !Number.isInteger(end) ||
    start < 0 ||
    end <= start
  ) {
    return null;
  }
  return {
    commandId,
    token,
    start,
    end,
    ...(typeof value.label === "string" ? { label: value.label } : {}),
    ...(typeof value.source === "string" ? { source: value.source } : {}),
  };
}

/**
 * The message metadata record carrying durable directive refs: top level on
 * the wire, under `custom` on the client runtime.
 */
function directiveMetadata(metadata: unknown, field: string): unknown {
  if (isRecord(metadata) && isRecord(metadata.custom) && field in metadata.custom) {
    return metadata.custom[field];
  }
  return isRecord(metadata) ? metadata[field] : undefined;
}

/** Reads durable server-verified invocation refs while keeping raw tokens copyable. */
export function sessionSkillInvocationDirectives(
  metadata: unknown
): readonly SessionSkillInvocationDirective[] {
  const candidates = directiveMetadata(metadata, "skill_invocations");
  if (!Array.isArray(candidates)) return [];
  return candidates
    .map(invocationFrom)
    .filter((value): value is SessionSkillInvocationDirective => value !== null);
}

/** A daemon-verified `@` mention at its UTF-16 offsets in the authored text. */
export interface SessionMentionDirective {
  kind: "file" | "folder" | "session";
  token: string;
  /** Relative path (folders end in `/`) or session id. */
  target: string;
  /** File/folder name or the session title recorded at send time. */
  label: string;
  start: number;
  end: number;
}

function mentionFrom(value: unknown): SessionMentionDirective | null {
  if (!isRecord(value)) return null;
  const { kind, token, target, label, start, end } = value;
  if (kind !== "file" && kind !== "folder" && kind !== "session") return null;
  if (typeof token !== "string" || !token.startsWith("@")) return null;
  if (typeof target !== "string" || target.length === 0) return null;
  if (
    typeof start !== "number" ||
    typeof end !== "number" ||
    !Number.isInteger(start) ||
    !Number.isInteger(end) ||
    start < 0 ||
    end <= start
  ) {
    return null;
  }
  const recorded = typeof label === "string" ? label.trim() : "";
  return { kind, token, target, label: recorded || target, start, end };
}

/**
 * Reads the daemon-verified `mentions` refs. Only these render as chips; a
 * token the daemon did not resolve stays text (US-008.EC-1), and a message
 * from before the feature has none (US-008.EC-3). Malformed entries are dropped.
 */
export function sessionMentionDirectives(metadata: unknown): readonly SessionMentionDirective[] {
  const candidates = directiveMetadata(metadata, "mentions");
  if (!Array.isArray(candidates)) return [];
  return candidates
    .map(mentionFrom)
    .filter((value): value is SessionMentionDirective => value !== null);
}

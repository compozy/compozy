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

/** Reads durable server-verified invocation refs while keeping raw tokens copyable. */
export function sessionSkillInvocationDirectives(
  metadata: unknown
): readonly SessionSkillInvocationDirective[] {
  const custom = isRecord(metadata) && isRecord(metadata.custom) ? metadata.custom : metadata;
  if (!isRecord(custom)) return [];
  const candidates = custom.skill_invocations;
  if (!Array.isArray(candidates)) return [];
  return candidates
    .map(invocationFrom)
    .filter((value): value is SessionSkillInvocationDirective => value !== null);
}

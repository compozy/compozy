import type { RawLoopNode } from "./codec";

/**
 * Per-kind canvas-card body rows. Only declared DSL fields — never a placeholder,
 * never an implicit runtime default (`on_error → quarantine`), never a guessed
 * `joins:` pairing.
 */

export interface LoopNodeCardRow {
  key: string;
  label: string;
  value: string;
  danger?: boolean;
}

const MAX_CARD_ROWS = 4;

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

function asString(value: unknown): string | undefined {
  return typeof value === "string" && value !== "" ? value : undefined;
}

function asNumber(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function paramsOf(raw: RawLoopNode): Record<string, unknown> | null {
  return asRecord(raw.params);
}

function pushRow(rows: LoopNodeCardRow[], key: string, label: string, value: string | undefined) {
  if (value === undefined) return;
  rows.push({ key, label, value });
}

function retrySummary(raw: RawLoopNode): string | undefined {
  const retry = asRecord(raw.retry);
  if (!retry) return undefined;
  const maxAttempts = asNumber(retry.max_attempts);
  const backoff = asRecord(retry.backoff);
  const base = backoff ? asString(backoff.base) : undefined;
  const cap = backoff ? asString(backoff.max) : undefined;
  if (maxAttempts === undefined && base === undefined && cap === undefined) return undefined;
  if (maxAttempts !== undefined && base && cap) return `${maxAttempts} · ${base}→${cap}`;
  if (maxAttempts !== undefined && base) return `${maxAttempts} · ${base}`;
  if (maxAttempts !== undefined) return String(maxAttempts);
  if (base && cap) return `${base}→${cap}`;
  return base ?? cap;
}

function onErrorRow(raw: RawLoopNode): LoopNodeCardRow | undefined {
  const onError = asRecord(raw.on_error);
  if (!onError) return undefined;
  const route = asString(onError.route);
  if (route) return { key: "on_error", label: "on_error", value: `route → ${route}`, danger: true };
  if (onError.allow_fail === true) {
    return { key: "on_error", label: "on_error", value: "allow_fail", danger: true };
  }
  return undefined;
}

function criteriaRow(value: unknown): LoopNodeCardRow | undefined {
  if (!Array.isArray(value) || value.length === 0) return undefined;
  const first = asRecord(value[0]);
  if (!first) return undefined;
  const type = asString(first.type);
  const id = asString(first.id);
  const check = asString(first.check);
  if (type === "human") return { key: "criteria", label: "human", value: id ?? "human" };
  if (type === "command") {
    return { key: "criteria", label: "check", value: check ?? id ?? "command" };
  }
  if (id) return { key: "criteria", label: "judge", value: id };
  if (type) return { key: "criteria", label: "judge", value: type };
  return undefined;
}

function expiresSummary(value: unknown): string | undefined {
  const expires = asRecord(value);
  if (!expires) return undefined;
  const after = asString(expires.after);
  if (!after) return undefined;
  const escalate = expires.escalate;
  const hasEscalate = Array.isArray(escalate)
    ? escalate.length > 0
    : escalate !== undefined && escalate !== null && escalate !== "";
  return hasEscalate ? `${after} · escalate` : after;
}

function expectSummary(params: Record<string, unknown> | null): string | undefined {
  const expect = asRecord(params?.expect);
  if (!expect) return undefined;
  const required = expect.required;
  if (Array.isArray(required) && required.length > 0 && typeof required[0] === "string") {
    return required[0];
  }
  return undefined;
}

function waitKindLabel(params: Record<string, unknown> | null): string {
  if (!params) return "wait";
  if (params.for !== undefined && params.for !== null) return "wait · for";
  if (params.until !== undefined && params.until !== null) return "wait · until";
  if (params.event !== undefined && params.event !== null) return "wait · event";
  return "wait";
}

function kindRows(raw: RawLoopNode): LoopNodeCardRow[] {
  const rows: LoopNodeCardRow[] = [];
  const kind = asString(raw.kind);
  const params = paramsOf(raw);

  switch (kind) {
    case "input":
      pushRow(rows, "kind", "kind", kind);
      pushRow(rows, "ref", "ref", asString(raw.input_ref));
      break;
    case "file-import":
      pushRow(rows, "pattern", "pattern", asString(raw.pattern));
      pushRow(rows, "parse", "parse", asString(raw.parse));
      break;
    case "watch-source":
    case "watch-events":
    case "collect":
    case "sub-loop":
    case "transform":
      pushRow(rows, "kind", "kind", kind);
      break;
    case "fan-out":
      pushRow(rows, "kind", "kind", kind);
      pushRow(rows, "collection", "collection", asString(raw.collection));
      break;
    case "branch":
      pushRow(rows, "kind", "kind", kind);
      pushRow(rows, "condition", "condition", asString(raw.condition));
      break;
    case "gate": {
      const criterion = criteriaRow(raw.criteria);
      if (criterion) rows.push(criterion);
      pushRow(rows, "verdict", "verdict", asString(raw.verdict_policy));
      pushRow(rows, "expires", "expires", expiresSummary(raw.expires));
      break;
    }
    case "wait":
      pushRow(rows, "kind", "kind", waitKindLabel(params));
      pushRow(rows, "expect", "expect", expectSummary(params));
      pushRow(rows, "expires", "expires", expiresSummary(params?.expires));
      break;
    case "run-agent":
      pushRow(rows, "agent", "agent", asString(params?.agent));
      pushRow(rows, "retry", "retry", retrySummary(raw));
      pushRow(rows, "deadline", "deadline", asString(raw.deadline));
      break;
    case "goal":
      pushRow(rows, "agent", "agent", asString(params?.agent));
      pushRow(rows, "retry", "retry", retrySummary(raw));
      break;
    case "run-loop":
      pushRow(rows, "kind", "kind", kind);
      pushRow(rows, "loop", "loop", asString(params?.loop));
      pushRow(rows, "on_parent_close", "on_parent_close", asString(raw.on_parent_close));
      break;
    default:
      pushRow(rows, "kind", "kind", kind);
      break;
  }
  return rows;
}

function appendOnError(rows: LoopNodeCardRow[], raw: RawLoopNode): LoopNodeCardRow[] {
  const error = onErrorRow(raw);
  if (!error) return rows.slice(0, MAX_CARD_ROWS);
  const without = rows.filter(row => row.key !== "on_error");
  if (without.length >= MAX_CARD_ROWS) {
    return [...without.slice(0, MAX_CARD_ROWS - 1), error];
  }
  return [...without, error];
}

/** 2–4 declared-field rows for one canvas node card. */
export function loopNodeCardRows(raw: RawLoopNode): LoopNodeCardRow[] {
  return appendOnError(kindRows(raw), raw);
}

import type { RawLoopNode } from "./codec";
import { getAtPath, type NodeFieldEdit } from "./loop-editor-draft";
import { str } from "./loop-node-fields";
import {
  LOOP_STRATEGY_KINDS,
  type LoopStrategyKind,
  type StrategyFieldSpec,
} from "./loop-node-schema-types";

function isStrategyKind(value: string): value is LoopStrategyKind {
  return (LOOP_STRATEGY_KINDS as readonly string[]).includes(value);
}

function readThreshold(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "object" && value !== null) {
    const count = (value as Record<string, unknown>).count;
    if (typeof count === "number" && Number.isFinite(count)) return String(count);
  }
  return "";
}

export function readStrategy(raw: RawLoopNode): StrategyFieldSpec {
  const value = getAtPath(raw, ["strategy"]);
  const shorthand = typeof value === "string" ? value.trim() : "";
  const record =
    typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
  const declared = shorthand || str(record?.kind);
  return {
    type: "strategy",
    key: "strategy",
    label: "Completion strategy",
    basePath: ["strategy"],
    kind: isStrategyKind(declared) ? declared : null,
    threshold: readThreshold(record?.threshold),
    missingAcceptable: str(record?.missing) === "acceptable",
    hint: "How the join settles. Undeclared means wait for every lane.",
  };
}

export function strategyEdits(input: {
  kind: LoopStrategyKind | null;
  threshold: string;
  missingAcceptable: boolean;
}): NodeFieldEdit[] {
  if (input.kind === null) return [{ path: ["strategy"], value: undefined }];
  const threshold = input.threshold.trim();
  const needsObject = input.kind === "best_effort" || threshold !== "" || input.missingAcceptable;
  if (!needsObject) return [{ path: ["strategy"], value: input.kind }];
  const value: Record<string, unknown> = { kind: input.kind };
  if (threshold !== "") {
    value.threshold = threshold.endsWith("%") ? threshold : { count: Number(threshold) };
  }
  if (input.missingAcceptable) value.missing = "acceptable";
  return [{ path: ["strategy"], value }];
}

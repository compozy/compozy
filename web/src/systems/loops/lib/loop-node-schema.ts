import type { RawLoopNode } from "./codec";
import type { LoopDefinition } from "../types";
import {
  branchFields,
  collectFields,
  fallbackFields,
  fanOutFields,
  fileImportFields,
  gateFields,
  inputFields,
  runAgentFields,
  runLoopFields,
  str,
  subLoopFields,
  toolActionFields,
  transformFields,
  watchEventsFields,
  watchSourceFields,
} from "./loop-node-fields";
import { goalFields } from "./loop-node-goal-fields";

export type {
  CriteriaFieldSpec,
  FieldPath,
  FieldSpec,
  FoldFieldSpec,
  HintFieldSpec,
  NumberFieldSpec,
  SelectFieldSpec,
  StaticFieldSpec,
  SwitchFieldSpec,
  TextFieldSpec,
} from "./loop-node-schema-types";

import type { FieldSpec } from "./loop-node-schema-types";

/**
 * Resolves the inspector field set for a node from its class + kind. Reserved kinds
 * (run-agent / run-loop / transform) get first-party forms; every other action kind is a
 * ToolID rendered by the generic params form; control/source kinds map to their DSL shapes.
 * An unrecognized kind degrades to id + kind. The per-kind builders live in
 * `loop-node-fields.ts`; this is the thin dispatcher (one responsibility per file).
 */
export function buildNodeFields(
  raw: RawLoopNode,
  definition?: Pick<LoopDefinition, "inputs" | "start">
): FieldSpec[] {
  const nodeClass = str(raw.class);
  const kind = str(raw.kind);
  if (nodeClass === "source") {
    if (kind === "input") return inputFields(raw, Object.keys(definition?.inputs ?? {}));
    if (kind === "file-import") return fileImportFields(raw);
    if (kind === "watch-source") return watchSourceFields(raw);
    if (kind === "watch-events") return watchEventsFields(raw);
  }
  if (nodeClass === "control") {
    if (kind === "fan-out") return fanOutFields(raw);
    if (kind === "collect") return collectFields(raw);
    if (kind === "gate") return gateFields(raw);
    if (kind === "branch") return branchFields(raw);
    if (kind === "sub-loop") return subLoopFields(raw);
  }
  if (nodeClass === "action") {
    if (kind === "goal") return goalFields(raw);
    if (kind === "run-agent") return runAgentFields(raw);
    if (kind === "run-loop") return runLoopFields(raw);
    if (kind === "transform") return transformFields(raw);
    return toolActionFields(raw);
  }
  return fallbackFields(raw);
}

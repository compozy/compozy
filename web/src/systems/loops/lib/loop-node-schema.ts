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
import { reliabilityFold, triggersFold } from "./loop-node-lifecycle-fields";
import { gateExpiryFold, waitFields } from "./loop-node-wait-fields";

export type {
  CriteriaFieldSpec,
  EffectsFieldSpec,
  FieldPath,
  FieldSpec,
  FoldFieldSpec,
  HintFieldSpec,
  LoopWaitMode,
  NumberFieldSpec,
  SelectFieldSpec,
  StaticFieldSpec,
  SwitchFieldSpec,
  TextFieldSpec,
  WaitModeFieldSpec,
} from "./loop-node-schema-types";

import type { FieldSpec } from "./loop-node-schema-types";

/** The kind-specific form for one control node, before the shared lifecycle envelope. */
function controlKindFields(raw: RawLoopNode, kind: string): FieldSpec[] | null {
  if (kind === "fan-out") return fanOutFields(raw);
  if (kind === "collect") return collectFields(raw);
  if (kind === "gate") return [...gateFields(raw), gateExpiryFold(raw)];
  if (kind === "branch") return branchFields(raw);
  if (kind === "sub-loop") return subLoopFields(raw);
  if (kind === "wait") return waitFields(raw);
  return null;
}

/** The kind-specific form for one action node, before the shared lifecycle envelope. */
function actionKindFields(raw: RawLoopNode, kind: string): FieldSpec[] {
  if (kind === "goal") return goalFields(raw);
  if (kind === "run-agent") return runAgentFields(raw);
  if (kind === "run-loop") return runLoopFields(raw);
  if (kind === "transform") return transformFields(raw);
  return toolActionFields(raw);
}

/**
 * Resolves the inspector field set for a node from its class + kind. Reserved kinds
 * (run-agent / run-loop / transform) get first-party forms; every other action kind is a
 * ToolID rendered by the generic params form; control/source kinds map to their DSL shapes.
 * An unrecognized kind degrades to id + kind. The per-kind builders live in
 * `loop-node-fields.ts`; this is the thin dispatcher (one responsibility per file).
 *
 * Lifecycle fields follow the runtime's class rules (ADR-010):
 * - action: reliability (deadline · retry · result_contract · on_error) + the six triggers,
 *   goal drops deadline/backoff/non_retryable (`retry_on_goal_node`);
 * - control: on_error + the six triggers only — a control produces no task run, so a retry or
 *   deadline would be inert and result_contract lacks a declared output schema;
 * - source: nothing — no source kind produces an attempt (`error_route_dead`).
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
    const kindFields = controlKindFields(raw, kind);
    if (kindFields) {
      return [...kindFields, reliabilityFold(raw, "control"), triggersFold(raw)];
    }
  }
  if (nodeClass === "action") {
    const variant = kind === "goal" ? "goal" : "action";
    return [...actionKindFields(raw, kind), reliabilityFold(raw, variant), triggersFold(raw)];
  }
  return fallbackFields(raw);
}

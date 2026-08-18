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
import { askFields } from "./loop-node-ask-fields";
import { readReview } from "./loop-node-review-fields";
import { routeFields } from "./loop-node-route-fields";
import { readStrategy } from "./loop-node-strategy-fields";
import { gateExpiryFold, waitFields } from "./loop-node-wait-fields";

export type {
  CriteriaFieldSpec,
  EffectsFieldSpec,
  FieldPath,
  FieldSpec,
  FoldFieldSpec,
  HintFieldSpec,
  LoopReviewDecision,
  LoopStrategyKind,
  LoopWaitMode,
  NumberFieldSpec,
  ReviewFieldSpec,
  RoutesFieldSpec,
  SelectFieldSpec,
  StaticFieldSpec,
  StrategyFieldSpec,
  SwitchFieldSpec,
  TextFieldSpec,
  WaitModeFieldSpec,
} from "./loop-node-schema-types";

import type { FieldSpec } from "./loop-node-schema-types";

export interface LoopNodeFieldContext {
  forwardNodeIds?: string[];
}

function evalErrorField(): FieldSpec {
  return {
    type: "select",
    key: "on_eval_error",
    label: "on_eval_error",
    path: ["on_eval_error"],
    options: ["fail", "exit"],
    clearOption: { value: "", label: "Inherit the loop policy" },
    hint: "What happens when this node's condition cannot be evaluated. Inherit unless this node needs its own answer.",
  };
}

/** The kind-specific form for one control node, before the shared lifecycle envelope. */
function controlKindFields(
  raw: RawLoopNode,
  kind: string,
  context: LoopNodeFieldContext
): FieldSpec[] | null {
  if (kind === "fan-out") return [...fanOutFields(raw), readStrategy(raw)];
  if (kind === "collect") return collectFields(raw);
  if (kind === "gate") return [...gateFields(raw), gateExpiryFold(raw), evalErrorField()];
  if (kind === "branch") return [...branchFields(raw), evalErrorField()];
  if (kind === "route") return routeFields(raw, context.forwardNodeIds ?? []);
  if (kind === "ask") return askFields(raw);
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

export function buildNodeFields(
  raw: RawLoopNode,
  definition?: Pick<LoopDefinition, "inputs" | "start">,
  context: LoopNodeFieldContext = {}
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
    const kindFields = controlKindFields(raw, kind, context);
    if (kindFields) {
      return [...kindFields, reliabilityFold(raw, "control"), triggersFold(raw)];
    }
  }
  if (nodeClass === "action") {
    const variant = kind === "goal" ? "goal" : "action";

    return [
      ...actionKindFields(raw, kind),
      readReview(raw, context.forwardNodeIds ?? []),
      reliabilityFold(raw, variant),
      triggersFold(raw),
    ];
  }
  return fallbackFields(raw);
}

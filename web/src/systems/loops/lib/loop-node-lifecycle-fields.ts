import type { RawLoopNode } from "./codec";
import type { EffectsFieldSpec, FieldPath, FieldSpec } from "./loop-node-schema-types";

/**
 * Reliability and reaction field descriptors (ADR-010). `loop-node-schema.ts` owns class
 * applicability:
 * - `retry` / `deadline` bind the action attempt path (`lifecycle_config.go`,
 *   `coordinator_action.go`), so they never render on a control node — a control produces no
 *   task run and the control would be inert.
 * - `result_contract` needs a declared output schema, which the linter resolves only for
 *   source/action classes (`linter_reference_schemas.go`), so it is action-only.
 * - `on_error` and the six triggers apply to action and control nodes.
 * - A goal node rejects `deadline`, `retry.backoff` and `retry.non_retryable`
 *   (`retry_on_goal_node`); its own attempt budget stays in `loop-node-goal-fields.ts`.
 */

/** Which reliability keys a node class may author. */
export type LoopLifecycleVariant = "action" | "goal" | "control";

/** The six node lifecycle triggers, in `dsl.TriggerEffects` declaration order. */
const NODE_TRIGGERS: { key: string; label: string }[] = [
  { key: "on_retry", label: "on_retry" },
  { key: "on_success", label: "on_success" },
  { key: "on_pause", label: "on_pause" },
  { key: "on_timeout", label: "on_timeout" },
  { key: "on_cancel", label: "on_cancel" },
  { key: "on_quarantine", label: "on_quarantine" },
];

/** One effect-list descriptor at a raw-node path. */
export function effectsField(key: string, label: string, path: FieldPath): EffectsFieldSpec {
  return { type: "effects", key, label, path };
}

function deadlineField(): FieldSpec {
  return {
    type: "text",
    key: "deadline",
    label: "Deadline (total)",
    path: ["deadline"],
    mono: true,
    optionalLabel: "optional",
    placeholder: "2h",
    hint: "Total budget across every attempt and backoff wait, anchored at first schedule. Must not be shorter than the per-attempt timeout.",
  };
}

function retryFields(): FieldSpec[] {
  return [
    {
      type: "number",
      key: "max_attempts",
      label: "Retry · max_attempts",
      path: ["retry", "max_attempts"],
      hint: "Attempts in total, not extra tries. Leave empty to keep the engine default.",
    },
    {
      type: "text",
      key: "backoff_base",
      label: "Retry · backoff.base",
      path: ["retry", "backoff", "base"],
      mono: true,
      optionalLabel: "optional",
      placeholder: "30s",
    },
    {
      type: "text",
      key: "backoff_max",
      label: "Retry · backoff.max",
      path: ["retry", "backoff", "max"],
      mono: true,
      optionalLabel: "optional",
      placeholder: "5m",
      hint: "The growing pause between attempts, capped here. The run page shows the countdown.",
    },
    {
      type: "textarea",
      key: "non_retryable",
      label: "Retry · non_retryable",
      path: ["retry", "non_retryable"],
      mono: true,
      json: true,
      optionalLabel: "optional · list of strings",
      placeholder: '["tool_invalid_input"]',
      hint: "Failure classes or machine codes that skip retries and go straight to the error path.",
    },
  ];
}

function resultContractFields(): FieldSpec[] {
  return [
    {
      type: "text",
      key: "result_failure_field",
      label: "result_contract · failure_field",
      path: ["result_contract", "failure_field"],
      mono: true,
      optionalLabel: "optional",
      placeholder: "err",
      hint: "Where the output payload reports failure, so a success-shaped error still counts as one. Requires a declared output schema.",
    },
    {
      type: "text",
      key: "result_message_field",
      label: "result_contract · message_field",
      path: ["result_contract", "message_field"],
      mono: true,
      optionalLabel: "optional",
      placeholder: "err.message",
    },
  ];
}

function errorPolicyFields(): FieldSpec[] {
  return [
    {
      type: "text",
      key: "on_error_route",
      label: "on_error · route",
      path: ["on_error", "route"],
      mono: true,
      optionalLabel: "optional",
      placeholder: "next node id",
      hint: "A forward-only edge target taken after retries are spent. Mutually exclusive with allow_fail — declaring both is rejected on publish (error_route_conflict).",
    },
    {
      type: "switch",
      key: "on_error_allow_fail",
      label: "on_error · allow_fail",
      path: ["on_error", "allow_fail"],
      subLabel: "Absorb the failure and continue instead of routing.",
    },
    effectsField("on_error_effects", "on_error · effects", ["on_error", "effects"]),
  ];
}

/** The reliability fold for one node class. */
export function reliabilityFold(_raw: RawLoopNode, variant: LoopLifecycleVariant): FieldSpec {
  if (variant === "control") {
    return {
      type: "fold",
      key: "error_handling",
      label: "Error handling",
      subLabel: "on_error",
      fields: errorPolicyFields(),
    };
  }
  if (variant === "goal") {
    return {
      type: "fold",
      key: "reliability",
      label: "Reliability",
      subLabel: "result_contract · on_error",
      fields: [
        {
          type: "hint",
          key: "goal_retry_hint",
          hint: "A goal node owns its own attempt budget above; deadline, retry.backoff and retry.non_retryable are rejected here (retry_on_goal_node).",
        },
        ...resultContractFields(),
        ...errorPolicyFields(),
      ],
    };
  }
  return {
    type: "fold",
    key: "reliability",
    label: "Reliability",
    subLabel: "deadline · retry · result_contract · on_error",
    fields: [deadlineField(), ...retryFields(), ...resultContractFields(), ...errorPolicyFields()],
  };
}

/** The six node triggers as plain effect lists. */
export function triggersFold(_raw: RawLoopNode): FieldSpec {
  return {
    type: "fold",
    key: "reactions",
    label: "Reactions",
    subLabel: "on_retry · on_success · on_pause · on_timeout · on_cancel · on_quarantine",
    fields: [
      {
        type: "hint",
        key: "reactions_hint",
        hint: "Effects are fail-open and observe-only: a failed effect lands on the timeline and never touches the step. Kill suppresses node reactions.",
      },
      ...NODE_TRIGGERS.map(trigger => effectsField(trigger.key, trigger.label, [trigger.key])),
    ],
  };
}

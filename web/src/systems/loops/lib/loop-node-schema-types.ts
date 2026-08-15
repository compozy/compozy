/**
 * The inspector field-descriptor contract (types only). The per-kind builders in
 * `loop-node-fields.ts` produce these, and `loop-node-schema.ts` dispatches over them —
 * one responsibility per file. This is the "schema generates the inspector" model,
 * derived from the canonical `compozy.loop/v1` DSL types + the ADR-021 reserved kinds;
 * the inspector renders FROM these descriptors, it is not a second schema authority.
 */

import type { LoopEnvironmentMode } from "../types";

export type FieldPath = (string | number)[];

interface FieldCommon {
  key: string;
  label: string;
  hint?: string;
}

export interface TextFieldSpec extends FieldCommon {
  type: "text" | "textarea";
  path: FieldPath;
  mono?: boolean;
  required?: boolean;
  optionalLabel?: string;
  placeholder?: string;
  /** Enables `{{ }}` reference autocomplete over the compiled namespace (ADR-020). */
  reference?: boolean;
  /**
   * The field holds an object/array in the definition (rendered as JSON, parsed back
   * on edit). Set for `params`, `output_schema`, `map`, `watch`, `allowed_tools` so a
   * scalar text edit never coerces a structured field to a string (round-trip safety).
   */
  json?: boolean;
  /**
   * The field is a CEL condition (not a `{{ }}` template) — autocomplete triggers on a
   * bare namespace identifier (`nodes.`, `inputs.`, `item`, …) instead of `{{` (ADR-020).
   */
  cel?: boolean;
}

export interface NumberFieldSpec extends FieldCommon {
  type: "number";
  path: FieldPath;
  ceiling?: number;
}

export interface SelectFieldSpec extends FieldCommon {
  type: "select";
  path: FieldPath;
  options: string[];
  optionalLabel?: string;
  /**
   * A leading option that CLEARS the key instead of writing a value. Optional DSL enums
   * (`ahead_arrival`, `on_parent_close`) have no authored default, so without this the select
   * would claim the first member the moment the inspector renders — an untruthful control.
   */
  clearOption?: { value: string; label: string };
}

export interface SwitchFieldSpec extends FieldCommon {
  type: "switch";
  path: FieldPath;
  subLabel?: string;
}

export interface StaticFieldSpec extends FieldCommon {
  type: "static";
  value: string;
  badge?: string;
}

export interface HintFieldSpec {
  type: "hint";
  key: string;
  hint: string;
}

export interface CriteriaFieldSpec extends FieldCommon {
  type: "criteria";
  path: FieldPath;
  allowedTypes?: readonly ("command" | "agent-judge" | "human" | "extension")[];
}

export interface EventsFieldSpec extends FieldCommon {
  type: "events";
  path: FieldPath;
}

/**
 * One effect list (`on_retry`, `contract.on_failed`, `expires.escalate`, …). Each entry
 * is exactly one of `{emit:{kind,payload?}}` or `{tool,with}` — the editor constructs the
 * shape rather than validating it, so `effect_shape_invalid` can only come from the daemon.
 */
export interface EffectsFieldSpec extends FieldCommon {
  type: "effects";
  path: FieldPath;
}

/** The closed `wait` discriminator vocabulary (`dsl.WaitParams`). */
export const WAIT_MODES = ["for", "until", "event"] as const;
export type LoopWaitMode = (typeof WAIT_MODES)[number];

/**
 * The `wait` mode switch. It is NOT a plain select: `wait_shape_invalid` fires on any extra
 * `params` key, so selecting a mode must write the winner and clear BOTH losers in one draft
 * transition (see `waitModeEdits` + `setNodeFields`).
 */
export interface WaitModeFieldSpec extends FieldCommon {
  type: "wait-mode";
  /** The container the three discriminators live under — `["params"]`. */
  basePath: FieldPath;
  /** The discriminator currently declared, or `null` when the node declares none. */
  mode: LoopWaitMode | null;
}

/**
 * The locked environment-mode vocabulary shared by every CompozyOS surface. Wire
 * values come from the contract (`LoopEnvironmentMode`); the labels are the only
 * mode wording allowed anywhere.
 */
export const LOOP_ENVIRONMENT_MODES = [
  "root",
  "worktree",
  "per_run",
  "directory",
] as const satisfies readonly LoopEnvironmentMode[];

export const LOOP_ENVIRONMENT_MODE_LABELS: Record<LoopEnvironmentMode, string> = {
  root: "Workspace root",
  worktree: "Named worktree",
  per_run: "Per-run",
  directory: "Directory",
};

/**
 * The single execution-environment control on agent-executing nodes.
 *
 * It is not a plain select: `root` and `per_run` must carry neither
 * `worktree_ref` nor `directory`, `worktree` requires only `worktree_ref`, and
 * `directory` requires only `directory`. Switching therefore writes the winner
 * and clears the losers in one transition (`environmentModeEdits` +
 * `setNodeFields`), so no rejected intermediate shape is ever observable.
 */
export interface EnvironmentFieldSpec extends FieldCommon {
  type: "environment";
  /** The container the descriptor lives under — `["params","environment"]`. */
  basePath: FieldPath;
  /** The declared mode, or `null` when the node declares none and inherits the loop default. */
  mode: LoopEnvironmentMode | null;
  /** Label for the inherit choice. Never "None", "Unset", or "Root". */
  inheritLabel: string;
}

export interface FoldFieldSpec extends FieldCommon {
  type: "fold";
  subLabel?: string;
  defaultOpen?: boolean;
  fields: FieldSpec[];
}

export type FieldSpec =
  | TextFieldSpec
  | NumberFieldSpec
  | SelectFieldSpec
  | SwitchFieldSpec
  | StaticFieldSpec
  | HintFieldSpec
  | CriteriaFieldSpec
  | EventsFieldSpec
  | EffectsFieldSpec
  | WaitModeFieldSpec
  | EnvironmentFieldSpec
  | FoldFieldSpec;

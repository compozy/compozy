/**
 * The inspector field-descriptor contract (types only). The per-kind builders in
 * `loop-node-fields.ts` produce these, and `loop-node-schema.ts` dispatches over them —
 * one responsibility per file. This is the "schema generates the inspector" model,
 * derived from the canonical `agh.loop/v1` DSL types + the ADR-021 reserved kinds;
 * the inspector renders FROM these descriptors, it is not a second schema authority.
 */

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

export interface FoldFieldSpec extends FieldCommon {
  type: "fold";
  subLabel?: string;
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
  | FoldFieldSpec;

import type { CmdPaletteArgument, ResolvedPaletteCommand } from "./cmd-palette-types";

/**
 * The inline-argument state machine (`_uiux.md` S8, US-015).
 *
 * Pure by design: the bar renders this state and reports keystrokes back, so
 * "which field blocks ⏎", "what the message says" and "what leaves for the
 * daemon" are decided in one place instead of inside three event handlers.
 *
 * Password discipline is structural here (US-015.EC-4, Safety Invariant 6):
 * password values live in this state and in the invoke payload, and nowhere
 * else — `submitArgs` is the only way out, and personalization records the
 * pre-selection query, never these values.
 */

export type PaletteArgFieldType = "text" | "password" | "dropdown" | "checkbox";

export interface PaletteArgField {
  readonly name: string;
  readonly type: PaletteArgFieldType;
  readonly required: boolean;
  readonly placeholder: string;
  readonly options: readonly string[];
  readonly value: string;
  /** Verbatim field message; empty when the field is fine. */
  readonly error: string;
}

export interface PaletteArgsState {
  readonly commandId: string;
  readonly title: string;
  readonly icon: string;
  readonly fields: readonly PaletteArgField[];
  /** Set by a blocked submit so the bar can focus and emphasize it. */
  readonly focusField: string | null;
}

const KNOWN_TYPES: readonly PaletteArgFieldType[] = ["text", "password", "dropdown", "checkbox"];
const TRUE_VALUES: readonly string[] = ["true", "yes", "1", "on"];
const FALSE_VALUES: readonly string[] = ["false", "no", "0", "off"];

/**
 * An unknown type degrades to text rather than dropping the field: losing an
 * argument silently would let the command run with less than it declared.
 */
function fieldType(declared: string): PaletteArgFieldType {
  const normalized = declared.trim().toLowerCase();
  return KNOWN_TYPES.find(known => known === normalized) ?? "text";
}

function initialField(argument: CmdPaletteArgument): PaletteArgField {
  return {
    name: argument.name,
    type: fieldType(argument.type),
    required: argument.required,
    placeholder: argument.placeholder ?? "",
    options: argument.options ?? [],
    value: "",
    error: "",
  };
}

/** True when the command cannot execute until the operator fills something in. */
export function commandNeedsArguments(command: ResolvedPaletteCommand): boolean {
  return command.arguments.length > 0;
}

export function createArgsState(command: ResolvedPaletteCommand): PaletteArgsState {
  return {
    commandId: command.id,
    title: command.title,
    icon: command.icon,
    fields: command.arguments.map(initialField),
    focusField: null,
  };
}

/**
 * Validates one field against its declared type. Messages are sentence
 * fragments without a trailing period, matching every other runtime reason the
 * palette renders (`_uiux.md` Copy).
 */
export function validateArgValue(field: PaletteArgField, value: string): string {
  const trimmed = value.trim();
  if (trimmed === "") return "";
  if (field.type === "dropdown") {
    if (field.options.length === 0) return "";
    return field.options.includes(trimmed) ? "" : `expected one of ${field.options.join(", ")}`;
  }
  if (field.type === "checkbox") {
    const normalized = trimmed.toLowerCase();
    return TRUE_VALUES.includes(normalized) || FALSE_VALUES.includes(normalized)
      ? ""
      : "expected true or false";
  }
  return "";
}

/** Typing or pasting into a field; validation is immediate so paste blocks too. */
export function setArgValue(
  state: PaletteArgsState,
  name: string,
  value: string
): PaletteArgsState {
  return {
    ...state,
    focusField: null,
    fields: state.fields.map(field =>
      field.name === name ? { ...field, value, error: validateArgValue(field, value) } : field
    ),
  };
}

export interface PaletteArgsSubmission {
  readonly state: PaletteArgsState;
  /** Present only when every field passed; the seam runs with exactly this. */
  readonly values: Readonly<Record<string, unknown>> | null;
}

function coerce(field: PaletteArgField): unknown {
  if (field.type === "dropdown") return field.value.trim();
  if (field.type !== "checkbox") return field.value;
  return TRUE_VALUES.includes(field.value.trim().toLowerCase());
}

/**
 * ⏎ from the bar. A missing required field blocks and takes focus with its
 * placeholder emphasized; an invalid value blocks with its own message
 * (US-015.AC-2, US-015.EC-2). Only a clean pass produces values.
 */
export function submitArgs(state: PaletteArgsState): PaletteArgsSubmission {
  const fields = state.fields.map(field => {
    const typeError = validateArgValue(field, field.value);
    if (typeError !== "") return { ...field, error: typeError };
    if (field.required && field.value.trim() === "") return { ...field, error: "required" };
    return { ...field, error: "" };
  });
  const blocked = fields.find(field => field.error !== "");
  if (blocked !== undefined) {
    return { state: { ...state, fields, focusField: blocked.name }, values: null };
  }
  const values: Record<string, unknown> = {};
  for (const field of fields) {
    if (field.value.trim() === "" && !field.required) continue;
    values[field.name] = coerce(field);
  }
  return { state: { ...state, fields, focusField: null }, values };
}

/** Options a dropdown offers for the value typed so far (US-015.AC-3). */
export function filterArgOptions(field: PaletteArgField, query: string): readonly string[] {
  const needle = query.trim().toLowerCase();
  if (needle === "") return field.options;
  return field.options.filter(option => option.toLowerCase().includes(needle));
}

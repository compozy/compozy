/**
 * The loop-target fields an automation edits when it runs a Loop: the target loop
 * name, static inputs, and (triggers/webhooks) an event-payload -> input mapping.
 * A structural subset of the automation `loop_target` (workspace_id is owned by the
 * automation draft), so the loops system stays decoupled from `@/systems/automation`.
 */
export interface LoopTargetDraft {
  loop_name: string;
  inputs?: Record<string, unknown>;
  input_mapping?: Record<string, string>;
}

/** Switches the target loop, clearing inputs + mapping tied to the previous loop's schema. */
export function setLoopTargetLoop(value: LoopTargetDraft, loopName: string): LoopTargetDraft {
  if (value.loop_name === loopName) return value;
  return { ...value, loop_name: loopName, inputs: {}, input_mapping: {} };
}

/** Sets or clears one static input value (an empty value removes the key). */
export function setLoopTargetInput(
  value: LoopTargetDraft,
  key: string,
  inputValue: unknown
): LoopTargetDraft {
  const inputs = { ...value.inputs };
  if (inputValue === undefined || inputValue === null || inputValue === "") {
    delete inputs[key];
  } else {
    inputs[key] = inputValue;
  }
  return { ...value, inputs };
}

/** Sets or clears one event-payload -> input mapping path (an empty path removes it). */
export function setLoopTargetMapping(
  value: LoopTargetDraft,
  key: string,
  path: string
): LoopTargetDraft {
  const mapping = { ...value.input_mapping };
  if (path.trim() === "") {
    delete mapping[key];
  } else {
    mapping[key] = path;
  }
  return { ...value, input_mapping: mapping };
}
import type {} from "../types";

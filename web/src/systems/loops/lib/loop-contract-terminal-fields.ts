import { effectsField } from "./loop-node-lifecycle-fields";
import type { EffectsFieldSpec } from "./loop-node-schema-types";

export const LOOP_TERMINAL_REACTIONS: EffectsFieldSpec[] = [
  effectsField("on_done", "on_done", ["on_done"]),
  effectsField("on_noop", "on_noop", ["on_noop"]),
  effectsField("on_blocked", "on_blocked", ["on_blocked"]),
  effectsField("on_failed", "on_failed", ["on_failed"]),
  effectsField("on_exhausted", "on_exhausted", ["on_exhausted"]),
  effectsField("on_stalled", "on_stalled", ["on_stalled"]),
  effectsField("on_canceled", "on_canceled", ["on_canceled"]),
];

export const LOOP_TERMINAL_REACTIONS_HINT =
  "Each trigger is a plain effect list firing exactly once per run on the resulting outcome. Cancellation does not suppress terminal reactions.";

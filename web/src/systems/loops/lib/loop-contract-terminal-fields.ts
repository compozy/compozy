import { effectsField } from "./loop-node-lifecycle-fields";
import type { EffectsFieldSpec } from "./loop-node-schema-types";

/**
 * The seven contract terminal reactions (`dsl.TerminalEffects`). Each is a plain effect list
 * that fires exactly once per run on the resulting outcome — `on_canceled` included, because
 * cancel and kill both land on the `canceled` terminal (ADR-010 §1).
 *
 * Paths are relative to `definition.contract`, so the contract lane writes them through
 * `withLoopContractPath` rather than the per-node draft setter.
 */

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
  "Each trigger is a plain effect list firing exactly once per run on the resulting outcome — kill included. Kill suppresses node reactions, never these.";

/**
 * The chips that carry call state, verdict, delivery, and child state.
 *
 * These related projections live in one file because they are one grammar: tone, glyph,
 * and the runtime's exact word always travel together, and splitting them across
 * files is how two of them eventually drift apart.
 *
 * The word inside a chip is the daemon's own term and never leaves the chip —
 * surrounding prose uses plain language, so the screen and the CLI can never
 * disagree about what a state is called.
 */
import { Pill, TypingDots, type PillTone } from "@compozy/ui";

import {
  CALL_DELIVERY_SIGNAL,
  CALL_STATE_SIGNAL,
  CALL_VERDICT_SIGNAL,
  CHILD_STATE_SIGNAL,
} from "../lib/call-state";
import type { CallDelivery, CallState, CallVerdict, ChildState } from "../types";

interface AgentCallStatePillProps {
  /** Null when the daemon sent a word the web does not know. */
  state: CallState | null;
  /** The raw wire value, rendered as-is when narrowing failed. */
  fallbackLabel?: string;
  "data-testid"?: string;
}

/**
 * One call state.
 *
 * An unrecognized state renders its raw word at neutral tone rather than being
 * mapped onto the nearest known one — showing an unfamiliar word is honest;
 * showing a familiar but wrong one is not.
 */
export function AgentCallStatePill({
  state,
  fallbackLabel,
  "data-testid": testId,
}: AgentCallStatePillProps) {
  if (state === null) {
    return (
      <Pill tone="neutral" size="xs" mono data-testid={testId} data-state={fallbackLabel}>
        {fallbackLabel ?? "unknown"}
      </Pill>
    );
  }
  const signal = CALL_STATE_SIGNAL[state];
  const Glyph = signal.glyph;
  return (
    <Pill tone={signal.tone} size="xs" mono data-testid={testId} data-state={state}>
      <Glyph className="size-3" aria-hidden="true" />
      {signal.label}
    </Pill>
  );
}

/**
 * Liveness. Motion says a call is alive; colour never does.
 *
 * A delegation tree can hold dozens of running calls, so tinting them would turn
 * the accent budget into a wash. Reduced motion is handled inside `TypingDots`.
 */
export function AgentCallLiveness({ state }: { state: CallState | null }) {
  if (state === null || !CALL_STATE_SIGNAL[state].live) return null;
  return <TypingDots aria-label="Working" />;
}

/**
 * How the answer arrived, beside the success chip.
 *
 * Deliberately unstyled by tone: provenance is a neutral fact, and tinting
 * `extracted` differently from `returned` would read as a warning about a
 * perfectly valid admission.
 */
export function AgentCallVerdictChip({
  verdict,
  "data-testid": testId,
}: {
  verdict: CallVerdict | null;
  "data-testid"?: string;
}) {
  if (verdict === null) return null;
  return (
    <span data-testid={testId} data-verdict={verdict} className="font-mono text-form text-muted">
      {CALL_VERDICT_SIGNAL[verdict].label}
    </span>
  );
}

/**
 * A delivery receipt on a message row.
 *
 * There is no read or seen state anywhere in this grammar because the runtime
 * models none — the four receipts describe transport, not attention.
 */
export function AgentMessageDeliveryPill({
  delivery,
  fallbackLabel,
  "data-testid": testId,
}: {
  delivery: CallDelivery | null;
  fallbackLabel?: string;
  "data-testid"?: string;
}) {
  if (delivery === null) {
    return (
      <Pill tone="neutral" size="xs" mono data-testid={testId}>
        {fallbackLabel ?? "unknown"}
      </Pill>
    );
  }
  const signal = CALL_DELIVERY_SIGNAL[delivery];
  const Glyph = signal.glyph;
  return (
    <Pill tone={signal.tone} size="xs" mono data-testid={testId} data-delivery={delivery}>
      <Glyph className="size-3" aria-hidden="true" />
      {signal.label}
    </Pill>
  );
}

/**
 * Working, resting, or gone.
 *
 * `parked` is not a degraded state — a parked child is still addressable, and
 * calling or messaging it is what brings it back. That is why no Revive control
 * exists next to this chip anywhere in the app.
 */
export function AgentChildStatePill({
  state,
  "data-testid": testId,
}: {
  state: ChildState;
  "data-testid"?: string;
}) {
  const signal = CHILD_STATE_SIGNAL[state];
  const Glyph = signal.glyph;
  const tone: PillTone = signal.tone;
  return (
    <Pill tone={tone} size="xs" mono data-testid={testId} data-child-state={state}>
      <Glyph className="size-3" aria-hidden="true" />
      {signal.label}
    </Pill>
  );
}

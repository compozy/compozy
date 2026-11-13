import { useReducedMotionConfig } from "motion/react";
import * as React from "react";

import type { DeleteButtonPhase } from "../components/custom/delete-button";

const SPRING = { type: "spring", stiffness: 520, damping: 34 } as const;
const INSTANT = { duration: 0 } as const;

export interface UseDeleteButtonInput {
  phase: DeleteButtonPhase | undefined;
  defaultPhase: Exclude<DeleteButtonPhase, "confirmed">;
  onPhaseChange: ((phase: DeleteButtonPhase) => void) | undefined;
  onCancel: (() => void) | undefined;
  confirmedDuration: number;
  /** Mounted open, focus starts on the cross; otherwise focus moves only on a phase change. */
  autoFocusCancel: boolean;
}

/**
 * The delete button's phase machine: controlled or own phase, focus moved to the cross when the
 * button opens and back to the bin on cancel, and the confirmed check held briefly before the
 * button rests again (a controlled owner hears "rest" through onPhaseChange). Focus never moves on
 * mount unless the host asks for it with `autoFocusCancel`.
 */
export function useDeleteButton({
  phase,
  defaultPhase,
  onPhaseChange,
  onCancel,
  confirmedDuration,
  autoFocusCancel,
}: UseDeleteButtonInput) {
  const [ownPhase, setOwnPhase] = React.useState<DeleteButtonPhase>(defaultPhase);
  const current = phase ?? ownPhase;
  const reduced = useReducedMotionConfig() === true;
  const binRef = React.useRef<HTMLButtonElement | null>(null);
  const cancelRef = React.useRef<HTMLButtonElement | null>(null);
  // Where focus goes once the next phase has rendered its controls. A button mounted open starts
  // on the cross only when its host asks: mounting is not a user action.
  const focusNext = React.useRef<"bin" | "cancel" | null>(
    autoFocusCancel && (phase ?? defaultPhase) === "open" ? "cancel" : null
  );

  const go = (next: DeleteButtonPhase, focus: "bin" | "cancel" | null) => {
    focusNext.current = focus;
    setOwnPhase(next);
    onPhaseChange?.(next);
  };

  React.useEffect(() => {
    const target = focusNext.current;
    focusNext.current = null;
    if (target === "cancel") cancelRef.current?.focus();
    if (target === "bin") binRef.current?.focus();
  }, [current]);

  const settle = React.useEffectEvent(() => {
    setOwnPhase("rest");
    onPhaseChange?.("rest");
  });

  // The button holds the confirmed check briefly, then rests again; a controlled owner hears it.
  React.useEffect(() => {
    if (current !== "confirmed") return;
    const timer = setTimeout(() => settle(), confirmedDuration);
    return () => clearTimeout(timer);
  }, [current, confirmedDuration]);

  const cancel = () => {
    go("rest", "bin");
    onCancel?.();
  };

  return {
    current,
    transition: reduced ? INSTANT : SPRING,
    binRef,
    cancelRef,
    go,
    cancel,
  };
}

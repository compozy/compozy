import { useEffect, useEffectEvent, useRef, useState, type KeyboardEvent } from "react";

/**
 * A pending close: the confirm's phase and where the desktop's tabs go, empty until one is chosen.
 * The card owns the phase, so a remount (after a rename) cannot reopen the confirm by itself.
 */
export interface DesktopCardClosing {
  phase: "open" | "confirmed";
  destinationId: string;
}

/**
 * A desktop card's rename and close state. An action asked from the indicator menu is applied once,
 * whether it seeds a mounting card or arrives at a mounted one, and is reported applied, so its
 * owner consumes it and a remount (a dragged card returns as a new element) does not reapply it
 * (R2-02). `closeKey` changes when a close is applied to a mounted card: keyed on it, the delete
 * button mounts again asking for confirmation with focus on the cross. Escape anywhere on the card cancels a pending close and
 * returns focus to the bin: the destination picker sits outside the delete button, so its Escape
 * reaches the card first and must not close the overview (FB-F-12).
 */
export function useDesktopCardActions(
  initialAction: "rename" | "close" | undefined,
  onInitialActionApplied: (() => void) | undefined
) {
  const [renaming, setRenaming] = useState(initialAction === "rename");
  const [closing, setClosing] = useState<DesktopCardClosing | null>(
    initialAction === "close" ? { phase: "open", destinationId: "" } : null
  );
  const [closeKey, setCloseKey] = useState(0);
  // An action arriving at a mounted card: the initializers above already ran, so apply it here.
  const [seenAction, setSeenAction] = useState(initialAction);
  if (initialAction !== seenAction) {
    setSeenAction(initialAction);
    if (initialAction === "rename") {
      setClosing(null);
      setRenaming(true);
    } else if (initialAction === "close") {
      setRenaming(false);
      setClosing({ phase: "open", destinationId: "" });
      setCloseKey(key => key + 1);
    }
  }
  const actionApplied = useEffectEvent(() => onInitialActionApplied?.());
  useEffect(() => {
    if (initialAction !== undefined) actionApplied();
  }, [initialAction]);

  const binRef = useRef<HTMLButtonElement | null>(null);
  // Set when the card, not the delete button, cancels the confirm; focus then returns to the bin.
  const refocusBin = useRef(false);
  useEffect(() => {
    if (closing !== null || !refocusBin.current) return;
    refocusBin.current = false;
    binRef.current?.focus();
  }, [closing]);

  const cancelCloseOnEscape = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key !== "Escape" || closing?.phase !== "open") return;
    event.preventDefault();
    event.stopPropagation();
    refocusBin.current = true;
    setClosing(null);
  };

  return { renaming, setRenaming, closing, setClosing, closeKey, binRef, cancelCloseOnEscape };
}

import { useEffect, useEffectEvent, type Dispatch, type SetStateAction } from "react";

import type { PermissionDecision } from "../adapters/session-api";
import { isEditableTarget } from "../lib/editable-target";
import { permissionDecisionOptions, permissionSubject } from "../lib/pending-permissions";
import type { PermissionRequest } from "../types";
import { useSessionPermissionDecision } from "./use-session-permission-decision";

// Fixed key map per the ACP decision contract: 1 allow-once · 2 allow-always ·
// 3 reject-once · 4 reject-always. Key 4 fires even while the menu is closed.
const DECISION_KEYS: Record<string, PermissionDecision> = {
  "1": "allow-once",
  "2": "allow-always",
  "3": "reject-once",
  "4": "reject-always",
};

const PASSIVE_TOUCH_LISTENER = { capture: false, passive: true } as const;

export interface UsePermissionDockOptions {
  permission: PermissionRequest;
  sessionId: string;
  workspaceId: string;
  onResolved?: () => void;
  rejectSplitElement: HTMLElement | null;
  menuOpen: boolean;
  setMenuOpen: Dispatch<SetStateAction<boolean>>;
}

/**
 * Behavior for the composer permission dock: decision submission, the digit-key
 * shortcuts (ignoring focused inputs), and the reject split-menu state with its
 * outside-press dismissal.
 */
export function usePermissionDock({
  permission,
  sessionId,
  workspaceId,
  onResolved,
  rejectSplitElement,
  menuOpen,
  setMenuOpen,
}: UsePermissionDockOptions) {
  const { decide, isResolved, isSubmitting } = useSessionPermissionDecision({
    workspaceId,
    sessionId,
    permission,
    onResolved,
  });
  const decisionOptions = permissionDecisionOptions(permission);

  const handleDecisionKey = useEffectEvent((event: KeyboardEvent) => {
    if (isSubmitting || isResolved) return;
    if (isEditableTarget(event.target)) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;
    const decision = DECISION_KEYS[event.key];
    if (!decision || !decisionOptions.includes(decision)) return;
    event.preventDefault();
    decide(decision);
  });

  useEffect(() => {
    document.addEventListener("keydown", handleDecisionKey);
    return () => document.removeEventListener("keydown", handleDecisionKey);
  }, []);

  const handleOutsidePress = useEffectEvent((event: Event) => {
    if (rejectSplitElement?.contains(event.target as Node)) return;
    setMenuOpen(false);
  });

  const handleMenuKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (event.key !== "Escape") return;
    event.preventDefault();
    setMenuOpen(false);
  });

  useEffect(() => {
    if (!menuOpen) return;
    document.addEventListener("mousedown", handleOutsidePress);
    document.addEventListener("touchstart", handleOutsidePress, PASSIVE_TOUCH_LISTENER);
    document.addEventListener("keydown", handleMenuKeyDown, true);
    return () => {
      document.removeEventListener("mousedown", handleOutsidePress);
      document.removeEventListener("touchstart", handleOutsidePress, PASSIVE_TOUCH_LISTENER);
      document.removeEventListener("keydown", handleMenuKeyDown, true);
    };
  }, [menuOpen]);

  return {
    decide,
    isResolved,
    isSubmitting,
    decisionOptions,
    subject: permissionSubject(permission),
  };
}

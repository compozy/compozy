import { useEffect, useEffectEvent, useRef, useState } from "react";

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

export interface UsePermissionDockOptions {
  permission: PermissionRequest;
  sessionId: string;
  workspaceId: string;
  onResolved?: () => void;
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
}: UsePermissionDockOptions) {
  const { decide, isResolved, isSubmitting } = useSessionPermissionDecision({
    workspaceId,
    sessionId,
    permission,
    onResolved,
  });
  const [menuOpen, setMenuOpen] = useState(false);
  const rejectSplitRef = useRef<HTMLDivElement | null>(null);
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

  const handleOutsidePress = useEffectEvent((event: MouseEvent) => {
    if (rejectSplitRef.current?.contains(event.target as Node)) return;
    setMenuOpen(false);
  });

  useEffect(() => {
    if (!menuOpen) return;
    document.addEventListener("mousedown", handleOutsidePress);
    return () => document.removeEventListener("mousedown", handleOutsidePress);
  }, [menuOpen]);

  return {
    decide,
    isResolved,
    isSubmitting,
    menuOpen,
    setMenuOpen,
    rejectSplitRef,
    decisionOptions,
    subject: permissionSubject(permission),
  };
}

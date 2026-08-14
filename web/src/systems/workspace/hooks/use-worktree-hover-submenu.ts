import { useRef } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { worktreeHoverSubmenuLogic } from "../stores/worktree-hover-submenu-store";

interface UseWorktreeHoverSubmenuOptions {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  focusOnOpen: boolean;
  onReturnFocus?: () => void;
}

export function useWorktreeHoverSubmenu({
  open,
  onOpenChange,
  focusOnOpen,
  onReturnFocus,
}: UseWorktreeHoverSubmenuOptions) {
  const store = useStore(worktreeHoverSubmenuLogic);
  const focusRequested = useSelector(store, snapshot => snapshot.context.phase === "focus-pending");
  const triggerRef = useRef<HTMLDivElement | null>(null);
  const contentRef = useRef<HTMLDivElement | null>(null);

  const openNow = () => store.trigger.interactionEntered({ open: () => onOpenChange(true) });
  const scheduleClose = () => store.trigger.closeScheduled({ close: () => onOpenChange(false) });

  const setTriggerRef = (node: HTMLDivElement | null) => {
    triggerRef.current = node;
    if (node === null) store.trigger.disposed();
  };

  const setContentRef = (node: HTMLDivElement | null) => {
    contentRef.current = node;
    if (node === null) return;
    if (!focusRequested && !focusOnOpen) return;
    node
      .querySelector<HTMLElement>(
        "button:not(:disabled), [role=menuitem]:not([aria-disabled=true])"
      )
      ?.focus();
    store.trigger.panelFocused();
  };

  const staysWithinSubmenu = (target: EventTarget | null) =>
    target instanceof Node &&
    (triggerRef.current?.contains(target) === true ||
      contentRef.current?.contains(target) === true);

  const handleBlur = (event: React.FocusEvent<HTMLElement>) => {
    if (!staysWithinSubmenu(event.relatedTarget)) scheduleClose();
  };

  const returnFocus = () => {
    if (onReturnFocus) {
      onReturnFocus();
      return;
    }
    triggerRef.current
      ?.querySelector<HTMLElement>("button, [role=option], [tabindex]:not([tabindex='-1'])")
      ?.focus();
  };

  const handleTriggerKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "ArrowRight") return;
    event.preventDefault();
    if (open) {
      contentRef.current
        ?.querySelector<HTMLElement>(
          "button:not(:disabled), [role=menuitem]:not([aria-disabled=true])"
        )
        ?.focus();
      return;
    }
    store.trigger.keyboardOpenRequested({ open: () => onOpenChange(true) });
  };

  const handleContentKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "ArrowLeft") return;
    event.preventDefault();
    store.trigger.keyboardClosed({ close: () => onOpenChange(false), returnFocus });
  };

  return {
    anchor: triggerRef,
    contentRef: setContentRef,
    triggerRef: setTriggerRef,
    handleBlur,
    handleContentKeyDown,
    handleTriggerKeyDown,
    openNow,
    scheduleClose,
  };
}

import { ConfirmDialog } from "@agh/ui";
import { useBlocker } from "@tanstack/react-router";
import type { ReactNode } from "react";

function shouldBlockNavigation(): boolean {
  return true;
}

export interface UseUnsavedGuardOptions {
  dirty: boolean;
  /** Entity display name interpolated into the confirmation copy. */
  entityName: string;
}

export interface UseUnsavedGuardResult {
  status: "idle" | "blocked";
  proceed: (() => void) | undefined;
  reset: (() => void) | undefined;
  /** Ready-to-render ConfirmDialog; consumer mounts it beside the editor. */
  confirmDialog: ReactNode;
}

/**
 * Shared unsaved navigation guard for agent editors.
 * Pins TanStack Router 1.170.17 useBlocker resolver semantics:
 * blocked.proceed allows navigation; blocked.reset stays.
 */
export function useUnsavedGuard({
  dirty,
  entityName,
}: UseUnsavedGuardOptions): UseUnsavedGuardResult {
  const blocker = useBlocker({
    shouldBlockFn: shouldBlockNavigation,
    withResolver: true,
    disabled: !dirty,
    enableBeforeUnload: dirty,
  });

  const proceed = () => {
    blocker.proceed?.();
  };

  const reset = () => {
    blocker.reset?.();
  };

  const confirmDialog = (
    <ConfirmDialog
      open={blocker.status === "blocked"}
      onOpenChange={open => {
        if (!open) reset();
      }}
      tone="warning"
      title="Discard unsaved changes?"
      description={`Your edits to ${entityName} will be lost.`}
      confirmLabel="Discard changes"
      cancelLabel="Keep editing"
      onConfirm={proceed}
      confirmButtonProps={{ "data-testid": "unsaved-guard-discard" }}
      cancelButtonProps={{ "data-testid": "unsaved-guard-keep-editing" }}
      contentProps={{ "data-testid": "unsaved-guard-dialog" }}
    />
  );

  return {
    status: blocker.status,
    proceed: blocker.status === "blocked" ? proceed : undefined,
    reset: blocker.status === "blocked" ? reset : undefined,
    confirmDialog,
  };
}

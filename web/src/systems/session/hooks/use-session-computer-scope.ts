import { useSelector } from "@xstate/store-react";
import { useEffect, useRef, useState } from "react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { isRemoteComputerId } from "@/lib/computer-scope";
import { useComputers } from "@/systems/computers";

import type { SessionWindowComputerScope } from "../lib/session-sidebar-scope";
import { resolveSessionWindowOrder, sessionSidebarStore } from "./use-session-sidebar-state";

export interface SessionComputerScopeModel {
  /** Active per-window computer filter: `"all"`, `"local"`, or a remote computer id. */
  scope: SessionWindowComputerScope;
  /** Updates this window's computer scope and clears any pending removal notice. */
  setScope: (next: SessionWindowComputerScope) => void;
  /** One-shot notice when the previously scoped computer was removed or disabled (C2, C4). */
  removedName: string | null;
  dismissRemovedNotice: () => void;
}

/**
 * Per-window computer scope with automatic fallback when the scoped computer
 * is removed or disabled in Settings (`C2`, `C4`). Never persists remote data
 * beyond the computer id and resets on account eviction (`C3`).
 */
export function useSessionComputerScope(scopeIdOverride?: string): SessionComputerScopeModel {
  const worktreeScopeId = useWorktreeScopeId();
  const scopeId = scopeIdOverride ?? worktreeScopeId;
  const scope = useSelector(
    sessionSidebarStore,
    snapshot => resolveSessionWindowOrder(snapshot.context.byScope, scopeId).computerScope
  );
  const { computers, enabledComputers, isSuccess, canRead } = useComputers({
    passive: true,
  });
  const [removedName, setRemovedName] = useState<string | null>(null);
  const lastKnownNameRef = useRef<Map<string, string>>(new Map());

  useEffect(() => {
    for (const computer of computers) {
      lastKnownNameRef.current.set(computer.id, computer.name);
    }
  }, [computers]);

  const previousEnabled = useRef<ReadonlySet<string>>(new Set());

  const scopedIsRemote = scope !== "all" && isRemoteComputerId(scope);
  const enabledSet = new Set(enabledComputers.map(computer => computer.id));
  const missingFromEnabled = scopedIsRemote && canRead && isSuccess && !enabledSet.has(scope);

  useEffect(() => {
    if (!missingFromEnabled) return;
    const matched = computers.find(computer => computer.id === scope);
    const name = matched?.name ?? lastKnownNameRef.current.get(scope) ?? scope;
    setRemovedName(name);
    sessionSidebarStore.trigger.remoteComputerRemoved({ computerId: scope });
  }, [missingFromEnabled, computers, scope]);

  useEffect(() => {
    if (!canRead || !isSuccess) return;
    const next = new Set(enabledComputers.map(computer => computer.id));
    for (const computerId of previousEnabled.current) {
      if (!next.has(computerId)) sessionSidebarStore.trigger.remoteComputerRemoved({ computerId });
    }
    previousEnabled.current = next;
  }, [canRead, isSuccess, enabledComputers]);

  const setScope = (next: SessionWindowComputerScope) => {
    setRemovedName(null);
    sessionSidebarStore.trigger.computerScopeChanged({ scopeId, computerScope: next });
  };

  return {
    scope: missingFromEnabled ? "all" : scope,
    setScope,
    removedName,
    dismissRemovedNotice: () => setRemovedName(null),
  };
}

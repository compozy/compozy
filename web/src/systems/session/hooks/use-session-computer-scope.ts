import { useEffect, useRef, useState } from "react";

import { useWorktreeScopeId } from "@/hooks/use-window-scope";
import { isRemoteComputerId } from "@/lib/computer-scope";
import { useComputers } from "@/systems/computers";

import type { SessionWindowComputerScope } from "../lib/session-sidebar-scope";
import { sessionSidebarStore } from "./use-session-sidebar-state";
import { useSessionWindowOrder } from "./use-session-window-order";

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
export function useSessionComputerScope(): SessionComputerScopeModel {
  const scopeId = useWorktreeScopeId();
  const order = useSessionWindowOrder();
  const { computers, enabledComputers, isLoading, isError, canRead } = useComputers();
  const [removedName, setRemovedName] = useState<string | null>(null);
  const lastKnownNameRef = useRef<Map<string, string>>(new Map());

  useEffect(() => {
    for (const computer of computers) {
      lastKnownNameRef.current.set(computer.id, computer.name);
    }
  }, [computers]);

  const scope = order.computerScope;
  const scopedIsRemote = scope !== "all" && isRemoteComputerId(scope);
  const enabledSet = new Set(enabledComputers.map(computer => computer.id));
  const missingFromEnabled =
    scopedIsRemote && canRead && !isLoading && !isError && !enabledSet.has(scope);

  useEffect(() => {
    if (!missingFromEnabled) return;
    const matched = computers.find(computer => computer.id === scope);
    const name = matched?.name ?? lastKnownNameRef.current.get(scope) ?? scope;
    setRemovedName(name);
    sessionSidebarStore.trigger.remoteComputerRemoved({ computerId: scope });
  }, [missingFromEnabled, computers, scope]);

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

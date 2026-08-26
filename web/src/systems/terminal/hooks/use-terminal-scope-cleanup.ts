import { destroyTerminalInstances } from "@compozy/ui";
import { useEffect } from "react";

import {
  isTerminalPaneKey,
  terminalInstanceKey,
  terminalInstanceKeyInScope,
  terminalScopeKey,
} from "../lib/terminal-scope-key";
import type { TerminalStore } from "../contexts/terminal-store-handle";
import type { TerminalInfo } from "../types";

/** Drops emulator resources that no longer belong to the active terminal scope. */
export function useTerminalScopeCleanup({
  workspaceId,
  profile,
  terminals,
  store,
}: {
  workspaceId: string;
  profile: string;
  terminals: readonly TerminalInfo[];
  store: TerminalStore;
}) {
  const scopeKey = terminalScopeKey(workspaceId, profile);
  const liveKeySignature = JSON.stringify(
    terminals
      .filter(terminal => terminal.profile_name === profile)
      .map(terminal => terminalInstanceKey(workspaceId, profile, terminal.id))
  );

  useEffect(() => {
    const live = new Set(JSON.parse(liveKeySignature) as string[]);
    store.trigger.scopeBound({ scopeKey });
    destroyTerminalInstances(key => {
      if (!isTerminalPaneKey(key)) return false;
      if (!terminalInstanceKeyInScope(key, scopeKey)) return true;
      return !live.has(key);
    });
  }, [liveKeySignature, scopeKey, store]);
}

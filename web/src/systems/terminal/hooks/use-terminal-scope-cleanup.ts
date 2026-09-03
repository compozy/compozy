import { destroyTerminalInstance } from "@compozy/ui";
import { useEffect, useState } from "react";

import { terminalInstanceKey, terminalScopeKey } from "../lib/terminal-scope-key";
import type { TerminalStore } from "../contexts/terminal-store-handle";
import type { TerminalInfo } from "../types";

interface TerminalScopeOwner {
  readonly keys: Set<string>;
}

const terminalInstanceOwners = new Map<string, Set<TerminalScopeOwner>>();

function retainTerminalInstance(owner: TerminalScopeOwner, key: string): void {
  if (owner.keys.has(key)) return;
  owner.keys.add(key);
  const owners = terminalInstanceOwners.get(key) ?? new Set<TerminalScopeOwner>();
  owners.add(owner);
  terminalInstanceOwners.set(key, owners);
}

function releaseTerminalInstance(owner: TerminalScopeOwner, key: string): void {
  if (!owner.keys.delete(key)) return;
  const owners = terminalInstanceOwners.get(key);
  owners?.delete(owner);
  if (owners && owners.size > 0) return;
  terminalInstanceOwners.delete(key);
  destroyTerminalInstance(key);
}

function reconcileTerminalInstances(owner: TerminalScopeOwner, live: ReadonlySet<string>): void {
  for (const key of Array.from(owner.keys)) {
    if (!live.has(key)) releaseTerminalInstance(owner, key);
  }
  for (const key of live) retainTerminalInstance(owner, key);
}

function releaseTerminalScope(owner: TerminalScopeOwner): void {
  for (const key of Array.from(owner.keys)) releaseTerminalInstance(owner, key);
}

/** Drops emulator resources only after the last window owning them lets go. */
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
  const [owner] = useState<TerminalScopeOwner>(() => ({ keys: new Set<string>() }));
  const scopeKey = terminalScopeKey(workspaceId, profile);
  const liveKeySignature = JSON.stringify(
    terminals.flatMap(terminal =>
      terminal.profile_name === profile
        ? [terminalInstanceKey(workspaceId, profile, terminal.id)]
        : []
    )
  );

  useEffect(() => () => releaseTerminalScope(owner), [owner]);

  useEffect(() => {
    const live = new Set(JSON.parse(liveKeySignature) as string[]);
    store.trigger.scopeBound({ scopeKey });
    reconcileTerminalInstances(owner, live);
  }, [liveKeySignature, owner, scopeKey, store]);
}

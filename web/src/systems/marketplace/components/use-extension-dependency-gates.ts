import { useState } from "react";

import type { ExtensionDependencyGate } from "./extension-dependency-model";

interface PendingGates {
  gates: ExtensionDependencyGate[];
  settle: (confirmed: boolean) => void;
}

export interface ExtensionDependencyGatesController {
  /** The gate on screen, or null when no dependency consent is pending. */
  current: ExtensionDependencyGate | null;
  /** Walks the gates in order; resolves true when all are confirmed, false on the first cancel. */
  request: (gates: ExtensionDependencyGate[]) => Promise<boolean>;
  accept: () => void;
  cancel: () => void;
}

/** Sequences the per-dependency trust gates of one Add; a cancel anywhere aborts the whole Add. */
export function useExtensionDependencyGates(): ExtensionDependencyGatesController {
  const [pending, setPending] = useState<PendingGates | null>(null);
  return {
    current: pending?.gates[0] ?? null,
    request: gates =>
      gates.length === 0
        ? Promise.resolve(true)
        : new Promise<boolean>(settle => setPending({ gates, settle })),
    accept: () => {
      if (!pending) return;
      const rest = pending.gates.slice(1);
      if (rest.length > 0) {
        setPending({ ...pending, gates: rest });
        return;
      }
      setPending(null);
      pending.settle(true);
    },
    cancel: () => {
      if (!pending) return;
      setPending(null);
      pending.settle(false);
    },
  };
}

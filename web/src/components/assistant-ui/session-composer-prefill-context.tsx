import type { ReactNode } from "react";

import {
  SessionComposerPrefillContext,
  type SessionComposerPrefill,
} from "./session-composer-prefill-context-value";

export function SessionComposerPrefillProvider({
  children,
  setComposerText,
}: {
  children: ReactNode;
  setComposerText: SessionComposerPrefill;
}) {
  return (
    <SessionComposerPrefillContext.Provider value={setComposerText}>
      {children}
    </SessionComposerPrefillContext.Provider>
  );
}

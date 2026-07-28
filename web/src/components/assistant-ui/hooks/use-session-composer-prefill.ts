import { use } from "react";

import {
  SessionComposerPrefillContext,
  type SessionComposerPrefill,
} from "../session-composer-prefill-context-value";

export function useSessionComposerPrefill(): SessionComposerPrefill | null {
  return use(SessionComposerPrefillContext);
}

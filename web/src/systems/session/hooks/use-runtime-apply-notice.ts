import { use, useState } from "react";

import {
  SessionPromptRuntimeActivityContext,
  SessionPromptRuntimeCapabilitiesContext,
} from "../contexts/session-prompt-runtime-context-value";
import {
  currentRuntimeApplyNotice,
  runtimeApplyTargetSettled,
  type RuntimeApplyTarget,
} from "../lib/runtime-apply-notice";

/** Tracks the last saved runtime selection and reports its effect from the live session. */
export function useRuntimeApplyNotice() {
  const badge = use(SessionPromptRuntimeActivityContext);
  const runtime = use(SessionPromptRuntimeCapabilitiesContext);
  const [applyTarget, setApplyTarget] = useState<RuntimeApplyTarget | null>(null);
  const live = { badge, runtime };
  const applyNotice = applyTarget ? currentRuntimeApplyNotice(applyTarget, live) : null;
  if (applyTarget && runtimeApplyTargetSettled(applyTarget, live)) setApplyTarget(null);
  return { applyNotice, setApplyTarget };
}

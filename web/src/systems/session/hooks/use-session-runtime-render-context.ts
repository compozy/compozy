import { use } from "react";

import { SessionRuntimeRenderContext } from "../lib/session-runtime-render-context-value";

export function useSessionRuntimeRenderContext() {
  return use(SessionRuntimeRenderContext);
}

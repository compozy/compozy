import { use } from "react";

import { ViewRuntimeContext } from "../runtime-context.js";
import type { ViewRuntime } from "../runtime-context.js";

export function useViewRuntime(): ViewRuntime {
  const runtime = use(ViewRuntimeContext);
  if (!runtime) {
    throw new Error("view hooks must run inside a CompozyOS view session");
  }
  return runtime;
}

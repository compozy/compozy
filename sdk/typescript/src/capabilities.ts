import { InternalError } from "./errors.js";
import { WATCH_POLL_METHOD, WATCH_SOURCE_CAPABILITY } from "./watch-source.js";

export const TOOL_PROVIDER_CAPABILITY = "tool.provider";
export const PROVIDE_TOOLS_METHOD = "provide_tools";
export const TOOLS_CALL_METHOD = "tools/call";

const REQUIRED_PROVIDES_METHODS: Record<string, string[]> = {
  "bridge.adapter": ["bridges/deliver"],
  "memory.backend": ["memory/store", "memory/recall", "memory/forget"],
  [TOOL_PROVIDER_CAPABILITY]: [PROVIDE_TOOLS_METHOD, TOOLS_CALL_METHOD],
  [WATCH_SOURCE_CAPABILITY]: [WATCH_POLL_METHOD],
};

export function isToolProviderMethod(method: string): boolean {
  return method === PROVIDE_TOOLS_METHOD || method === TOOLS_CALL_METHOD;
}

export function validateProvidedMethodCoverage(
  provides: readonly string[],
  implementedMethods: readonly string[]
): void {
  const implemented = new Set(implementedMethods);
  for (const capability of provides) {
    const requiredMethods = REQUIRED_PROVIDES_METHODS[capability];
    if (!requiredMethods) {
      continue;
    }
    const missing = requiredMethods.filter(method => !implemented.has(method));
    if (missing.length > 0) {
      throw new InternalError(`capability ${capability} requires methods ${missing.join(", ")}`);
    }
  }
}

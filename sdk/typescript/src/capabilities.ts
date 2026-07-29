import { InternalError } from "./errors.js";
import { REQUIRED_METHODS_BY_PROVIDE } from "./generated/contracts.js";

export const TOOL_PROVIDER_CAPABILITY = "tool.provider";
export const PROVIDE_TOOLS_METHOD = "provide_tools";
export const TOOLS_CALL_METHOD = "tools/call";

export function isToolProviderMethod(method: string): boolean {
  return method === PROVIDE_TOOLS_METHOD || method === TOOLS_CALL_METHOD;
}

export function validateProvidedMethodCoverage(
  provides: readonly string[],
  implementedMethods: readonly string[]
): void {
  const implemented = new Set(implementedMethods);
  for (const capability of provides) {
    const requiredMethods = (
      REQUIRED_METHODS_BY_PROVIDE as Readonly<Record<string, readonly string[]>>
    )[capability];
    if (!requiredMethods) {
      continue;
    }
    const missing = requiredMethods.filter(method => !implemented.has(method));
    if (missing.length > 0) {
      throw new InternalError(`capability ${capability} requires methods ${missing.join(", ")}`);
    }
  }
}

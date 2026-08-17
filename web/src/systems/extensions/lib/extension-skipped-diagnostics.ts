import type { ExtensionKitInventory } from "../types";

type ExtensionInventoryDiagnostic = NonNullable<ExtensionKitInventory["diagnostics"]>[number];

const skippedDiagnosticCodes = new Set([
  "extension_agent_plugin_component_skipped",
  "extension_mcp_server_unhealthy",
]);

function selectExtensionSkippedDiagnostics(
  diagnostics: readonly ExtensionInventoryDiagnostic[] | undefined
): ExtensionInventoryDiagnostic[] {
  return diagnostics?.filter(diagnostic => skippedDiagnosticCodes.has(diagnostic.code)) ?? [];
}

export { selectExtensionSkippedDiagnostics };
export type { ExtensionInventoryDiagnostic };

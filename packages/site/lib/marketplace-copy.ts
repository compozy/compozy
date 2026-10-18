export const MARKETPLACE_DESCRIPTION =
  "Extensions with MCP servers, skills and tools for CompozyOS — rendered from this build's checked-in catalog snapshot.";

export function bundledExtensionDescription(description: string): string {
  return `${description} Bundled with the CompozyOS runtime and enrolled at first boot.`;
}

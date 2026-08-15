export const MARKETPLACE_DESCRIPTION =
  "Skills, extensions, MCP servers, and bridge providers for CompozyOS — rendered from this build's checked-in catalog snapshot.";

export function marketplaceBridgesDescription(providerCount: number): string {
  return `Chat and tracker platforms your agents can live in: the ${providerCount} in-tree CompozyOS bridge providers, their secret slots, and their setup guides.`;
}

export function bundledSpecCycleDescription(description: string): string {
  return `${description} Bundled with the CompozyOS runtime and enrolled at first boot.`;
}

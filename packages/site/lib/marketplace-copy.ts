export const MARKETPLACE_DESCRIPTION =
  "Skills, extensions, MCP servers, and bridge providers for CompozyOS — rendered from this build's checked-in catalog snapshot.";

export function marketplaceBridgesDescription(providerCount: number): string {
  return `Chat and tracker platforms your agents can live in: the ${providerCount} in-tree Compozy bridge providers, their secret slots, and their setup guides.`;
}

export function bundledDevCycleDescription(description: string): string {
  return `${description} Bundled with the Compozy runtime and enrolled at first boot.`;
}

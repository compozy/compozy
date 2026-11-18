import { providerKindIconRegistry } from "@compozy/ui";

interface ProviderLogoSource {
  name: string;
  settings?: { runtime_provider?: string };
}

/**
 * Logo key for a provider: its own brand when one exists, otherwise the family
 * it runs on (an alias such as `claude-mm1` with `runtime_provider = "claude"`),
 * otherwise its name. Built-ins with their own brand (for example `pi`, which
 * runs on Anthropic) keep that brand.
 */
export function providerLogoKey(provider: ProviderLogoSource): string {
  const name = provider.name.trim().toLowerCase();
  if (Object.hasOwn(providerKindIconRegistry, name)) return name;
  return provider.settings?.runtime_provider?.trim().toLowerCase() || name;
}

import { Disclosure } from "@compozy/ui";

import type { SettingsProviderEntry } from "../types";
import { ProviderCard, ProviderCardGrid } from "./provider-card";

interface DisabledProvidersGroupProps {
  providers: SettingsProviderEntry[];
  onOpen: (entry: SettingsProviderEntry) => void;
}

/** Turned-off providers, folded away after the grid. Renders nothing while none are off. */
export function DisabledProvidersGroup({ providers, onOpen }: DisabledProvidersGroupProps) {
  if (providers.length === 0) return null;
  return (
    <Disclosure
      data-testid="settings-page-providers-disabled-group"
      label={`Disabled (${providers.length})`}
      size="md"
      triggerProps={{ "data-testid": "settings-page-providers-disabled-toggle" }}
    >
      <ProviderCardGrid data-testid="settings-page-providers-disabled-list">
        {providers.map(provider => (
          <ProviderCard key={provider.name} onOpen={onOpen} provider={provider} />
        ))}
      </ProviderCardGrid>
    </Disclosure>
  );
}

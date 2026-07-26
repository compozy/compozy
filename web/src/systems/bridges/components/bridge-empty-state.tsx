import { Plus, Waypoints } from "lucide-react";

import { Button, Empty, Section } from "@agh/ui";

import {
  buildBridgeProviderKey,
  isBridgeProviderSelectable,
} from "@/systems/bridges/lib/bridge-formatters";
import type { BridgeProvider } from "@/systems/bridges/types";
import { BridgeProviderCatalogCard } from "./bridge-provider-catalog-card";

interface BridgeEmptyStateProps {
  onCreate: () => void;
  providers: BridgeProvider[];
}

export function BridgeEmptyState({ onCreate, providers }: BridgeEmptyStateProps) {
  const hasInstalledProviders = providers.length > 0;
  const canCreate = providers.some(isBridgeProviderSelectable);

  const title = hasInstalledProviders ? "No bridges configured" : "No bridge providers installed";
  const description = hasInstalledProviders
    ? "Start by creating a bridge from an installed provider. Bridge instances keep routing, delivery defaults, and health separated per workspace or globally."
    : "Install a bridge-capable extension first. The create flow only becomes available when AGH can discover a provider through the runtime catalog.";

  const action = (
    <>
      <Button
        className="min-w-36"
        data-testid="bridge-empty-create-btn"
        disabled={!canCreate}
        onClick={onCreate}
        size="lg"
      >
        <Plus className="size-4" />
        Create Bridge
      </Button>
      {!canCreate && hasInstalledProviders ? (
        <p className="text-xs text-subtle">
          Installed providers are currently unavailable. Resolve extension health first.
        </p>
      ) : null}
    </>
  );

  return (
    <div className="flex flex-1 overflow-y-auto p-6" data-testid="bridges-empty-state">
      <div className="mx-auto flex w-full max-w-5xl flex-col gap-6">
        <Empty action={action} description={description} icon={Waypoints} title={title} />

        {hasInstalledProviders ? (
          <Section label="Installed providers">
            <p className="text-small-body text-muted">
              Providers come from installed bridge-capable extensions. Unavailable providers stay
              visible so the operator can diagnose runtime state.
            </p>
            <div className="mt-3 grid gap-4 lg:grid-cols-2">
              {providers.map(provider => (
                <BridgeProviderCatalogCard
                  key={buildBridgeProviderKey(provider)}
                  provider={provider}
                >
                  <p className="text-eyebrow leading-relaxed text-subtle">
                    {provider.health_message ||
                      (isBridgeProviderSelectable(provider)
                        ? "This provider can be used to create a bridge instance."
                        : "This provider is installed but not available for bridge creation right now.")}
                  </p>
                </BridgeProviderCatalogCard>
              ))}
            </div>
          </Section>
        ) : null}
      </div>
    </div>
  );
}

import { FileJson2, Plug } from "lucide-react";

import { ActionResultBanner, FormSection } from "@agh/ui";

import { buildBridgeProviderKey, isBridgeProviderSelectable } from "../lib/bridge-formatters";
import type { BridgeProvider } from "../types";
import { BridgeProviderCatalogCard } from "./bridge-provider-catalog-card";

interface BridgeCreateProviderStepProps {
  disabled?: boolean;
  onSelect: (providerKey: string) => void;
  providers: BridgeProvider[];
  selectedProviderKey: string;
  supportsManifest: boolean;
}

export function BridgeCreateProviderStep({
  disabled = false,
  onSelect,
  providers,
  selectedProviderKey,
  supportsManifest,
}: BridgeCreateProviderStepProps) {
  return (
    <FormSection
      data-testid="bridge-wizard-section-provider"
      description="Only providers with healthy runtime state can be selected for bridge creation."
      icon={Plug}
      title="Provider"
    >
      {providers.length === 0 ? (
        <div
          className="rounded bg-canvas-tint px-5 py-8 text-center text-small-body leading-6 text-muted"
          data-testid="bridge-provider-empty"
        >
          No bridge providers are currently available. Install or enable a bridge adapter extension
          before creating a new bridge.
        </div>
      ) : (
        // Provider choice is permanent — `UpdateBridgeRequest` omits platform and
        // extension — so it is a mutually exclusive, consequence-bearing choice
        // and carries radiogroup semantics, not independent toggle buttons.
        <div
          aria-label="Bridge provider"
          className="grid gap-3 lg:grid-cols-2"
          data-testid="bridge-wizard-provider-grid"
          role="radiogroup"
        >
          {providers.map(provider => {
            const providerKey = buildBridgeProviderKey(provider);
            const selectable = !disabled && isBridgeProviderSelectable(provider);
            const selected = providerKey === selectedProviderKey;

            return (
              <BridgeProviderCatalogCard
                actionable={selectable}
                aria-checked={selected}
                key={providerKey}
                onClick={selectable ? () => onSelect(providerKey) : undefined}
                onKeyDown={
                  selectable
                    ? event => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          onSelect(providerKey);
                        }
                      }
                    : undefined
                }
                provider={provider}
                role="radio"
                selected={selected}
                tabIndex={selectable ? 0 : -1}
              />
            );
          })}
        </div>
      )}

      {supportsManifest && selectedProviderKey ? (
        <ActionResultBanner
          data-testid="bridge-manifest-precreate-hint"
          description="AGH generates the JSON from the persisted bridge ID and saved webhook URL. Create the bridge first, then copy the real manifest into Slack."
          icon={FileJson2}
          title="Slack manifest available after creation"
          tone="info"
        />
      ) : null}
    </FormSection>
  );
}

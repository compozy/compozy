import type { ReactNode } from "react";

import {
  bridgeKindIconRegistry,
  CatalogCard,
  Eyebrow,
  KindChip,
  KindIcon,
  Pill,
  type CatalogCardProps,
} from "@agh/ui";

import { providerHealthTone, providerStateTone } from "@/systems/model-catalog";
import { buildBridgeProviderKey, isBridgeProviderSelectable } from "../lib/bridge-formatters";
import type { BridgeProvider } from "../types";

type BridgeProviderCatalogCardProps = Omit<CatalogCardProps, "aria-disabled" | "children"> & {
  children?: ReactNode;
  provider: BridgeProvider;
};

export function BridgeProviderCatalogCard({
  children,
  provider,
  ...props
}: BridgeProviderCatalogCardProps) {
  const selectable = isBridgeProviderSelectable(provider);

  return (
    <CatalogCard
      {...props}
      aria-disabled={selectable ? undefined : true}
      data-testid={`bridge-provider-card-${buildBridgeProviderKey(provider)}`}
    >
      <div className="flex items-start gap-3">
        <CatalogCard.Logo size="lg">
          <KindIcon
            kind={provider.platform}
            registry={bridgeKindIconRegistry}
            size="md"
            tone="default"
          />
        </CatalogCard.Logo>
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <CatalogCard.Title className="min-w-0">{provider.display_name}</CatalogCard.Title>
            <Pill mono tone={providerHealthTone(provider.health)}>
              {provider.health}
            </Pill>
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <KindChip kind={provider.platform} />
            <Eyebrow className="text-muted">{provider.extension_name}</Eyebrow>
          </div>
        </div>
      </div>
      <CatalogCard.Description>
        {provider.description ?? "Bridge adapter installed and ready for instance configuration."}
      </CatalogCard.Description>
      <CatalogCard.Actions className="border-t-0 pt-0">
        <Pill mono tone={providerStateTone(provider.state)}>
          {provider.state}
        </Pill>
        {selectable ? null : (
          <Pill mono tone="danger">
            UNAVAILABLE
          </Pill>
        )}
      </CatalogCard.Actions>
      {children}
    </CatalogCard>
  );
}

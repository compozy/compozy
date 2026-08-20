import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Empty,
  ListingRow,
  Spinner,
} from "@compozy/ui";
import { Radio } from "lucide-react";

import { TIER_LABEL } from "../lib/gateway-copy";
import type { GatewayProviderCandidate, GatewayProviderModel } from "../lib/gateway-provider-model";
import type { GatewayTier } from "../types";
import { GatewayProviderHealthChip } from "./gateway-provider-health-chip";
import { GatewayProviderRow } from "./gateway-provider-row";
import { SettingsGroup } from "@/systems/settings";

export interface GatewayProviderSectionProps {
  model: GatewayProviderModel;
  isLoading: boolean;
  isSubmitting: boolean;
  inventoryError?: string | null;
  actionError?: string | null;
  onEnable: (candidate: GatewayProviderCandidate, tier: GatewayTier) => void;
  onDisable: (name: string, tier: GatewayTier) => void;
}

/**
 * Which connectivity provider carries each tier, and its live health.
 *
 * The daemon owns exposure policy; a provider only contributes a reachability
 * mechanism. Nothing here is turned on by choosing a provider — a surface still
 * has to be enabled on the ladder above.
 */
export function GatewayProviderSection({
  model,
  isLoading,
  isSubmitting,
  inventoryError,
  actionError,
  onEnable,
  onDisable,
}: GatewayProviderSectionProps) {
  return (
    <SettingsGroup data-testid="gateway-provider-section" title="Connectivity providers">
      {inventoryError ? (
        <div className="px-4 py-3">
          <Alert variant="warning">
            <AlertTitle>Installed providers could not be read</AlertTitle>
            <AlertDescription>{inventoryError}</AlertDescription>
          </Alert>
        </div>
      ) : null}
      {actionError ? (
        <div className="px-4 py-3">
          <Alert data-testid="gateway-provider-error" variant="danger">
            <AlertTitle>That provider change was not applied</AlertTitle>
            <AlertDescription>{actionError}</AlertDescription>
          </Alert>
        </div>
      ) : null}

      {isLoading ? (
        <div
          aria-label="Loading connectivity providers"
          className="flex justify-center py-8"
          role="status"
        >
          <Spinner aria-hidden="true" className="size-4 text-subtle" />
        </div>
      ) : null}

      {!isLoading && !inventoryError && model.isEmpty ? (
        <Empty
          className="py-8"
          data-testid="gateway-provider-empty"
          description="Install a connectivity provider in Settings → Extensions before exposing this machine remotely."
          title="No connectivity provider installed"
        />
      ) : null}

      {model.candidates.map(candidate => (
        <GatewayProviderRow
          candidate={candidate}
          isSubmitting={isSubmitting}
          key={candidate.name}
          onDisable={onDisable}
          onEnable={onEnable}
        />
      ))}

      {model.orphanedActivations.map(activation => (
        <ListingRow
          data-testid={`gateway-provider-orphan-${activation.name}-${activation.tier ?? "unknown"}`}
          interactive={false}
          key={`${activation.name}:${activation.tier ?? "unknown"}`}
        >
          <ListingRow.Icon>
            <Radio aria-hidden="true" className="size-3.5" />
          </ListingRow.Icon>
          <ListingRow.Main>
            <ListingRow.Title>
              <ListingRow.Name>{activation.name}</ListingRow.Name>
            </ListingRow.Title>
            <ListingRow.Meta>
              <span>
                This provider is no longer installed but remains enabled for the{" "}
                {activation.tier ? TIER_LABEL[activation.tier] : "unknown address"}.
              </span>
            </ListingRow.Meta>
          </ListingRow.Main>
          <ListingRow.Trail>
            <GatewayProviderHealthChip activation={activation} />
            <Button
              data-testid={`gateway-provider-orphan-${activation.name}-${activation.tier ?? "unknown"}-disable`}
              disabled={isSubmitting || activation.tier === null}
              onClick={() => {
                if (activation.tier) onDisable(activation.name, activation.tier);
              }}
              size="sm"
              type="button"
              variant="ghost"
            >
              Disable
            </Button>
          </ListingRow.Trail>
        </ListingRow>
      ))}
    </SettingsGroup>
  );
}

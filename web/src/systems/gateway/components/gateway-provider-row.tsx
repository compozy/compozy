import { Radio } from "lucide-react";

import { Button, ListingRow, MonoId } from "@compozy/ui";

import { TIER_LABEL } from "../lib/gateway-copy";
import { activationForTier, type GatewayProviderCandidate } from "../lib/gateway-provider-model";
import type { GatewayTier } from "../types";
import { GatewayProviderHealthChip } from "./gateway-provider-health-chip";

const TIERS: readonly GatewayTier[] = ["private", "public"];

export interface GatewayProviderRowProps {
  candidate: GatewayProviderCandidate;
  isSubmitting: boolean;
  onEnable: (candidate: GatewayProviderCandidate, tier: GatewayTier) => void;
  onDisable: (name: string, tier: GatewayTier) => void;
}

/**
 * One installed connectivity provider and its per-tier activation.
 *
 * Blockers come from the extension's own payload, and the actions that clear
 * them belong to the extensions surface — this row names them instead of
 * duplicating that flow.
 */
export function GatewayProviderRow({
  candidate,
  isSubmitting,
  onEnable,
  onDisable,
}: GatewayProviderRowProps) {
  const blocked = candidate.blockers.length > 0;

  return (
    <ListingRow data-testid={`gateway-provider-${candidate.name}`} interactive={false}>
      <ListingRow.Icon>
        <Radio className="size-3.5" />
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Title>
          <ListingRow.Name>{candidate.name}</ListingRow.Name>
        </ListingRow.Title>
        <ListingRow.Meta>
          <span>
            Installed from <MonoId preserveCase value={candidate.installSource} />
          </span>
        </ListingRow.Meta>
        {candidate.blockers.map(blocker => (
          <p
            className="mt-1 text-form-label text-muted"
            data-testid={`gateway-provider-${candidate.name}-blocker-${blocker.id}`}
            key={blocker.id}
          >
            {blocker.message}
          </p>
        ))}
      </ListingRow.Main>
      <ListingRow.Trail>
        {TIERS.map(tier => {
          const activation = activationForTier(candidate, tier);
          const active = activation?.desired === "enabled";
          return (
            <span className="flex items-center gap-1.5" key={tier}>
              {activation ? (
                <GatewayProviderHealthChip activation={activation} />
              ) : (
                <span className="text-form-label text-subtle">{TIER_LABEL[tier]}</span>
              )}
              <Button
                data-testid={`gateway-provider-${candidate.name}-${tier}-${active ? "disable" : "enable"}`}
                disabled={isSubmitting || (!active && blocked)}
                onClick={() =>
                  active ? onDisable(candidate.name, tier) : onEnable(candidate, tier)
                }
                size="sm"
                type="button"
                variant={active ? "ghost" : "neutral"}
              >
                {active ? "Disable" : `Use for ${TIER_LABEL[tier].toLowerCase()}`}
              </Button>
            </span>
          );
        })}
      </ListingRow.Trail>
    </ListingRow>
  );
}

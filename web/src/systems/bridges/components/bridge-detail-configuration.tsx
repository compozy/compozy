import { MetadataList, Section } from "@agh/ui";

import {
  describeBridgeDeliveryDefaults,
  describeBridgeDmPolicy,
  describeBridgeRoutingPolicy,
  formatBridgeDateTime,
} from "../lib/bridge-formatters";
import type { BridgeSummary } from "../types";

const TILE_CLASS =
  "grid-cols-1 items-start rounded-md border border-line bg-canvas-soft px-4 py-3 sm:grid-cols-[7.5rem_1fr] sm:items-center";
const TERM_CLASS = "text-muted";
const VALUE_CLASS = "min-w-0 break-words text-small-body text-fg";

export function BridgeDetailConfiguration({
  bridge,
  workspaceName,
}: {
  bridge: BridgeSummary;
  workspaceName?: string | null;
}) {
  return (
    <Section label="Configuration">
      <MetadataList className="grid gap-3 sm:grid-cols-2">
        <MetadataList.Row
          className={TILE_CLASS}
          label="Provider"
          termProps={{ className: TERM_CLASS }}
          valueProps={{ className: VALUE_CLASS }}
        >
          {bridge.platform} / {bridge.extension_name}
        </MetadataList.Row>
        <MetadataList.Row
          className={TILE_CLASS}
          label="Workspace"
          termProps={{ className: TERM_CLASS }}
          valueProps={{ className: VALUE_CLASS }}
        >
          {bridge.scope === "workspace"
            ? (workspaceName ?? bridge.workspace_id ?? "Unavailable")
            : "Global scope"}
        </MetadataList.Row>
        <MetadataList.Row
          className={TILE_CLASS}
          label="Routing policy"
          termProps={{ className: TERM_CLASS }}
          valueProps={{ className: VALUE_CLASS }}
        >
          {describeBridgeRoutingPolicy(bridge.routing_policy)}
        </MetadataList.Row>
        <MetadataList.Row
          className={TILE_CLASS}
          label="Delivery defaults"
          termProps={{ className: TERM_CLASS }}
          valueProps={{ className: VALUE_CLASS }}
        >
          {describeBridgeDeliveryDefaults(bridge.delivery_defaults)}
        </MetadataList.Row>
        <MetadataList.Row
          className={TILE_CLASS}
          label="DM policy"
          termProps={{ className: TERM_CLASS }}
          valueProps={{ className: VALUE_CLASS }}
        >
          {describeBridgeDmPolicy(bridge.dm_policy)}
        </MetadataList.Row>
        <MetadataList.Row
          className={TILE_CLASS}
          label="Created"
          termProps={{ className: TERM_CLASS }}
          valueProps={{ className: VALUE_CLASS }}
        >
          {formatBridgeDateTime(bridge.created_at)}
        </MetadataList.Row>
        <MetadataList.Row
          className={TILE_CLASS}
          label="Updated"
          termProps={{ className: TERM_CLASS }}
          valueProps={{ className: VALUE_CLASS }}
        >
          {formatBridgeDateTime(bridge.updated_at)}
        </MetadataList.Row>
      </MetadataList>
    </Section>
  );
}

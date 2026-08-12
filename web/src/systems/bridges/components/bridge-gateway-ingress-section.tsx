import { Section } from "@compozy/ui";

import type { BridgeSummary } from "../types";
import { GatewayIngressStatus } from "@/systems/gateway";

/**
 * Whether this bridge's inbound callbacks actually reach CompozyOS.
 *
 * Only bridges bound to the gateway carry `gateway_ingress`; a bridge fronted
 * by the operator's own proxy has none, and this section stays silent rather
 * than implying the gateway is in that path.
 */
export function BridgeGatewayIngressSection({ bridge }: { bridge: BridgeSummary }) {
  if (!bridge.gateway_ingress) return null;
  return (
    <Section label="Callback ingress">
      <GatewayIngressStatus
        data-testid="bridge-gateway-ingress"
        ingress={bridge.gateway_ingress}
        subject="this bridge"
      />
    </Section>
  );
}

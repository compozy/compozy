import { Metric, MetricGrid, Section } from "@agh/ui";

import { formatBridgeRelativeTime } from "../lib/bridge-formatters";
import type { BridgeHealth, BridgeRoute } from "../types";

interface BridgeMetrics {
  activeRoutes: number;
  authFailures: number;
  deliveryBacklog: number;
  deliveryDropped: number;
  deliveryFailures: number;
  lastDelivery: string;
}

function computeBridgeMetrics(
  health: BridgeHealth | undefined,
  routes: BridgeRoute[]
): BridgeMetrics {
  return {
    activeRoutes: health?.route_count ?? routes.length,
    authFailures: health?.auth_failures_total ?? 0,
    deliveryBacklog: health?.delivery_backlog ?? 0,
    deliveryDropped: health?.delivery_dropped_total ?? 0,
    deliveryFailures: health?.delivery_failures_total ?? 0,
    lastDelivery: formatBridgeRelativeTime(health?.last_success_at),
  };
}

export function BridgeDetailMetrics({
  health,
  routes,
}: {
  health: BridgeHealth | undefined;
  routes: BridgeRoute[];
}) {
  const metrics = computeBridgeMetrics(health, routes);

  return (
    <Section label="Delivery metrics">
      <MetricGrid columns={3}>
        <Metric
          data-testid="bridge-metric-delivery-backlog"
          label="Delivery backlog"
          subtext="currently queued"
          tone={metrics.deliveryBacklog > 0 ? "warning" : "default"}
          value={metrics.deliveryBacklog}
        />
        <Metric
          data-testid="bridge-metric-delivery-failures"
          label="Delivery failures"
          subtext="recorded total"
          tone={metrics.deliveryFailures > 0 ? "danger" : "default"}
          value={metrics.deliveryFailures}
        />
        <Metric
          data-testid="bridge-metric-delivery-dropped"
          label="Dropped deliveries"
          subtext="recorded total"
          tone={metrics.deliveryDropped > 0 ? "danger" : "default"}
          value={metrics.deliveryDropped}
        />
        <Metric
          data-testid="bridge-metric-auth-failures"
          label="Authentication failures"
          subtext="recorded total"
          tone={metrics.authFailures > 0 ? "danger" : "default"}
          value={metrics.authFailures}
        />
        <Metric
          data-testid="bridge-metric-last-delivery"
          label="Last delivery"
          subtext="most recent success"
          value={metrics.lastDelivery}
        />
        <Metric
          data-testid="bridge-metric-active-routes"
          label="Active routes"
          subtext="sessions mapped"
          value={metrics.activeRoutes}
        />
      </MetricGrid>
    </Section>
  );
}

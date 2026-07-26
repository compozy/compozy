import { Waypoints } from "lucide-react";

import {
  DataSurface,
  Empty,
  Eyebrow,
  Pill,
  Section,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@agh/ui";

import { describeBridgeRouteTarget, formatBridgeRelativeTime } from "../lib/bridge-formatters";
import type { BridgeRoute } from "../types";

export function BridgeEventStreamSection({
  isRoutesLoading,
  routes,
}: {
  isRoutesLoading: boolean;
  routes: BridgeRoute[];
}) {
  if (isRoutesLoading) {
    return (
      <Section label="Event stream">
        <DataSurface.Loading
          data-testid="bridge-routes-loading"
          label="Loading bridge routes"
          size="sm"
        />
      </Section>
    );
  }

  if (routes.length === 0) {
    return (
      <Section label="Event stream">
        <div data-testid="bridge-routes-empty">
          <Empty
            description="This bridge has not mapped any sessions yet. Check a target without sending or wait for the first inbound route to be claimed."
            icon={Waypoints}
            title="No routes"
          />
        </div>
      </Section>
    );
  }

  return (
    <Section label="Event stream" right={<Pill mono>{routes.length}</Pill>}>
      <div
        className="overflow-hidden rounded-md border border-line bg-canvas-soft"
        data-testid="bridge-routes-table"
      >
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>
                <Eyebrow>Status</Eyebrow>
              </TableHead>
              <TableHead>
                <Eyebrow>Agent</Eyebrow>
              </TableHead>
              <TableHead>
                <Eyebrow>Target</Eyebrow>
              </TableHead>
              <TableHead>
                <Eyebrow>Scope</Eyebrow>
              </TableHead>
              <TableHead>
                <Eyebrow>Last activity</Eyebrow>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {routes.map(route => (
              <TableRow
                data-testid={`bridge-route-${route.session_id}`}
                key={`${route.session_id}:${route.routing_key_hash}`}
              >
                <TableCell>
                  <div className="flex items-center gap-2">
                    <Pill.Dot tone="success" />
                    <Pill mono tone="success">
                      ACTIVE
                    </Pill>
                  </div>
                </TableCell>
                <TableCell>
                  <div className="min-w-0">
                    <div className="text-small-body text-fg">{route.agent_name}</div>
                    <div className="mt-1 break-all font-mono text-eyebrow text-subtle">
                      <Eyebrow className="mr-1">Session</Eyebrow>
                      <span>{route.session_id}</span>
                    </div>
                  </div>
                </TableCell>
                <TableCell className="font-mono text-xs text-muted">
                  {describeBridgeRouteTarget(route)}
                </TableCell>
                <TableCell>
                  <Pill mono tone={route.scope === "workspace" ? "info" : "neutral"}>
                    {route.scope}
                  </Pill>
                </TableCell>
                <TableCell className="font-mono text-xs text-subtle">
                  {formatBridgeRelativeTime(route.last_activity_at)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </Section>
  );
}

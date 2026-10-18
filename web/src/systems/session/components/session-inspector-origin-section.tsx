import { PropertyRow } from "@compozy/ui";

import {
  sessionOriginLedgerValue,
  sessionRouteFailure,
  sessionSeedView,
} from "../lib/session-origin";
import type { SessionPayload } from "../types";
import { SessionInspectorSection, SessionInspectorSectionHead } from "./session-inspector-section";

/**
 * Where a continued or forked session came from, on the inspection surface:
 * the kind and anchor, the seed the daemon used, and a bind-time
 * `route_not_found` in the daemon's own words. Roots, spawned children,
 * provenance and recovery sessions render nothing.
 */
export function SessionInspectorOriginSection({ session }: { session: SessionPayload }) {
  const origin = sessionOriginLedgerValue(session);
  if (origin === null) return null;
  const seed = sessionSeedView(session);
  const routeFailure = sessionRouteFailure(session);
  return (
    <SessionInspectorSection data-testid="session-inspector-origin">
      <SessionInspectorSectionHead>Origin</SessionInspectorSectionHead>
      <div className="flex flex-col">
        <PropertyRow data-testid="ledger-origin" label="Origin" mono>
          {origin}
        </PropertyRow>
        {seed ? (
          <PropertyRow
            data-testid="ledger-seed"
            label="Seed"
            mono
            valueTitle={seed.detail ?? seed.label}
          >
            {seed.label}
          </PropertyRow>
        ) : null}
        {routeFailure ? (
          <PropertyRow data-testid="ledger-route-failure" label="Route" valueTitle={routeFailure}>
            <span className="truncate text-danger">{routeFailure}</span>
          </PropertyRow>
        ) : null}
      </div>
    </SessionInspectorSection>
  );
}

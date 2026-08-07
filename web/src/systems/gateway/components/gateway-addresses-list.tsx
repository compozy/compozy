import { MonoId } from "@compozy/ui";

import type { GatewayAddressStatus } from "../types";

export interface GatewayAddressesListProps {
  addresses: readonly GatewayAddressStatus[];
  "data-testid"?: string;
}

/**
 * Only verified addresses appear here. The daemon marks an address `live` after
 * a challenge proves it reaches this listener, so an unverified provider claim
 * has nothing to render and the list stays empty rather than showing a URL that
 * may go nowhere.
 */
export function GatewayAddressesList({
  addresses,
  "data-testid": testId,
}: GatewayAddressesListProps) {
  if (addresses.length === 0) return null;
  return (
    <ul className="flex flex-col gap-1" data-testid={testId}>
      {addresses.map(address => (
        <li key={`${address.tier}:${address.address}`}>
          <MonoId copy copyLabel="Copy address" preserveCase value={address.address} />
        </li>
      ))}
    </ul>
  );
}

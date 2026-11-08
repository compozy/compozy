import type { MarketplaceCatalogResponse } from "../types";

type CatalogEnvelope = Pick<MarketplaceCatalogResponse, "error" | "error_class" | "sources">;

/**
 * The daemon's answer reports the catalog unreachable as a whole: it carries an error and none of
 * the enabled sources it lists answered (`ok`) or still holds cached rows. One degraded source
 * beside a healthy one is a partial failure, so an empty search or category stays an empty result.
 */
export function marketplaceCatalogUnreachable(
  envelope: CatalogEnvelope | undefined,
  hasItems: boolean
): Error | null {
  if (!envelope?.error_class || hasItems) return null;
  const answered = envelope.sources.some(source => source.state === "ok" || source.count > 0);
  return answered
    ? null
    : new Error(envelope.error || `Catalog unavailable (${envelope.error_class})`);
}

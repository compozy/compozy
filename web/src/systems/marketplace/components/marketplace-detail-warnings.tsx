import { TriangleAlert } from "lucide-react";

import type { MarketplaceCatalogEntryResponse } from "../types";
import { MarketplaceDetailSection } from "./marketplace-detail-shell";
import { MarketplaceTrustWarningList } from "./marketplace-trust-warning-list";

type TrustWarnings = NonNullable<
  NonNullable<MarketplaceCatalogEntryResponse["entry"]["trust"]>["warnings"]
>;

interface MarketplaceDetailWarningsProps {
  warnings: TrustWarnings | undefined;
}

/** Trust warnings from the catalog join, one severity-toned alert per finding. */
function MarketplaceDetailWarnings({ warnings }: MarketplaceDetailWarningsProps) {
  if (!warnings || warnings.length === 0) return null;
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-detail-warnings"
      icon={TriangleAlert}
      summary={`${warnings.length} ${warnings.length === 1 ? "warning" : "warnings"}`}
      title="Warnings"
    >
      <MarketplaceTrustWarningList className="p-3" items={warnings} />
    </MarketplaceDetailSection>
  );
}

export { MarketplaceDetailWarnings };
export type { MarketplaceDetailWarningsProps };

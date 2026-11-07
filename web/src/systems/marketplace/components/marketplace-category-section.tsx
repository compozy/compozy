import { Link } from "@tanstack/react-router";
import { ChevronRight } from "@compozy/ui/icons";

import { buttonVariants, Section } from "@compozy/ui";

import type { MarketplaceSection } from "../types";
import { MarketplaceCatalogRow, type MarketplaceRowInventory } from "./marketplace-catalog-row";
import { MARKETPLACE_SECTION_GRID_CLASS } from "./marketplace-classes";
import { MarketplaceGrid } from "./marketplace-grid";
import type { MarketplaceActionController } from "./use-marketplace-action-controller";

interface MarketplaceCategorySectionProps {
  section: MarketplaceSection;
  actions: MarketplaceActionController;
  inventory: MarketplaceRowInventory;
}

/**
 * One Browse section: compact rows (logo, name, action) under the daemon's label as the
 * heading, View all · N on the right (N is the daemon's full count, never the rows shown), then
 * up to the daemon's per-section cut of rows.
 * The section is a labelled region; View all names its category for assistive tech.
 */
function MarketplaceCategorySection({
  section,
  actions,
  inventory,
}: MarketplaceCategorySectionProps) {
  return (
    <Section
      aria-label={section.label}
      data-testid={`marketplace-category-${section.id}`}
      headClassName="flex-row items-center justify-between pb-0 @md/section:items-center"
      label={section.label}
      right={
        // The primitive's link look on a real link: navigation keeps the link role.
        <Link
          aria-label={`View all ${section.label}`}
          className={buttonVariants({ size: "sm", variant: "link" })}
          data-testid={`marketplace-view-all-${section.id}`}
          params={{ category: section.id }}
          to="/marketplace/categories/$category"
        >
          View all
          <span className="tabular-nums">· {section.total}</span>
          <ChevronRight aria-hidden="true" className="size-3" />
        </Link>
      }
      rightClassName="w-auto shrink-0 self-center"
    >
      <MarketplaceGrid
        className={MARKETPLACE_SECTION_GRID_CLASS}
        data-testid={`marketplace-grid-${section.id}`}
      >
        {section.items.map(entry => (
          <MarketplaceCatalogRow
            actions={actions}
            compact
            entry={entry}
            inventory={inventory}
            // Featured mixes categories, so only its rows keep the category word.
            listCategory={section.id === "featured" ? undefined : section.id}
            key={`${entry.source_ref ?? entry.source}:${entry.entry_id}`}
          />
        ))}
      </MarketplaceGrid>
    </Section>
  );
}

export { MarketplaceCategorySection };
export type { MarketplaceCategorySectionProps };

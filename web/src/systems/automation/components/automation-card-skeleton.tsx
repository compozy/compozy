import { CatalogCard, CatalogCardSkeleton, Skeleton, SkeletonText } from "@compozy/ui";

const CARD_SKELETONS = [
  { id: "automation-card-skeleton-1", title: "w-40", foot: "w-28" },
  { id: "automation-card-skeleton-2", title: "w-32", foot: "w-36" },
  { id: "automation-card-skeleton-3", title: "w-48", foot: "w-24" },
  { id: "automation-card-skeleton-4", title: "w-36", foot: "w-32" },
  { id: "automation-card-skeleton-5", title: "w-44", foot: "w-28" },
  { id: "automation-card-skeleton-6", title: "w-28", foot: "w-36" },
] as const;

/**
 * `count` placeholder cards with the card's own stack: glyph, name and caption, a sentence clamp,
 * and a foot with the time truth opposite the switch. The foot is that row's real height (a line
 * of text and a small switch), not the standard action button's.
 */
export function AutomationCardSkeleton({ count = 6 }: { count?: number }) {
  return (
    <>
      {CARD_SKELETONS.slice(0, count).map(card => (
        <CatalogCardSkeleton key={card.id}>
          <div className="flex min-w-0 flex-col gap-3">
            <div className="flex items-start gap-3">
              <Skeleton className="size-(--size-catalog-logo) shrink-0 rounded-sm" />
              <div className="flex min-w-0 flex-1 flex-col gap-1">
                <CatalogCard.Title>
                  <SkeletonText className={card.title} />
                </CatalogCard.Title>
                <CatalogCard.Meta>
                  <SkeletonText className="w-24" />
                </CatalogCard.Meta>
              </div>
            </div>
            <CatalogCard.Description className="line-clamp-3">
              <SkeletonText lines={2} />
            </CatalogCard.Description>
          </div>
          <CatalogCard.Actions className="justify-between">
            <span className="text-form-label">
              <SkeletonText className={card.foot} />
            </span>
            <Skeleton className="h-3.5 w-6 rounded-pill" />
          </CatalogCard.Actions>
        </CatalogCardSkeleton>
      ))}
    </>
  );
}

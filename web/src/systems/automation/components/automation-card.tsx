import { CatalogCard, CatalogCardSkeleton, Skeleton, SkeletonText, Time, cn } from "@compozy/ui";

import {
  automationCardFootLead,
  automationTimeStat,
  type AutomationView,
} from "../lib/automation-view";
import { AutomationRowSwitch } from "./automation-enable-switch";
import type { AutomationItemControls } from "./automation-row";
import { AutomationDetailLink } from "./automation-row";
import { AutomationSentenceText, AutomationStartGlyph } from "./automation-row-parts";
import { ProfileOwnerTag } from "@/systems/profiles";
import { WorkspaceBadge } from "@/systems/workspace";

const START_CAPTION = {
  schedule: "Scheduled",
  event: "On an event",
  webhook: "Webhook",
} as const;

function cardCaption(view: AutomationView): string {
  const parts: string[] = [START_CAPTION[view.start]];
  if (!view.enabled) parts.push("Off");
  if (view.source === "config") parts.push("From config");
  if (view.source === "package") parts.push("From package");
  return parts.join(" · ");
}

/** Card display: glyph, name, caption, 3-line sentence, foot with time truth and the switch. */
export function AutomationCard({
  view,
  controls,
}: {
  view: AutomationView;
  controls: AutomationItemControls;
}) {
  const stat = automationTimeStat(view);
  const footLead = automationCardFootLead(stat);
  return (
    <CatalogCard
      actionable
      data-automation-id={view.id}
      data-automation-kind={view.kind}
      data-testid={`automation-card-${view.kind}-${view.id}`}
    >
      <AutomationDetailLink
        aria-label={`Open ${view.name}`}
        className="flex min-w-0 flex-col gap-3"
        search={controls.detailSearch}
        view={view}
      >
        <div className="flex items-start gap-3">
          <CatalogCard.Logo>
            <AutomationStartGlyph start={view.start} />
          </CatalogCard.Logo>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <CatalogCard.Title className={cn(!view.enabled && "text-muted")}>
              {view.name}
            </CatalogCard.Title>
            <CatalogCard.Meta data-testid={`automation-card-caption-${view.id}`}>
              {view.project ? (
                <WorkspaceBadge
                  data-testid={`automation-project-${view.id}`}
                  identity={view.project}
                />
              ) : null}
              <span>{cardCaption(view)}</span>
              {view.owner ? (
                <ProfileOwnerTag data-testid={`automation-profile-${view.id}`} owner={view.owner} />
              ) : null}
            </CatalogCard.Meta>
          </div>
        </div>
        <CatalogCard.Description className="line-clamp-3">
          <AutomationSentenceText sentence={view.sentence} />
        </CatalogCard.Description>
      </AutomationDetailLink>
      <CatalogCard.Actions className="justify-between">
        <span
          className={cn("text-form-label", stat.value === null ? "text-faint" : "text-muted")}
          data-testid={`automation-card-foot-${view.id}`}
        >
          {footLead === null ? (
            "—"
          ) : stat.at ? (
            <>
              {footLead} <Time iso={stat.at} />
            </>
          ) : (
            footLead
          )}
        </span>
        <AutomationRowSwitch
          data-testid={`automation-switch-${view.id}`}
          disabled={controls.unavailable}
          enabled={view.enabled}
          name={view.name}
          onEnabledChange={enabled => controls.onToggleEnabled(view, enabled)}
          pending={controls.isTogglePending(view)}
          size="sm"
        />
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

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

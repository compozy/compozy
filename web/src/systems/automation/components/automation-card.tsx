import { CatalogCard, Time, cn } from "@compozy/ui";

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

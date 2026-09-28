import { Zap } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { CatalogCard } from "@compozy/ui";

import { formatPromptPreview } from "../lib/automation-formatters";
import { describeAutomationTarget, projectAutomationTarget } from "../lib/automation-target";
import { triggerEventLabel } from "../lib/trigger-sentence";
import type { AutomationTrigger } from "../types";
import { AutomationStateBadges } from "./automation-state-badges";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface AutomationTriggerCardProps {
  trigger: AutomationTrigger;
  /** The trigger's profile, supplied only in aggregate mode. */
  owner?: ProfileOwner;
}

/** Card presentation for a trigger in the Triggers catalog (cards view). */
function AutomationTriggerCard({ trigger, owner }: AutomationTriggerCardProps) {
  const target = projectAutomationTarget(trigger);
  const description =
    target.kind === "loop" ? describeAutomationTarget(target) : formatPromptPreview(target.prompt);

  return (
    <CatalogCard
      actionable
      data-automation-id={trigger.id}
      data-testid={`automation-card-${trigger.id}`}
    >
      <Link
        aria-label={`Open ${trigger.name}`}
        className="flex min-w-0 flex-col gap-3"
        params={{ triggerId: trigger.id }}
        to="/triggers/$triggerId"
      >
        <div className="flex items-start gap-3">
          <CatalogCard.Logo>
            <Zap className="size-4" />
          </CatalogCard.Logo>
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <CatalogCard.Title>{trigger.name}</CatalogCard.Title>
            <CatalogCard.Meta>
              <span>{triggerEventLabel(trigger)}</span>
              {owner ? (
                <ProfileOwnerTag data-testid={`automation-profile-${trigger.id}`} owner={owner} />
              ) : null}
            </CatalogCard.Meta>
          </div>
        </div>
        {description ? <CatalogCard.Description>{description}</CatalogCard.Description> : null}
      </Link>
      <CatalogCard.Actions className="justify-between">
        <span className="flex items-center gap-1.5">
          <AutomationStateBadges enabled={trigger.enabled} source={trigger.source} />
        </span>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export { AutomationTriggerCard };

import { Zap } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { ListingRow } from "@compozy/ui";

import { automationScopeLabel, formatPromptPreview } from "../lib/automation-formatters";
import { describeAutomationTarget, projectAutomationTarget } from "../lib/automation-target";
import { triggerEventLabel } from "../lib/trigger-sentence";
import type { AutomationTrigger } from "../types";
import { AutomationStateBadges } from "./automation-state-badges";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface AutomationTriggerRowProps {
  trigger: AutomationTrigger;
  /** The trigger's profile, supplied only in aggregate mode. */
  profileOwner?: ProfileOwner;
}

/** Row presentation for a trigger in the Triggers catalog (rows view). */
function AutomationTriggerRow({ trigger, profileOwner }: AutomationTriggerRowProps) {
  const target = projectAutomationTarget(trigger);
  const description =
    target.kind === "loop" ? describeAutomationTarget(target) : formatPromptPreview(target.prompt);

  return (
    <ListingRow data-automation-id={trigger.id} data-testid={`automation-item-${trigger.id}`}>
      <ListingRow.Link
        render={
          <Link
            aria-label={`Open ${trigger.name}`}
            params={{ triggerId: trigger.id }}
            to="/triggers/$triggerId"
          />
        }
      >
        <ListingRow.Icon>
          <Zap aria-hidden="true" className="size-4" />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{trigger.name}</ListingRow.Title>
            <AutomationStateBadges enabled={trigger.enabled} source={trigger.source} />
          </ListingRow.Name>
          {description ? <ListingRow.Description>{description}</ListingRow.Description> : null}
          <ListingRow.Meta>
            <span>{automationScopeLabel(trigger.scope)}</span>
            {profileOwner ? (
              <>
                <ListingRow.MetaDot />
                <ProfileOwnerTag
                  data-testid={`automation-profile-${trigger.id}`}
                  owner={profileOwner}
                />
              </>
            ) : null}
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <span
          className="truncate text-form-label text-muted"
          data-testid="automation-trigger-event"
        >
          {triggerEventLabel(trigger)}
        </span>
      </ListingRow.Trail>
    </ListingRow>
  );
}

export { AutomationTriggerRow };

import { Zap } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { ListingRow, Pill } from "@agh/ui";

import {
  automationScopeLabel,
  automationSourceLabel,
  automationSourceTone,
  automationStatusTone,
  formatPromptPreview,
} from "../lib/automation-formatters";
import { describeAutomationTarget, projectAutomationTarget } from "../lib/automation-target";
import type { AutomationTrigger } from "../types";

export interface AutomationTriggerRowProps {
  trigger: AutomationTrigger;
}

/** Row presentation for a trigger in the Triggers catalog (rows view). */
function AutomationTriggerRow({ trigger }: AutomationTriggerRowProps) {
  const enabledTone = automationStatusTone(trigger.enabled ? "enabled" : "disabled");
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
            <span className="flex shrink-0 items-center gap-1.5">
              <Pill.Dot tone={enabledTone} />
              <Pill mono size="xs" tone={enabledTone}>
                {trigger.enabled ? "ENABLED" : "DISABLED"}
              </Pill>
            </span>
            <Pill mono size="xs" tone={automationSourceTone(trigger.source)}>
              {automationSourceLabel(trigger.source)}
            </Pill>
          </ListingRow.Name>
          {description ? <ListingRow.Description>{description}</ListingRow.Description> : null}
          <ListingRow.Meta>
            <span>{automationScopeLabel(trigger.scope)}</span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail>
        <Pill mono size="sm" tone="info">
          {trigger.event}
        </Pill>
      </ListingRow.Trail>
    </ListingRow>
  );
}

export { AutomationTriggerRow };

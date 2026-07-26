import { Zap } from "lucide-react";
import { Link } from "@tanstack/react-router";

import { CatalogCard, Pill } from "@agh/ui";

import {
  automationSourceLabel,
  automationSourceTone,
  automationStatusTone,
  formatPromptPreview,
} from "../lib/automation-formatters";
import { describeAutomationTarget, projectAutomationTarget } from "../lib/automation-target";
import type { AutomationTrigger } from "../types";

export interface AutomationTriggerCardProps {
  trigger: AutomationTrigger;
}

/** Card presentation for a trigger in the Triggers catalog (cards view). */
function AutomationTriggerCard({ trigger }: AutomationTriggerCardProps) {
  const enabledTone = automationStatusTone(trigger.enabled ? "enabled" : "disabled");
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
              <span>{trigger.event}</span>
            </CatalogCard.Meta>
          </div>
        </div>
        {description ? <CatalogCard.Description>{description}</CatalogCard.Description> : null}
      </Link>
      <CatalogCard.Actions className="justify-between">
        <span className="flex items-center gap-1.5">
          <Pill.Dot tone={enabledTone} />
          <Pill mono size="sm" tone={automationSourceTone(trigger.source)}>
            {automationSourceLabel(trigger.source)}
          </Pill>
        </span>
        <Pill mono size="sm" tone="info">
          {trigger.event}
        </Pill>
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export { AutomationTriggerCard };

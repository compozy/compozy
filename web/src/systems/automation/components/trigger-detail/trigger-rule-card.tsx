import type { LucideIcon } from "lucide-react";
import { CircleDot, Database, Filter, Puzzle, Unplug, Webhook } from "lucide-react";
import type { ReactNode } from "react";

import { Eyebrow, Pill } from "@compozy/ui";

import {
  describeTriggerIf,
  describeTriggerWhen,
  triggerWebhookPath,
} from "../../lib/trigger-sentence";
import type { EventIconKey } from "../../lib/trigger-catalog";
import type { AutomationTrigger } from "../../types";
import { TriggerRuleThen } from "./trigger-rule-then";
import { TriggerValueBadge } from "./trigger-value-badge";
import { TriggerWebhookEndpoint } from "./trigger-webhook-endpoint";

const EVENT_ICONS: Record<EventIconKey, LucideIcon> = {
  "session-start": CircleDot,
  "session-stop": CircleDot,
  memory: Database,
  hook: Unplug,
  webhook: Webhook,
  extension: Puzzle,
};

function RuleRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-[56px_minmax(0,1fr)] gap-3 border-t border-line-soft px-3 py-3 first:border-t-0 md:grid-cols-[72px_minmax(0,1fr)] md:gap-3.5 md:px-4 md:py-3.5">
      <Eyebrow className="pt-0.5 text-subtle">{label}</Eyebrow>
      <div className="flex min-w-0 flex-col">{children}</div>
    </div>
  );
}

function RuleSubLine({ children }: { children: ReactNode }) {
  return <span className="mt-1.5 text-small-body leading-relaxed text-muted">{children}</span>;
}

interface TriggerRuleCardProps {
  trigger: AutomationTrigger;
  workspaceName: string | null;
  loopWorkspaceName: string | null;
}

/**
 * The When / If / Then block — a trigger is a rule, so it reads as one, top to
 * bottom, in the operator's language with the runtime's own strings kept as
 * quiet sub-lines rather than the headline.
 */
export function TriggerRuleCard({
  trigger,
  workspaceName,
  loopWorkspaceName,
}: TriggerRuleCardProps) {
  const when = describeTriggerWhen(trigger, workspaceName);
  const condition = describeTriggerIf(trigger);
  const WhenIcon = EVENT_ICONS[when.icon];
  const webhookPath = triggerWebhookPath(trigger);

  return (
    <div
      className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
      data-testid="trigger-rule-card"
    >
      <RuleRow label="When">
        <span className="flex flex-wrap items-center gap-2">
          <WhenIcon aria-hidden="true" className="size-3.5 text-subtle" strokeWidth={1.75} />
          <b className="text-modal-title font-medium text-fg-strong">{when.headline}</b>
          <Pill mono size="sm">
            {when.eventId}
          </Pill>
        </span>
        {when.sub ? <RuleSubLine>{when.sub}</RuleSubLine> : null}
        {webhookPath ? <TriggerWebhookEndpoint path={webhookPath} /> : null}
      </RuleRow>

      <RuleRow label="If">
        <span className="flex flex-wrap items-center gap-x-2 gap-y-1.5">
          <Filter aria-hidden="true" className="size-3.5 shrink-0 text-subtle" strokeWidth={1.75} />
          {condition.clauses.length === 0 ? (
            <span className="text-modal-title text-fg">Any event of this kind</span>
          ) : (
            condition.clauses.map(clause => (
              <span
                className="inline-flex items-center gap-1.5 text-modal-title text-fg"
                key={`${clause.lead}-${clause.value}`}
              >
                {clause.lead} <TriggerValueBadge value={clause.value} />
              </span>
            ))
          )}
        </span>
        {condition.note ? (
          <RuleSubLine>
            {condition.note.lead}{" "}
            {condition.note.paths.map((path, index) => (
              <span key={path}>
                {index > 0 ? <span aria-hidden="true"> · </span> : null}
                <span className="font-mono text-mono-id text-faint">{path}</span>
              </span>
            ))}
          </RuleSubLine>
        ) : null}
      </RuleRow>

      <RuleRow label="Then">
        <TriggerRuleThen loopWorkspaceName={loopWorkspaceName} trigger={trigger} />
      </RuleRow>
    </div>
  );
}

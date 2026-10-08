import type { LucideIcon } from "lucide-react";
import { CircleDot, CircleStop, Clock3, Filter, Puzzle, Unplug, Webhook } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { Eyebrow, cn } from "@compozy/ui";

import { describeScheduleStarts } from "../../lib/automation-detail";
import { isAutomationTrigger, type AutomationEntity } from "../../lib/automation-entity";
import {
  describeTriggerIf,
  describeTriggerWhen,
  triggerWebhookPath,
} from "../../lib/automation-rule";
import type { SentenceContext } from "../../lib/automation-sentence";
import type { EventIconKey } from "../../lib/trigger-catalog";
import type { AutomationView } from "../../lib/automation-view";
import type { AutomationJob, AutomationTrigger } from "../../types";
import { AutomationNextRuns } from "./automation-next-runs";
import { AutomationRuleDoes } from "./automation-rule-does";
import { AutomationValueBadge } from "./automation-value-badge";
import { AutomationWebhookEndpoint } from "./automation-webhook-endpoint";

const EVENT_ICONS: Record<EventIconKey, LucideIcon> = {
  "session-start": CircleDot,
  "session-stop": CircleStop,
  hook: Unplug,
  webhook: Webhook,
  extension: Puzzle,
};

type RuleRowProps = Omit<ComponentProps<"div">, "children"> & {
  label: string;
  children: ReactNode;
};

function RuleRow({ label, children, className, ...props }: RuleRowProps) {
  return (
    <div
      className={cn(
        "grid grid-cols-[56px_minmax(0,1fr)] gap-3 border-t border-line-soft px-3 py-3 first:border-t-0 md:grid-cols-[84px_minmax(0,1fr)] md:gap-3.5 md:px-4 md:py-3.5",
        className
      )}
      {...props}
    >
      <Eyebrow className="pt-0.5 text-subtle">{label}</Eyebrow>
      <div className="flex min-w-0 flex-col">{children}</div>
    </div>
  );
}

function RuleHeadline({ icon: Icon, children }: { icon: LucideIcon; children: ReactNode }) {
  return (
    <span className="flex flex-wrap items-center gap-2">
      <Icon aria-hidden="true" className="size-3.5 text-subtle" />
      <b className="text-form-input font-medium text-fg-strong">{children}</b>
    </span>
  );
}

function RuleSubLine({ children }: { children: ReactNode }) {
  return <span className="mt-1.5 text-eyebrow leading-normal text-subtle">{children}</span>;
}

function RuleCode({ children }: { children: ReactNode }) {
  return <code className="font-mono text-mono-id text-faint">{children}</code>;
}

function ScheduleStarts({ job, ctx }: { job: AutomationJob; ctx: SentenceContext }) {
  const starts = describeScheduleStarts(job, ctx);
  return (
    <>
      <RuleHeadline icon={Clock3}>{starts.headline}</RuleHeadline>
      <RuleSubLine>
        {starts.sub}
        {starts.expression ? (
          <>
            {" · "}
            <RuleCode>{starts.expression}</RuleCode>
          </>
        ) : null}
      </RuleSubLine>
      <AutomationNextRuns className="mt-2.5" runs={starts.nextRuns} />
    </>
  );
}

function TriggerStarts({
  trigger,
  workspaceName,
}: {
  trigger: AutomationTrigger;
  workspaceName: string | null;
}) {
  const when = describeTriggerWhen(trigger, workspaceName);
  const webhookPath = triggerWebhookPath(trigger);
  if (trigger.event === "webhook") {
    return (
      <>
        <RuleHeadline icon={Webhook}>{when.headline}</RuleHeadline>
        {webhookPath ? <AutomationWebhookEndpoint path={webhookPath} /> : null}
        {when.sub ? <RuleSubLine>{when.sub}</RuleSubLine> : null}
      </>
    );
  }
  return (
    <>
      <RuleHeadline icon={EVENT_ICONS[when.icon]}>{when.headline}</RuleHeadline>
      <RuleSubLine>
        {when.sub ? `${when.sub} · ` : null}
        <RuleCode>{when.eventId}</RuleCode>
      </RuleSubLine>
    </>
  );
}

function OnlyIfRow({ trigger }: { trigger: AutomationTrigger }) {
  const condition = describeTriggerIf(trigger);
  if (condition.clauses.length === 0) return null;
  return (
    <RuleRow data-testid="automation-rule-only-if" label="Only if">
      <span className="flex flex-wrap items-center gap-x-2 gap-y-1.5">
        <Filter aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
        {condition.clauses.map(clause => (
          <span
            className="inline-flex items-center gap-1.5 text-form-input font-medium text-fg-strong"
            key={`${clause.lead}-${clause.value}`}
          >
            {clause.lead} <AutomationValueBadge value={clause.value} />
          </span>
        ))}
      </span>
      {condition.note ? (
        <RuleSubLine>
          {condition.clauses.length > 1 ? "All must match · " : `${condition.note.lead} `}
          {condition.note.paths.map((path, index) => (
            <span key={path}>
              {index > 0 ? <span aria-hidden="true"> · </span> : null}
              <RuleCode>{path}</RuleCode>
            </span>
          ))}
        </RuleSubLine>
      ) : null}
    </RuleRow>
  );
}

interface AutomationRuleCardProps extends Omit<ComponentProps<"div">, "children"> {
  entity: AutomationEntity;
  view: AutomationView;
  sentenceContext: SentenceContext;
  loopWorkspaceName: string | null;
  loopMissing: boolean;
}

/**
 * "How it works": Starts / Only if / Does, top to bottom, in the operator's
 * language, with the runtime's own strings kept as faint sub-lines. Only the
 * Starts row changes with the start kind; Only if appears only with conditions.
 */
export function AutomationRuleCard({
  entity,
  view,
  sentenceContext,
  loopWorkspaceName,
  loopMissing,
  className,
  ...props
}: AutomationRuleCardProps) {
  return (
    <div
      className={cn("overflow-hidden rounded-lg bg-card shadow-card", className)}
      data-testid="automation-rule-card"
      {...props}
    >
      <RuleRow data-testid="automation-rule-starts" label="Starts">
        {isAutomationTrigger(entity) ? (
          <TriggerStarts trigger={entity} workspaceName={view.workspaceName ?? null} />
        ) : (
          <ScheduleStarts ctx={sentenceContext} job={entity} />
        )}
      </RuleRow>
      {isAutomationTrigger(entity) ? <OnlyIfRow trigger={entity} /> : null}
      <RuleRow data-testid="automation-rule-does" label="Does">
        <AutomationRuleDoes
          entity={entity}
          loopMissing={loopMissing}
          loopWorkspaceName={loopWorkspaceName}
          view={view}
        />
      </RuleRow>
    </div>
  );
}

import { Bot, Repeat2 } from "lucide-react";

import { CodeBlock, Eyebrow, KindChip, Pill, Section } from "@agh/ui";

import { automationTriggerToDraft } from "../lib/automation-drafts";
import { describeFireLimit, describeRetry } from "../lib/automation-formatters";
import type { AgentTargetProjection, LoopTargetProjection } from "../lib/automation-target";
import { buildTriggerPreview } from "../lib/trigger-preview";
import type { AutomationJob, AutomationTrigger } from "../types";
import { AutomationTargetDetails } from "./automation-target-details";
import { WebhookEndpointCard } from "./trigger-form/preview/webhook-endpoint-card";

export function TriggerHookSection({
  target,
  trigger,
}: {
  target: AgentTargetProjection | null;
  trigger: AutomationTrigger;
}) {
  const filters = Object.entries(trigger.filter ?? {});
  const webhook =
    trigger.event === "webhook"
      ? buildTriggerPreview(automationTriggerToDraft(trigger)).webhook
      : null;

  return (
    <Section label="Hook" right={<KindChip kind={trigger.event} />}>
      <div className="space-y-3 rounded-md border border-line bg-canvas-soft px-4 py-3">
        <div className="flex flex-wrap items-center gap-2">
          <Eyebrow className="text-muted">Event</Eyebrow>
          <Pill mono tone="info">
            {trigger.event}
          </Pill>
        </div>
        <div className="space-y-1.5">
          <Eyebrow className="text-muted">Filters</Eyebrow>
          {filters.length === 0 ? (
            <p className="text-xs text-subtle">No filters</p>
          ) : (
            <div className="flex flex-wrap items-center gap-1.5">
              {filters.map(([key, value]) => (
                <Pill mono key={`${key}=${value}`} tone="neutral">
                  {`${key}=${value}`}
                </Pill>
              ))}
            </div>
          )}
        </div>
        {trigger.event === "webhook" ? (
          <div className="grid gap-3 md:grid-cols-2">
            <div>
              <Eyebrow className="text-muted">Endpoint</Eyebrow>
              <p className="mt-1 font-mono text-small-body text-fg">
                {trigger.endpoint_slug ?? "--"}
              </p>
            </div>
            <div>
              <Eyebrow className="text-muted">Webhook id</Eyebrow>
              <p className="mt-1 font-mono text-small-body text-fg">{trigger.webhook_id ?? "--"}</p>
            </div>
          </div>
        ) : null}
        {target ? (
          <div className="flex flex-wrap items-center gap-2">
            <Bot aria-hidden="true" className="size-3 text-subtle" />
            <span className="text-small-body text-muted">Dispatches to</span>
            <Pill mono tone="neutral">
              {target.agentName}
            </Pill>
          </div>
        ) : null}
      </div>
      {webhook ? (
        <div className="mt-3">
          <WebhookEndpointCard curl={webhook.curl} url={webhook.url} />
        </div>
      ) : null}
    </Section>
  );
}

export function AutomationTargetSection({
  showInputMapping,
  target,
}: {
  showInputMapping: boolean;
  target: LoopTargetProjection;
}) {
  return (
    <Section
      label="Target"
      right={
        <Pill mono tone="accent">
          <Repeat2 aria-hidden="true" className="size-3" />
          LOOP
        </Pill>
      }
    >
      <div className="rounded-md border border-line bg-canvas-soft px-4 py-3">
        <AutomationTargetDetails showInputMapping={showInputMapping} target={target} />
      </div>
    </Section>
  );
}

export function PromptSection({ isTrigger, prompt }: { isTrigger: boolean; prompt: string }) {
  return (
    <Section
      label={isTrigger ? "Prompt template" : "Prompt"}
      right={
        isTrigger ? (
          <Pill mono tone="info">
            GO TEMPLATE
          </Pill>
        ) : undefined
      }
    >
      <CodeBlock code={prompt} copyable={false} showPrompt={false} />
    </Section>
  );
}

export function GovernanceSection({ item }: { item: AutomationJob | AutomationTrigger }) {
  return (
    <Section label="Governance">
      <div className="grid gap-2 rounded-md border border-line bg-canvas-soft px-4 py-3 md:grid-cols-2">
        <div>
          <Eyebrow className="text-muted">Retry</Eyebrow>
          <p className="mt-1 text-small-body text-muted">{describeRetry(item.retry)}</p>
        </div>
        <div>
          <Eyebrow className="text-muted">Fire limit</Eyebrow>
          <p className="mt-1 text-small-body text-muted">{describeFireLimit(item.fire_limit)}</p>
        </div>
      </div>
    </Section>
  );
}

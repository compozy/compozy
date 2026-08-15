import type { LucideIcon } from "lucide-react";
import { ChevronDown, Gauge, Globe, Hash, Info, Search } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  MonoId,
  PropertyRow,
  Time,
  cn,
} from "@compozy/ui";

import {
  describeTriggerFireLimit,
  describeTriggerRetry,
  summarizeTriggerReliability,
} from "../../lib/automation-formatters";
import { projectAutomationTarget } from "../../lib/automation-target";
import { triggerEventLabel } from "../../lib/trigger-sentence";
import type { AutomationTrigger } from "../../types";
import { GatewayIngressStatus, ingressReachabilityCopy } from "@/systems/gateway";

interface TriggerRailCardProps {
  icon: LucideIcon;
  title: string;
  summary?: ReactNode;
  defaultOpen?: boolean;
  children: ReactNode;
  bodyClassName?: string;
  "data-testid"?: string;
}

function TriggerRailCard({
  icon: Icon,
  title,
  summary,
  defaultOpen = false,
  children,
  bodyClassName,
  "data-testid": testId,
}: TriggerRailCardProps) {
  return (
    <Collapsible
      className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
      defaultOpen={defaultOpen}
    >
      <CollapsibleTrigger
        className={cn(
          "group/rail flex w-full items-center gap-2 px-3.5 py-3 text-left transition-colors duration-base ease-out",
          "hover:bg-row-hover focus-visible:shadow-focus-inset focus-visible:outline-none"
        )}
        data-testid={testId}
      >
        <Icon aria-hidden="true" className="size-3.5 shrink-0 text-subtle" strokeWidth={1.75} />
        <span className="shrink-0 text-form-label font-medium text-fg-strong">{title}</span>
        <span className="min-w-0 flex-1 truncate text-right text-badge text-faint">{summary}</span>
        <ChevronDown
          aria-hidden="true"
          className="size-3 shrink-0 -rotate-90 text-subtle transition-transform duration-base ease-out group-data-panel-open/rail:rotate-0"
          strokeWidth={1.75}
        />
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div className={cn("border-t border-line-soft px-3.5 py-1.5", bodyClassName)}>
          {children}
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

const SOURCE_COPY: Record<AutomationTrigger["source"], string> = {
  dynamic: "You created this",
  config: "From configuration files",
  package: "From an installed package",
};

interface TriggerDetailRailProps {
  trigger: AutomationTrigger;
  loopWorkspaceName: string | null;
  onInspect: () => void;
}

/**
 * The facts rail: what this trigger is, where it can be reached, how hard it
 * tries, and who it is — four quiet cards, only the first two open by default,
 * with the raw-enum read one click away under them.
 */
export function TriggerDetailRail({
  trigger,
  loopWorkspaceName,
  onInspect,
}: TriggerDetailRailProps) {
  const target = projectAutomationTarget(trigger);
  const scopeLabel = trigger.scope === "workspace" ? "This workspace" : "Global";
  const eventLabel = triggerEventLabel(trigger);
  const isWebhook = trigger.event === "webhook";
  const kindLabel = target.kind === "loop" ? "Loop" : isWebhook ? "Webhook" : eventLabel;

  return (
    <aside className="flex flex-col gap-3" data-testid="trigger-detail-rail">
      <TriggerRailCard
        data-testid="trigger-rail-properties"
        defaultOpen
        icon={Info}
        summary={`${scopeLabel} · ${kindLabel}`}
        title="Properties"
      >
        <PropertyRow label="Scope">{scopeLabel}</PropertyRow>
        <PropertyRow label="Event">{eventLabel}</PropertyRow>
        <PropertyRow label="Target">
          {target.kind === "loop" ? `Loop · ${target.loopName}` : `Agent · ${target.agentName}`}
        </PropertyRow>
        {target.kind === "loop" ? (
          <PropertyRow label="Loop workspace">
            {loopWorkspaceName ?? target.workspaceId}
          </PropertyRow>
        ) : null}
        {isWebhook && trigger.endpoint_slug ? (
          <PropertyRow label="Endpoint" mono>
            {trigger.endpoint_slug}
          </PropertyRow>
        ) : null}
        {isWebhook && trigger.webhook_id ? (
          <PropertyRow label="Webhook id">
            <MonoId copy copyLabel="Copy webhook id" value={trigger.webhook_id} />
          </PropertyRow>
        ) : null}
        {isWebhook ? (
          <PropertyRow label="Signing secret">
            {trigger.webhook_secret_present ? "Set" : "Not set"}
          </PropertyRow>
        ) : null}
        <PropertyRow label="Source">{SOURCE_COPY[trigger.source]}</PropertyRow>
      </TriggerRailCard>

      {isWebhook ? (
        <TriggerRailCard
          bodyClassName="py-3"
          data-testid="trigger-rail-public-delivery"
          defaultOpen
          icon={Globe}
          summary={
            trigger.ingress ? ingressReachabilityCopy(trigger.ingress.reachability).label : "Off"
          }
          title="Public delivery"
        >
          <GatewayIngressStatus
            data-testid="automation-trigger-ingress"
            ingress={trigger.ingress}
            subject="this trigger"
          />
        </TriggerRailCard>
      ) : null}

      <TriggerRailCard
        data-testid="trigger-rail-reliability"
        icon={Gauge}
        summary={summarizeTriggerReliability(trigger.retry, trigger.fire_limit)}
        title="Reliability"
      >
        <PropertyRow label="Retry">{describeTriggerRetry(trigger.retry)}</PropertyRow>
        <PropertyRow label="At most">{describeTriggerFireLimit(trigger.fire_limit)}</PropertyRow>
      </TriggerRailCard>

      <TriggerRailCard
        data-testid="trigger-rail-identity"
        icon={Hash}
        summary={<Time iso={trigger.created_at} />}
        title="Identity"
      >
        <PropertyRow label="Created">
          <Time iso={trigger.created_at} />
        </PropertyRow>
        <PropertyRow label="Trigger id">
          <MonoId copy copyLabel="Copy trigger id" value={trigger.id} />
        </PropertyRow>
      </TriggerRailCard>

      <div className="flex items-center justify-between gap-2 px-1 pt-0.5">
        <Button
          data-testid="trigger-inspect-btn"
          onClick={onInspect}
          size="xs"
          type="button"
          variant="ghost"
        >
          <Search className="size-3" />
          Inspect
        </Button>
        <span className="truncate font-mono text-badge text-faint">
          compozy automation triggers get
        </span>
      </div>
    </aside>
  );
}

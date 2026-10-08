import type { LucideIcon } from "lucide-react";
import { CalendarClock, Globe, Hash, Info, KeyRound, Search, ShieldCheck } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { Button, MonoId, PropertyRow, StatusDot, Time, cn } from "@compozy/ui";

import {
  automationCliHint,
  automationDoesWord,
  automationStartWord,
  describeScheduleStarts,
  isAutomationTrigger,
  type AutomationEntity,
} from "../../lib/automation-detail";
import {
  catchUpPolicyLabel,
  describeFireLimit,
  describeRetryPlain,
} from "../../lib/automation-formatters";
import { triggerEventLabel } from "../../lib/automation-rule";
import type { SentenceContext } from "../../lib/automation-sentence";
import { projectAutomationTarget } from "../../lib/automation-target";
import { automationLocationLabel, type AutomationView } from "../../lib/automation-view";
import type { AutomationJob, AutomationTrigger } from "../../types";
import { ingressReachabilityCopy } from "@/systems/gateway";

interface RailSectionProps extends Omit<ComponentProps<"section">, "title"> {
  icon: LucideIcon;
  title: string;
  children: ReactNode;
}

/** One band of the one-card rail; bands are separated by hairlines (patterns §03). */
function RailSection({ icon: Icon, title, children, className, ...props }: RailSectionProps) {
  return (
    <section
      className={cn("border-t border-line-soft px-4 py-3.5 first:border-t-0", className)}
      {...props}
    >
      <h3 className="mb-2 flex items-center gap-1.5 text-eyebrow text-muted">
        <Icon aria-hidden="true" className="size-3 text-muted" />
        {title}
      </h3>
      {children}
    </section>
  );
}

const SOURCE_COPY = {
  dynamic: "You created this",
  config: "From configuration files",
  package: "From an installed package",
} as const satisfies Record<AutomationView["source"], string>;

function DetailsSection({
  entity,
  view,
  loopWorkspaceName,
}: {
  entity: AutomationEntity;
  view: AutomationView;
  loopWorkspaceName: string | null;
}) {
  const target = projectAutomationTarget(entity);
  const starts = isAutomationTrigger(entity)
    ? entity.event === "webhook"
      ? "Another app calls a link"
      : triggerEventLabel(entity)
    : automationStartWord("schedule");
  return (
    <RailSection data-testid="automation-rail-details" icon={Info} title="Details">
      <PropertyRow label="Starts">{starts}</PropertyRow>
      <PropertyRow label="Does">{automationDoesWord(view.does)}</PropertyRow>
      {view.does === "agent" && target.kind === "agent" ? (
        <PropertyRow label="Agent">{target.agentName}</PropertyRow>
      ) : null}
      {target.kind === "loop" ? (
        <>
          <PropertyRow label="Loop">{target.loopName}</PropertyRow>
          <PropertyRow label="Loop project">{loopWorkspaceName ?? target.workspaceId}</PropertyRow>
        </>
      ) : null}
      <PropertyRow label="Location">{automationLocationLabel(view)}</PropertyRow>
      <PropertyRow label="Source">{SOURCE_COPY[view.source]}</PropertyRow>
    </RailSection>
  );
}

function ScheduleSection({
  job,
  sentenceContext,
  lastRanAt,
}: {
  job: AutomationJob;
  sentenceContext: SentenceContext;
  lastRanAt: string | null;
}) {
  const starts = describeScheduleStarts(job, sentenceContext);
  const policy = job.scheduler?.catch_up_policy ?? job.schedule?.catch_up_policy;
  return (
    <RailSection data-testid="automation-rail-schedule" icon={CalendarClock} title="Schedule">
      <PropertyRow label="Repeats">{starts.headline}</PropertyRow>
      <PropertyRow label="Time zone">{sentenceContext.timeZone?.trim() || "UTC"}</PropertyRow>
      <PropertyRow label="Missed runs">{catchUpPolicyLabel(policy)}</PropertyRow>
      <PropertyRow label="Last run">{lastRanAt ? <Time iso={lastRanAt} /> : "—"}</PropertyRow>
    </RailSection>
  );
}

function PublicLinkSections({ trigger }: { trigger: AutomationTrigger }) {
  const ingress = trigger.ingress;
  const reachability = ingress?.reachability ?? "off";
  const copy = ingressReachabilityCopy(reachability);
  const broken = reachability === "broken";
  const url = ingress?.url?.trim();
  return (
    <>
      <RailSection data-testid="automation-rail-public-link" icon={Globe} title="Public link">
        <PropertyRow label="Status" valueTitle={copy.detail}>
          <StatusDot
            size="sm"
            tone={broken ? "danger" : reachability === "live" ? "success" : "faint"}
          />
          <span className={broken ? "text-danger" : undefined}>{copy.label}</span>
        </PropertyRow>
        <PropertyRow label="Confirmed">
          {ingress?.confirmed_at ? <Time iso={ingress.confirmed_at} /> : "—"}
        </PropertyRow>
        <PropertyRow label="Public address" mono valueTitle={url}>
          {url || "—"}
        </PropertyRow>
      </RailSection>
      <RailSection data-testid="automation-rail-security" icon={KeyRound} title="Security">
        <PropertyRow label="Signing secret">
          {trigger.webhook_secret_present ? "Set" : "Not set"}
        </PropertyRow>
        {trigger.webhook_id ? (
          <PropertyRow label="Webhook id">
            <MonoId copy copyLabel="Copy webhook id" value={trigger.webhook_id} />
          </PropertyRow>
        ) : null}
      </RailSection>
    </>
  );
}

interface AutomationRailProps extends Omit<ComponentProps<"aside">, "children"> {
  entity: AutomationEntity;
  view: AutomationView;
  sentenceContext: SentenceContext;
  loopWorkspaceName: string | null;
  lastRanAt: string | null;
  onInspect: () => void;
}

/**
 * The facts rail as one card: what it is, when it runs (or where it can be
 * reached), how hard it tries and who it is — with the machine read one click
 * away in Inspect and the CLI hint beside it.
 */
export function AutomationRail({
  entity,
  view,
  sentenceContext,
  loopWorkspaceName,
  lastRanAt,
  onInspect,
  className,
  ...props
}: AutomationRailProps) {
  return (
    <aside
      className={cn("flex flex-col gap-2", className)}
      data-testid="automation-rail"
      {...props}
    >
      <div className="overflow-hidden rounded-lg bg-card shadow-card">
        {isAutomationTrigger(entity) && entity.event === "webhook" ? (
          <PublicLinkSections trigger={entity} />
        ) : null}
        <DetailsSection entity={entity} loopWorkspaceName={loopWorkspaceName} view={view} />
        {isAutomationTrigger(entity) ? null : (
          <ScheduleSection job={entity} lastRanAt={lastRanAt} sentenceContext={sentenceContext} />
        )}
        <RailSection
          data-testid="automation-rail-reliability"
          icon={ShieldCheck}
          title="Reliability"
        >
          <PropertyRow label="Retries">{describeRetryPlain(entity.retry)}</PropertyRow>
          <PropertyRow label="Run limit">{describeFireLimit(entity.fire_limit)}</PropertyRow>
        </RailSection>
        <RailSection data-testid="automation-rail-identity" icon={Hash} title="Identity">
          <PropertyRow label="Created">
            <Time iso={entity.created_at} />
          </PropertyRow>
          <PropertyRow label="Id">
            <MonoId copy copyLabel="Copy automation id" value={entity.id} />
          </PropertyRow>
        </RailSection>
      </div>
      <div className="flex items-center justify-between gap-2 px-1 pt-0.5">
        <Button
          data-testid="automation-inspect-btn"
          onClick={onInspect}
          size="xs"
          type="button"
          variant="ghost"
        >
          <Search />
          Inspect
        </Button>
        <span
          className="truncate font-mono text-badge text-faint"
          data-testid="automation-rail-cli"
        >
          {automationCliHint(entity)}
        </span>
      </div>
    </aside>
  );
}

/** S4: the rich header and the summary-row hub. */
import {
  Brain,
  ChevronRight,
  Clock,
  Fingerprint,
  HeartPulse,
  Lock,
  MapPin,
  MessagesSquare,
  Sparkles,
  Trash2,
  Wrench,
  type LucideIcon,
} from "lucide-react";
import type { ReactNode } from "react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  KindIcon,
  Pill,
  cn,
  providerKindIconRegistry,
} from "@compozy/ui";

import { setParam } from "@compozy/proto";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import { KindPill } from "../persistent-agents-shared/agent-page";
import { SCOPE_LABEL, type DraftAgent } from "../persistent-agents-shared/fixtures";

function SummaryRow({
  icon: Icon,
  title,
  summary,
  to,
  owner,
  danger = false,
}: {
  icon: LucideIcon;
  title: string;
  summary: ReactNode;
  to?: string;
  owner?: string;
  danger?: boolean;
}) {
  return (
    <button
      className="group flex min-h-13 w-full items-center gap-3 px-4 py-2.5 text-left outline-none transition-colors hover:bg-hover focus-visible:shadow-focus-inset"
      onClick={() => (to ? setParam("state", to) : undefined)}
      type="button"
    >
      <span
        className={cn(
          "grid size-7 shrink-0 place-items-center rounded-icon-well bg-surface-2",
          danger ? "text-danger" : "text-muted"
        )}
      >
        <Icon aria-hidden="true" className="size-3.5" />
      </span>
      <span className="w-36 shrink-0 text-small-body font-medium text-fg">{title}</span>
      <span className="flex min-w-0 flex-1 items-center gap-2 text-small-body text-muted">
        {summary}
      </span>
      {owner ? (
        <Pill form="plain" size="sm">
          <Lock aria-hidden="true" />
          {owner}
        </Pill>
      ) : null}
      <ChevronRight aria-hidden="true" className="size-4 shrink-0 text-faint" />
    </button>
  );
}

function RowGroup({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-col divide-y divide-line-soft overflow-hidden rounded-lg bg-card shadow-card">
      {children}
    </div>
  );
}

function ProfileHeader({ agent, editable = true }: { agent: DraftAgent; editable?: boolean }) {
  return (
    <header className="flex items-center gap-4">
      <AgentAvatar
        appearance={agent.appearance}
        label={agent.label}
        provider={agent.provider}
        quietIdle
        size={64}
      />
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex items-center gap-2">
          <h1 className="text-detail-h1 font-medium text-fg">{agent.label}</h1>
          <KindPill agent={agent} />
        </div>
        <p className="text-small-body text-muted">{agent.description}</p>
        <p className="font-mono text-mono-id text-subtle">{agent.name}</p>
      </div>
      {editable ? null : (
        <Pill size="sm" tone="neutral">
          From {agent.owner}
        </Pill>
      )}
    </header>
  );
}

function PersistentRows({ agent }: { agent: DraftAgent }) {
  return (
    <RowGroup>
      <SummaryRow
        icon={MapPin}
        summary={
          agent.scopes.length > 0 ? agent.scopes.map(SCOPE_LABEL).join(" · ") : "Not running"
        }
        title="Where it runs"
        to="where-active"
      />
      <SummaryRow
        icon={HeartPulse}
        summary={
          <>
            Every 30 min, 09:00 to 19:00
            <span className="text-subtle">· next check in 18m</span>
          </>
        }
        title="Heartbeat"
      />
      <SummaryRow
        icon={Brain}
        summary={
          <>
            14 notes
            <span className="block h-1 w-20 overflow-hidden rounded-pill bg-surface-2">
              <span className="block h-full w-1/4 rounded-pill bg-fg-2" />
            </span>
          </>
        }
        title="Memory"
      />
    </RowGroup>
  );
}

export function ProfileHub({
  agent,
  readOnly = false,
  invalid,
}: {
  agent: DraftAgent;
  readOnly?: boolean;
  invalid?: ReactNode;
}) {
  const owner = readOnly ? agent.owner : undefined;
  return (
    <div className="flex flex-col gap-5 py-2">
      <ProfileHeader agent={agent} editable={!readOnly} />
      {invalid}
      <RowGroup>
        <SummaryRow
          icon={Fingerprint}
          owner={owner}
          summary={`${agent.label}, generated avatar`}
          title="Identity"
          to={readOnly ? "identity-extension" : "identity"}
        />
        <SummaryRow
          icon={Sparkles}
          owner={owner}
          summary="Direct, flags what blocks the work"
          title="Persona"
        />
        <SummaryRow
          icon={Clock}
          owner={owner}
          summary={
            <>
              <KindIcon kind={agent.provider} registry={providerKindIconRegistry} size="xs" />
              <span className="font-mono text-mono-id">{agent.model}</span>
            </>
          }
          title="Runtime"
          to="deep-link"
        />
        <SummaryRow
          icon={Wrench}
          owner={owner}
          summary="12 tools, this workspace"
          title="Tools and access"
        />
      </RowGroup>
      {agent.kind === "persistent" ? <PersistentRows agent={agent} /> : null}
      <RowGroup>
        <SummaryRow
          icon={MessagesSquare}
          summary={`${agent.active} active, 128 in history`}
          title="Sessions"
        />
        <SummaryRow
          danger
          icon={Trash2}
          summary={
            agent.kind === "persistent"
              ? "Sessions stay in history. Memory is retired, not erased."
              : "Sessions stay in history."
          }
          title="Delete agent"
        />
      </RowGroup>
    </div>
  );
}

export function InvalidBanner() {
  return (
    <Alert variant="warning">
      <AlertTitle>This agent can't run</AlertTitle>
      <AlertDescription>
        AGENT.md line 6: kind must be persistent or ephemeral, found always-on.
      </AlertDescription>
    </Alert>
  );
}

export function NewSessionAction() {
  return (
    <Button size="sm" type="button" variant="secondary">
      New session
    </Button>
  );
}

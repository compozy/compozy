import { Link } from "@tanstack/react-router";

import { CatalogCard, KindIcon, Pill, StateGlyph, providerKindIconRegistry } from "@compozy/ui";

import { formatCategoryMetaSegment, type AgentFleetRowModel } from "../lib/agent-fleet-projection";
import { WorkspaceBadge } from "@/systems/workspace";
import { AgentFleetNewSessionButton } from "./agent-fleet-new-session-button";

export interface AgentFleetCardProps {
  row: AgentFleetRowModel;
  newSessionDisabled?: boolean;
  onNewSession: (agentName: string) => void;
  /** All projects: opens the agent in place, bound to its project, instead of a route link. */
  onOpen?: (row: AgentFleetRowModel) => void;
}

const LINK_CLASS =
  "flex min-w-0 flex-col gap-3 text-left outline-none focus-visible:shadow-focus-inset";

function AgentFleetCard({
  row,
  newSessionDisabled = false,
  onNewSession,
  onOpen,
}: AgentFleetCardProps) {
  const { agent, signals, ariaLabel, hasDiagnostics, sessionsAvailable, cardOrigin } = row;
  const category = formatCategoryMetaSegment(agent.category_path);
  const model = agent.model?.trim() || null;
  const body = (
    <div className="flex min-w-0 items-start gap-3">
      <CatalogCard.Logo>
        <KindIcon
          className="size-4"
          kind={agent.provider}
          registry={providerKindIconRegistry}
          size="sm"
          tone="default"
        />
      </CatalogCard.Logo>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <div className="flex min-w-0 items-center gap-2">
          <CatalogCard.Title className="min-w-0 flex-1">{agent.name}</CatalogCard.Title>
          {row.project ? null : (
            <Pill size="xs" tone="neutral" data-testid={`agent-fleet-origin-${agent.name}`}>
              {cardOrigin}
            </Pill>
          )}
        </div>
        <CatalogCard.Meta data-testid={`agent-fleet-card-meta-${agent.name}`}>
          {row.project ? <WorkspaceBadge identity={row.project} /> : null}
          {category ? <span>{category}</span> : null}
          {model ? <span>{model}</span> : null}
        </CatalogCard.Meta>
      </div>
    </div>
  );

  return (
    <CatalogCard actionable data-agent={agent.name} data-testid={`agent-fleet-card-${agent.name}`}>
      {onOpen ? (
        <button
          aria-label={ariaLabel}
          className={LINK_CLASS}
          data-testid={`agent-fleet-card-link-${agent.name}`}
          onClick={() => onOpen(row)}
          type="button"
        >
          {body}
        </button>
      ) : (
        <Link
          aria-label={ariaLabel}
          className={LINK_CLASS}
          data-testid={`agent-fleet-card-link-${agent.name}`}
          params={{ name: agent.name }}
          to="/agents/$name"
        >
          {body}
        </Link>
      )}
      <CatalogCard.Actions className="justify-between">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          {sessionsAvailable && signals ? (
            <Pill form="plain" data-testid={`agent-fleet-status-${agent.name}`}>
              <StateGlyph state={signals.status === "active" ? "running" : "idle"} size="sm" />
              {signals.status === "active" ? "Active" : "Idle"}
            </Pill>
          ) : null}
          {hasDiagnostics ? (
            <Pill tone="warning" size="sm" data-testid={`agent-fleet-invalid-${agent.name}`}>
              Invalid
            </Pill>
          ) : null}
        </div>
        <AgentFleetNewSessionButton
          agentName={agent.name}
          disabled={newSessionDisabled}
          onNewSession={onNewSession}
        />
      </CatalogCard.Actions>
    </CatalogCard>
  );
}

export { AgentFleetCard };

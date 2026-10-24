import { Fragment } from "react";
import { Link } from "@tanstack/react-router";

import { KindIcon, ListingRow, Pill, StateGlyph, providerKindIconRegistry } from "@compozy/ui";

import { formatCategoryMetaSegment, type AgentFleetRowModel } from "../lib/agent-fleet-projection";
import { WorkspaceBadge } from "@/systems/workspace";
import { AgentFleetNewSessionButton } from "./agent-fleet-new-session-button";

export interface AgentFleetRowProps {
  row: AgentFleetRowModel;
  newSessionDisabled?: boolean;
  onNewSession: (agentName: string) => void;
  /** All projects: opens the agent in place, bound to its project, instead of a route link. */
  onOpen?: (row: AgentFleetRowModel) => void;
}

function agentFleetMetaSegments(
  agent: AgentFleetRowModel["agent"]
): Array<{ key: "category" | "model"; value: string }> {
  // The provider already shows as the row icon, so the meta line skips it.
  const segments: Array<{ key: "category" | "model"; value: string }> = [];
  const category = formatCategoryMetaSegment(agent.category_path);
  if (category) segments.push({ key: "category", value: category });
  if (agent.model?.trim()) segments.push({ key: "model", value: agent.model.trim() });
  return segments;
}

function AgentFleetRow({
  row,
  newSessionDisabled = false,
  onNewSession,
  onOpen,
}: AgentFleetRowProps) {
  const { agent, signals, ariaLabel, hasDiagnostics, sessionsAvailable, cardOrigin } = row;
  const metaSegments = agentFleetMetaSegments(agent);

  return (
    <ListingRow data-agent={agent.name} data-testid={`agent-fleet-row-${agent.name}`}>
      <ListingRow.Link
        render={
          onOpen ? (
            <button
              aria-label={ariaLabel}
              data-testid={`agent-fleet-row-link-${agent.name}`}
              onClick={() => onOpen(row)}
              type="button"
            />
          ) : (
            <Link
              to="/agents/$name"
              params={{ name: agent.name }}
              aria-label={ariaLabel}
              data-testid={`agent-fleet-row-link-${agent.name}`}
            />
          )
        }
      >
        <ListingRow.Icon>
          <KindIcon
            className="size-4"
            kind={agent.provider}
            registry={providerKindIconRegistry}
            size="sm"
            tone="default"
          />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{agent.name}</ListingRow.Title>
            {row.project ? null : (
              <Pill size="xs" tone="neutral" data-testid={`agent-fleet-origin-${agent.name}`}>
                {cardOrigin}
              </Pill>
            )}
          </ListingRow.Name>
          {metaSegments.length > 0 || row.project ? (
            <ListingRow.Meta data-testid={`agent-fleet-meta-${agent.name}`}>
              {row.project ? (
                <WorkspaceBadge
                  data-testid={`agent-fleet-project-${row.key}`}
                  identity={row.project}
                />
              ) : null}
              {metaSegments.map((segment, index) => (
                <Fragment key={segment.key}>
                  {index > 0 || row.project ? <ListingRow.MetaDot /> : null}
                  <span>{segment.value}</span>
                </Fragment>
              ))}
            </ListingRow.Meta>
          ) : null}
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail className="gap-3">
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
        <AgentFleetNewSessionButton
          agentName={agent.name}
          disabled={newSessionDisabled}
          onNewSession={onNewSession}
        />
      </ListingRow.Trail>
    </ListingRow>
  );
}

export { AgentFleetRow };

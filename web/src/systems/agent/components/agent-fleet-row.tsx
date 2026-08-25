import { Fragment } from "react";
import { Link } from "@tanstack/react-router";

import { KindIcon, ListingRow, Pill, providerKindIconRegistry } from "@compozy/ui";

import { formatCategoryMetaSegment, type AgentFleetRowModel } from "../lib/agent-fleet-projection";
import { AgentFleetNewSessionButton } from "./agent-fleet-new-session-button";
import { AgentLayerProvenance } from "./agent-layer-provenance";

export interface AgentFleetRowProps {
  row: AgentFleetRowModel;
  newSessionDisabled?: boolean;
  onNewSession: (agentName: string) => void;
}

function agentFleetMetaSegments(
  agent: AgentFleetRowModel["agent"]
): Array<{ key: "category" | "provider" | "model"; value: string }> {
  const segments: Array<{ key: "category" | "provider" | "model"; value: string }> = [];
  const category = formatCategoryMetaSegment(agent.category_path);
  if (category) segments.push({ key: "category", value: category });
  if (agent.provider?.trim()) segments.push({ key: "provider", value: agent.provider.trim() });
  if (agent.model?.trim()) segments.push({ key: "model", value: agent.model.trim() });
  return segments;
}

function AgentFleetRow({ row, newSessionDisabled = false, onNewSession }: AgentFleetRowProps) {
  const { agent, signals, ariaLabel, hasDiagnostics, sessionsAvailable, cardOrigin } = row;
  const metaSegments = agentFleetMetaSegments(agent);

  return (
    <ListingRow data-agent={agent.name} data-testid={`agent-fleet-row-${agent.name}`}>
      <ListingRow.Link
        render={
          <Link
            to="/agents/$name"
            params={{ name: agent.name }}
            aria-label={ariaLabel}
            data-testid={`agent-fleet-row-link-${agent.name}`}
          />
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
            <Pill size="xs" tone="neutral" data-testid={`agent-fleet-origin-${agent.name}`}>
              {cardOrigin}
            </Pill>
          </ListingRow.Name>
          {metaSegments.length > 0 ? (
            <ListingRow.Meta data-testid={`agent-fleet-meta-${agent.name}`}>
              {metaSegments.map((segment, index) => (
                <Fragment key={segment.key}>
                  {index > 0 ? <ListingRow.MetaDot /> : null}
                  <span className="font-mono text-badge text-subtle">{segment.value}</span>
                </Fragment>
              ))}
            </ListingRow.Meta>
          ) : null}
          {/*
            The description is what another agent reads when choosing a
            specialist, so it is the row's most useful line — and it is model
            output, which is why it renders as plain text inside the row rather
            than as anything that could carry a link or a control.
          */}
          {agent.description.trim() ? (
            <ListingRow.Description data-testid={`agent-fleet-description-${agent.name}`}>
              {agent.description}
            </ListingRow.Description>
          ) : null}
          <AgentLayerProvenance
            data-testid={`agent-fleet-provenance-${agent.name}`}
            layer={row.layer}
            shadows={row.shadowLayers}
          />
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail className="gap-3">
        {/*
          An inactive name collision: this definition exists but a same-named one
          in a nearer layer is what actually runs.
        */}
        {agent.shadowed ? (
          <Pill tone="warning" size="sm" data-testid={`agent-fleet-shadowed-${agent.name}`}>
            Shadowed
          </Pill>
        ) : null}
        {sessionsAvailable && signals ? (
          <Pill
            size="sm"
            tone={signals.status === "active" ? "success" : "neutral"}
            data-testid={`agent-fleet-status-${agent.name}`}
          >
            <Pill.Dot tone={signals.status === "active" ? "success" : "neutral"} size="sm" />
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

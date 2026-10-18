import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

import { Button, MetadataList, Panel, Pill } from "@compozy/ui";

import { formatAbsentOverride } from "../lib/agent-absent-value";
import { agentShadowLayers, formatAgentLayer } from "../lib/agent-fleet-projection";
import { permissionLabel } from "../lib/agent-permissions";
import type { AgentPayload } from "../types";
import { AgentMcpServersPanel } from "./agent-mcp-servers-panel";

export interface AgentConfigurationTabProps {
  agent: AgentPayload;
  onEditSection: (section: "runtime" | "access" | "mcp") => void;
}

export function AgentConfigurationTab({ agent, onEditSection }: AgentConfigurationTabProps) {
  const tools = agent.tools ?? [];
  const denyTools = agent.deny_tools ?? [];
  const toolsets = agent.toolsets ?? [];
  const shadows = agentShadowLayers(agent);
  const command = agent.command?.trim() ?? "";

  return (
    <div className="flex flex-col gap-6" data-testid="agent-configuration-tab">
      <Panel
        bodyClassName="p-0"
        data-testid="agent-config-runtime"
        right={
          <EditButton onClick={() => onEditSection("runtime")} testId="agent-config-edit-runtime" />
        }
        title="Runtime"
      >
        <MetadataList className="gap-0">
          <ConfigRow label="Defined in" testId="agent-config-defined-in">
            <span className="font-mono text-mono-id">{formatAgentLayer(agent)}</span>
            {shadows.length > 0 ? (
              <span className="text-muted"> · overrides {shadows.join(", ")}</span>
            ) : null}
          </ConfigRow>
          <ConfigRow label="Command">
            <span className={command ? "font-mono" : "text-muted"}>
              {formatAbsentOverride(command)}
            </span>
          </ConfigRow>
          <ConfigRow label="Permissions">{permissionLabel(agent.permissions)}</ConfigRow>
        </MetadataList>
      </Panel>

      <Panel
        bodyClassName="p-0"
        data-testid="agent-config-access"
        right={
          <EditButton onClick={() => onEditSection("access")} testId="agent-config-edit-access" />
        }
        title="Access"
      >
        <MetadataList className="gap-0">
          <AccessTokenRow label="Allowed tools" testId="agent-config-tools" values={tools} />
          <AccessTokenRow
            label="Blocked tools"
            testId="agent-config-deny-tools"
            values={denyTools}
          />
          <AccessTokenRow label="Tool groups" testId="agent-config-toolsets" values={toolsets} />
        </MetadataList>
      </Panel>

      <Panel
        bodyClassName="p-0"
        data-testid="agent-config-mcp"
        right={<EditButton onClick={() => onEditSection("mcp")} testId="agent-config-edit-mcp" />}
        title="MCP servers"
      >
        <AgentMcpServersPanel agent={agent} bare />
      </Panel>
    </div>
  );
}

function EditButton({ onClick, testId }: { onClick: () => void; testId: string }) {
  return (
    // Optical: the trailing chevron lands on the card's content edge.
    <Button
      className="-mr-2"
      data-testid={testId}
      onClick={onClick}
      size="sm"
      type="button"
      variant="link"
    >
      Edit
      <ChevronRight aria-hidden="true" data-icon="inline-end" />
    </Button>
  );
}

const configRowClassName =
  "flex-col items-start gap-1.5 border-t border-line-soft px-4 py-3.5 first:border-t-0";

function ConfigRow({
  label,
  testId,
  children,
}: {
  label: string;
  testId?: string;
  children: ReactNode;
}) {
  return (
    <MetadataList.Row className={configRowClassName} data-testid={testId}>
      <MetadataList.Term>{label}</MetadataList.Term>
      <MetadataList.Value className="text-fg">{children}</MetadataList.Value>
    </MetadataList.Row>
  );
}

function AccessTokenRow({
  label,
  values,
  testId,
}: {
  label: string;
  values: readonly string[];
  testId: string;
}) {
  return (
    <ConfigRow label={label} testId={testId}>
      {values.length === 0 ? (
        <span className="text-muted">None</span>
      ) : (
        <span className="flex flex-wrap gap-1.5">
          {values.map(value => (
            <Pill key={value} mono size="sm">
              {value}
            </Pill>
          ))}
        </span>
      )}
    </ConfigRow>
  );
}

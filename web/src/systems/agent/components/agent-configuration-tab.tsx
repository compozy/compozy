import { Pill, Section } from "@agh/ui";

import { formatAbsentOverride } from "../lib/agent-absent-value";
import type { AgentPayload } from "../types";
import { AgentMcpServersPanel } from "./agent-mcp-servers-panel";
import { AgentPanelBox } from "./agent-panel-box";

export interface AgentConfigurationTabProps {
  agent: AgentPayload;
  onEditSection: (section: "runtime" | "access" | "mcp") => void;
}

export function AgentConfigurationTab({ agent, onEditSection }: AgentConfigurationTabProps) {
  const tools = agent.tools ?? [];
  const denyTools = agent.deny_tools ?? [];
  const toolsets = agent.toolsets ?? [];

  return (
    <div className="flex flex-col gap-6" data-testid="agent-configuration-tab">
      <Section
        className="gap-0"
        label="Runtime"
        data-testid="agent-config-runtime"
        right={
          <EditLink onClick={() => onEditSection("runtime")} testId="agent-config-edit-runtime" />
        }
      >
        <AgentPanelBox>
          <div className="flex flex-col">
            <ConfigKvRow
              label="Command"
              value={formatAbsentOverride(agent.command)}
              muted={!agent.command?.trim()}
              mono={Boolean(agent.command?.trim())}
            />
            <ConfigKvRow
              label="Permissions"
              value={agent.permissions?.trim() || "Default"}
              muted={!agent.permissions?.trim()}
              mono={Boolean(agent.permissions?.trim())}
            />
          </div>
        </AgentPanelBox>
      </Section>

      <Section
        className="gap-0"
        label="Access"
        data-testid="agent-config-access"
        right={
          <EditLink onClick={() => onEditSection("access")} testId="agent-config-edit-access" />
        }
      >
        <AgentPanelBox>
          <AccessTokenRow label="Tools" values={tools} testId="agent-config-tools" />
          <AccessTokenRow
            label="Deny tools"
            values={denyTools}
            tone="danger"
            testId="agent-config-deny-tools"
          />
          <AccessTokenRow label="Toolsets" values={toolsets} testId="agent-config-toolsets" />
        </AgentPanelBox>
      </Section>

      <Section
        className="gap-0"
        label="MCP servers"
        data-testid="agent-config-mcp"
        right={<EditLink onClick={() => onEditSection("mcp")} testId="agent-config-edit-mcp" />}
      >
        <AgentPanelBox>
          <AgentMcpServersPanel agent={agent} bare />
        </AgentPanelBox>
      </Section>
    </div>
  );
}

function EditLink({ onClick, testId }: { onClick: () => void; testId: string }) {
  return (
    <button
      type="button"
      className="text-small-body text-muted hover:text-fg"
      onClick={onClick}
      data-testid={testId}
    >
      Edit ›
    </button>
  );
}

function ConfigKvRow({
  label,
  value,
  muted,
  mono,
}: {
  label: string;
  value: string;
  muted?: boolean;
  mono?: boolean;
}) {
  return (
    <div className="border-t border-line-soft px-4 py-3.5 first:border-t-0">
      <p className="eyebrow text-muted">{label}</p>
      <p
        className={[
          "mt-1.5 text-body",
          muted ? "text-muted" : "text-fg",
          mono ? "font-mono" : "",
        ].join(" ")}
      >
        {value}
      </p>
    </div>
  );
}

function AccessTokenRow({
  label,
  values,
  tone = "neutral",
  testId,
}: {
  label: string;
  values: readonly string[];
  tone?: "neutral" | "danger";
  testId: string;
}) {
  return (
    <div className="border-t border-line-soft px-4 py-3.5 first:border-t-0" data-testid={testId}>
      <p className="eyebrow mb-2 text-muted">{label}</p>
      {values.length === 0 ? (
        <p className="text-small-body text-muted">None</p>
      ) : (
        <div className="flex flex-wrap gap-1.5">
          {values.map(value => (
            <Pill key={value} mono size="sm" tone={tone}>
              {value}
            </Pill>
          ))}
        </div>
      )}
    </div>
  );
}

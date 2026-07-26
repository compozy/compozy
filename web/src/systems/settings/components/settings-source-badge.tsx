import { Pill, type PillTone } from "@agh/ui";

import type { SettingsSource, SettingsSourceKind } from "../types";

interface SettingsSourceBadgeProps {
  source: SettingsSource;
  shadowed?: SettingsSource[];
  "data-testid"?: string;
}

const KIND_LABELS: Record<SettingsSourceKind, string> = {
  "builtin-provider": "BUILTIN",
  "global-config": "CONFIG",
  "workspace-config": "WORKSPACE",
  "global-mcp-sidecar": "MCP.JSON",
  "workspace-mcp-sidecar": "WS-MCP.JSON",
  "global-agent-file": "AGENT",
  "workspace-agent-file": "WS-AGENT",
};

function badgeTone(kind: SettingsSourceKind): PillTone {
  switch (kind) {
    case "builtin-provider":
      return "neutral";
    case "global-config":
    case "global-mcp-sidecar":
    case "global-agent-file":
      return "info";
    case "workspace-config":
    case "workspace-mcp-sidecar":
    case "workspace-agent-file":
      return "warning";
    default:
      return "neutral";
  }
}

function sourceLabel(source: SettingsSource): string {
  const parts = [KIND_LABELS[source.kind]];
  if (source.agent_name) {
    parts.push(source.agent_name);
  }
  if (source.workspace_id) {
    parts.push(source.workspace_id);
  }
  return parts.join(" · ");
}

function SettingsSourceBadge({
  source,
  shadowed,
  "data-testid": testId,
}: SettingsSourceBadgeProps) {
  return (
    <div className="flex flex-wrap items-center gap-1.5" data-testid={testId}>
      <Pill
        mono
        tone={badgeTone(source.kind)}
        data-testid={testId ? `${testId}-effective` : undefined}
      >
        {sourceLabel(source)}
      </Pill>
      {shadowed && shadowed.length > 0 ? (
        <span
          className="flex flex-wrap items-center gap-1 font-mono text-badge font-medium tracking-mono text-muted"
          data-testid={testId ? `${testId}-shadowed` : undefined}
        >
          <span className="uppercase">shadows</span>
          {shadowed.map(entry => (
            <Pill
              mono
              tone="neutral"
              key={`${entry.kind}-${entry.scope}-${entry.agent_name ?? ""}-${entry.workspace_id ?? ""}`}
            >
              {sourceLabel(entry)}
            </Pill>
          ))}
        </span>
      ) : null}
    </div>
  );
}

export { SettingsSourceBadge };
export type { SettingsSource };

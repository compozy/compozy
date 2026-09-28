import { Pill, type PillTone } from "@compozy/ui";

import type { SettingsSource, SettingsSourceKind } from "../types";

interface SettingsSourceBadgeProps {
  source: SettingsSource;
  shadowed?: SettingsSource[];
  "data-testid"?: string;
}

const KIND_LABELS: Record<SettingsSourceKind, string> = {
  extension: "From extension",
  "builtin-provider": "Built in",
  "global-config": "From settings",
  "profile-config": "From profile",
  "workspace-config": "From project",
  "workspace-profile-config": "From project profile",
  "global-mcp-sidecar": "From mcp.json",
  "profile-mcp-sidecar": "From profile",
  "workspace-mcp-sidecar": "From project",
  "workspace-profile-mcp-sidecar": "From project profile",
  "global-agent-file": "From agent file",
  "workspace-agent-file": "From project agent file",
};

/** The file behind a source, shown on hover so the label itself stays plain. */
const KIND_FILES: Partial<Record<SettingsSourceKind, string>> = {
  "global-config": "config.toml",
  "profile-config": "profile config.toml",
  "workspace-config": "project config.toml",
  "workspace-profile-config": "project profile config.toml",
  "global-mcp-sidecar": "mcp.json",
  "profile-mcp-sidecar": "profile mcp.json",
  "workspace-mcp-sidecar": "project mcp.json",
  "workspace-profile-mcp-sidecar": "project profile mcp.json",
  "global-agent-file": "agent file",
  "workspace-agent-file": "project agent file",
};

function badgeTone(kind: SettingsSourceKind): PillTone {
  switch (kind) {
    case "builtin-provider":
      return "neutral";
    case "global-config":
    case "global-mcp-sidecar":
    case "global-agent-file":
    case "profile-config":
    case "profile-mcp-sidecar":
      return "info";
    case "workspace-config":
    case "workspace-profile-config":
    case "workspace-mcp-sidecar":
    case "workspace-profile-mcp-sidecar":
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
  if (source.profile) {
    parts.push(source.profile);
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
        tone={badgeTone(source.kind)}
        title={KIND_FILES[source.kind]}
        data-testid={testId ? `${testId}-effective` : undefined}
      >
        {sourceLabel(source)}
      </Pill>
      {shadowed && shadowed.length > 0 ? (
        <span
          className="flex flex-wrap items-center gap-1 text-badge font-medium text-muted"
          data-testid={testId ? `${testId}-shadowed` : undefined}
        >
          <span>replaces</span>
          {shadowed.map(entry => (
            <Pill
              tone="neutral"
              title={KIND_FILES[entry.kind]}
              key={`${entry.kind}-${entry.scope}-${entry.agent_name ?? ""}-${entry.profile ?? ""}-${entry.workspace_id ?? ""}`}
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

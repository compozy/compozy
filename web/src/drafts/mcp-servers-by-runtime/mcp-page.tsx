/**
 * Settings › MCP servers inside the app shell (`_uiux.md` S1, S2): the real Settings nav and page
 * frame around the production `MCPServersByRuntimeView` and `MCPCopyPanel`, fed with
 * the system's fixtures. Marketplace › MCPs reuses the same body.
 */
import { Plus, Settings } from "@compozy/ui/icons";
import { useState } from "react";

import { Button, HelpTip, ListingToolbar, PillGroup, Toaster } from "@compozy/ui";

import { SettingsWindowNav } from "@/systems/os/apps/settings/settings-window-nav";
import { MCPRuntimeModeSwitchView } from "@/systems/settings/components/mcp-runtime-mode-switch-view";
import { MCPServersByRuntimeView } from "@/systems/settings/components/mcp-servers-by-runtime-view";
import { SettingsPageFrame } from "@/systems/settings/components/settings-page-frame";
import { mcpDefinitionKey } from "@/systems/settings/lib/mcp-management-target";
import {
  isMCPApprovalWaiting,
  matchesMCPQuery,
  MCP_FILTER_ALL,
  type MCPApprovalNote,
} from "@/systems/settings/lib/mcp-runtime-view-model";
import type { RuntimeMCPSnapshot, SettingsMCPServerEntry } from "@/systems/settings/types";

import { DraftShell } from "../always-on-agents-shared/draft-shell";
import { useCopyDialog } from "./hooks/use-copy-dialog";
import { CopyPanelHost } from "./copy-panel-host";
import type { CopyDialogCase } from "./mcp-page-fixtures";

export interface MCPPageProps {
  compozy: readonly SettingsMCPServerEntry[];
  runtimes: readonly RuntimeMCPSnapshot[];
  approvalNotes?: Readonly<Record<string, MCPApprovalNote>>;
  loading?: boolean;
  query?: string;
  filter?: string;
  /** Opens the copy dialog on the first copyable server with secret keys. */
  copy?: CopyDialogCase;
  /** The copy just finished: highlight the new row and show the toast. */
  copied?: SettingsMCPServerEntry;
}

export function MCPServersByRuntimePage({
  compozy: initialServers,
  runtimes,
  approvalNotes = {},
  loading = false,
  query: initialQuery = "",
  filter: initialFilter = MCP_FILTER_ALL,
  copy,
  copied,
}: MCPPageProps) {
  const [query, setQuery] = useState(initialQuery);
  const [filter, setFilter] = useState(initialFilter);
  const [servers, setServers] = useState(initialServers);
  const [modes, setModes] = useState<Record<string, string>>({});
  const [copying, setCopying] = useCopyDialog(runtimes, copy, copied);

  const visible = servers.filter(server =>
    matchesMCPQuery([server.name, server.owner, server.runtime_name], query)
  );
  const snapshots = runtimes.map(snapshot => ({
    ...snapshot,
    runtime_mcp: {
      ...snapshot.runtime_mcp,
      mode: modes[snapshot.runtime] ?? snapshot.runtime_mcp.mode,
    },
  }));
  const waiting = servers.filter(isMCPApprovalWaiting).length;
  const fromAgents = runtimes.reduce((sum, snapshot) => sum + snapshot.servers.length, 0);

  return (
    <DraftShell
      app="dashboard"
      glyph={<Settings aria-hidden="true" />}
      title="Settings"
      topbar={{
        crumb: "MCP servers",
        actions: (
          <Button size="sm" type="button" variant="secondary">
            <Plus aria-hidden="true" />
            Add MCP server
          </Button>
        ),
      }}
    >
      <div className="@container flex min-h-0 flex-1 flex-col overflow-hidden">
        <div className="flex min-h-0 flex-1 flex-col @min-settings-takeover:flex-row">
          <SettingsWindowNav activeSlug="mcp" connection="connected" onNavigate={() => undefined} />
          <SettingsPageFrame
            meta={
              loading
                ? []
                : [
                    { key: "servers", content: `${servers.length} in CompozyOS` },
                    ...(fromAgents > 0
                      ? [{ key: "runtimes", content: `${fromAgents} from your coding agents` }]
                      : []),
                    ...(waiting > 0
                      ? [{ key: "waiting", content: `${waiting} waiting for approval` }]
                      : []),
                  ]
            }
            slug="mcp"
            width="wide"
          >
            <div className="flex flex-col gap-3">
              <ListingToolbar>
                <ListingToolbar.Leading>
                  <PillGroup
                    aria-label="Show servers for"
                    items={[
                      { value: "user", label: "Personal" },
                      { value: "workspace", label: "Project" },
                    ]}
                    onChange={() => undefined}
                    size="sm"
                    value="user"
                  />
                  <ListingToolbar.Search
                    aria-label="Search MCP servers"
                    kbd={null}
                    onChange={setQuery}
                    placeholder="Search servers"
                    value={query}
                  />
                </ListingToolbar.Leading>
                <ListingToolbar.Trailing>
                  <HelpTip label="About MCP servers">
                    CompozyOS servers work in every session. Your coding agents also load their own
                    servers; those are listed under each agent and stay as they are.
                  </HelpTip>
                </ListingToolbar.Trailing>
              </ListingToolbar>
              <MCPServersByRuntimeView
                compozy={{
                  state: loading ? "pending" : "ready",
                  servers,
                  visibleServers: visible,
                  onCreate: () => undefined,
                  table: {
                    onAuthorize: () => undefined,
                    onEdit: () => undefined,
                    onSelect: () => undefined,
                    onAllow: entry =>
                      setServers(current =>
                        current.map(server =>
                          server === entry
                            ? { ...server, approval: { ...server.approval, state: "allowed" } }
                            : server
                        )
                      ),
                    approvalNotes,
                    highlightedServer: copied ? mcpDefinitionKey(copied) : undefined,
                  },
                }}
                filter={filter}
                onClearQuery={() => setQuery("")}
                onCopy={(snapshot, server) => setCopying({ snapshot, server })}
                onFilterChange={setFilter}
                onShowFile={() => undefined}
                query={query}
                renderModeSwitch={snapshot => (
                  <MCPRuntimeModeSwitchView
                    checked={snapshot.runtime_mcp.mode !== "off"}
                    onCheckedChange={next =>
                      setModes(current => ({
                        ...current,
                        [snapshot.runtime]: next ? "load" : "off",
                      }))
                    }
                    runtimeName={snapshot.display_name}
                  />
                )}
                runtimes={{ state: loading ? "pending" : "ready", snapshots }}
              />
            </div>
          </SettingsPageFrame>
        </div>
      </div>
      <CopyPanelHost copy={copy} copying={copying} onClose={() => setCopying(null)} />
      <Toaster />
    </DraftShell>
  );
}

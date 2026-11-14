/**
 * Settings › MCP servers inside the app shell (`_uiux.md` S1): the real Settings nav and page
 * frame, the toolbar with the new runtime filter, then the CompozyOS group and one read-only
 * group per runtime. Marketplace › MCPs reuses the same body.
 */
import { Cable, Plus, SearchX, Settings } from "@compozy/ui/icons";
import { useEffect, useState } from "react";

import {
  Button,
  Empty,
  HelpTip,
  ListingRowSkeleton,
  ListingToolbar,
  PillGroup,
  Skeleton,
  SkeletonRegion,
  Toaster,
  toast,
} from "@compozy/ui";

import { SettingsWindowNav } from "@/systems/os/apps/settings/settings-window-nav";
import { ProviderLogo } from "@/systems/settings/components/provider-logo";
import { SettingsPageFrame } from "@/systems/settings/components/settings-page-frame";
import { mcpManagementServerFixtures } from "@/systems/settings/mocks/fixtures";
import type { SettingsMCPServerEntry } from "@/systems/settings/types";

import { DraftShell } from "../always-on-agents-shared/draft-shell";
import { CompozyGroup, type ApprovalOverlay } from "./compozy-group";
import { CopyToCompozyDialog, type CopyDialogCase } from "./copy-dialog";
import { RuntimeGroup } from "./runtime-group";
import type { RuntimeGroupData, RuntimeServer } from "./runtime-fixtures";

export const COMPOZY_SERVERS: readonly SettingsMCPServerEntry[] = mcpManagementServerFixtures.slice(
  0,
  5
);

export interface MCPPageProps {
  compozy: readonly SettingsMCPServerEntry[];
  runtimes: readonly RuntimeGroupData[];
  approvals?: ApprovalOverlay;
  loading?: boolean;
  query?: string;
  filter?: string;
  /** Opens the copy dialog on the first copyable Claude server. */
  copy?: CopyDialogCase;
  /** The copy just finished: highlight the new row and show the toast. */
  copied?: string;
  foldedRuntimes?: boolean;
}

function runtimeLabel(group: RuntimeGroupData) {
  return (
    <span className="inline-flex items-center gap-1.5">
      <ProviderLogo className="size-3.5" provider={group.runtime} />
      {group.displayName}
    </span>
  );
}

function LoadingBody() {
  return (
    <SkeletonRegion className="flex flex-col gap-4" label="Loading MCP servers">
      {["compozy", "claude", "codex"].map(id => (
        <div className="overflow-hidden rounded-lg bg-card shadow-card" key={id}>
          <div className="flex min-h-12 items-center gap-3 border-b border-line px-4">
            <Skeleton className="size-7 rounded-md" />
            <Skeleton className="h-3.5 w-32 rounded-pill" />
          </div>
          <ListingRowSkeleton count={2} trail="pill" />
        </div>
      ))}
    </SkeletonRegion>
  );
}

function matches(name: string, query: string) {
  return name.toLowerCase().includes(query.trim().toLowerCase());
}

export function MCPServersByRuntimePage({
  compozy,
  runtimes,
  approvals = {},
  loading = false,
  query: initialQuery = "",
  filter: initialFilter = "all",
  copy,
  copied,
  foldedRuntimes = true,
}: MCPPageProps) {
  const [query, setQuery] = useState(initialQuery);
  const [filter, setFilter] = useState(initialFilter);
  const [copying, setCopying] = useState<RuntimeServer | null>(
    copy ? (runtimes[0]?.servers.find(server => server.copyable && server.envKeys.length + server.headerKeys.length > 0) ?? null) : null
  );
  const [allowed, setAllowed] = useState<ApprovalOverlay>(approvals);

  useEffect(() => {
    if (!copied) return;
    toast.success(`${copied} copied. Sessions can use it from their next message.`);
  }, [copied]);

  const compozyRows = compozy.filter(server => matches(server.name, query));
  const runtimeRows = runtimes.map(group => ({
    ...group,
    servers: group.servers.filter(server => matches(server.name, query)),
  }));
  const shownRuntimes = runtimeRows.filter(
    group =>
      (filter === "all" || filter === group.runtime) &&
      (group.servers.length > 0 || group.readState === "unreadable")
  );
  const showCompozy = filter === "all" || filter === "compozy";
  const waiting = Object.keys(allowed).length;
  const total = compozy.length + runtimes.reduce((sum, group) => sum + group.servers.length, 0);
  const nothing = compozy.length === 0 && runtimes.every(group => group.servers.length === 0);
  const noMatch =
    !nothing &&
    query !== "" &&
    compozyRows.length === 0 &&
    runtimeRows.every(group => group.servers.length === 0);
  const filterItems = [
    { value: "all", label: "All", badge: total },
    { value: "compozy", label: "CompozyOS", badge: compozy.length },
    ...runtimes
      .filter(group => group.servers.length > 0 || group.readState === "unreadable")
      .map(group => ({
        value: group.runtime,
        label: runtimeLabel(group),
        badge: group.servers.length,
      })),
  ];
  const copyGroup = runtimes[0];

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
                  { key: "servers", content: `${compozy.length} in CompozyOS` },
                  {
                    key: "runtimes",
                    content: `${total - compozy.length} from your coding agents`,
                  },
                  ...(waiting > 0
                    ? [{ key: "waiting", content: `${waiting} waiting for approval` }]
                    : []),
                ]
          }
          slug="mcp"
          width="wide"
        >
          <div className="flex flex-col gap-4">
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
            {!loading && !nothing ? (
              <PillGroup
                aria-label="Filter by where the server comes from"
                items={filterItems}
                onChange={setFilter}
                size="sm"
                value={filter}
              />
            ) : null}
            {loading ? (
              <LoadingBody />
            ) : nothing ? (
              <Empty
                action={
                  <Button size="sm" type="button">
                    <Plus aria-hidden="true" />
                    Add MCP server
                  </Button>
                }
                description="Servers you add here, the ones your extensions add, and the ones your coding agents load all show up in this list."
                icon={Cable}
                title="No MCP servers yet"
              />
            ) : noMatch ? (
              <Empty
                action={
                  <Button onClick={() => setQuery("")} size="sm" type="button" variant="secondary">
                    Clear search
                  </Button>
                }
                description={`Nothing matches "${query}".`}
                icon={SearchX}
                title="No MCP servers match"
              />
            ) : (
              <>
                {showCompozy && compozyRows.length > 0 ? (
                  <CompozyGroup
                    approvals={allowed}
                    highlighted={copied}
                    onAllow={name =>
                      setAllowed(current =>
                        Object.fromEntries(Object.entries(current).filter(([key]) => key !== name))
                      )
                    }
                    servers={compozyRows}
                  />
                ) : null}
                {showCompozy && compozy.length === 0 ? (
                  <Empty
                    action={
                      <Button size="sm" type="button">
                        <Plus aria-hidden="true" />
                        Add MCP server
                      </Button>
                    }
                    description="Add one, or copy a server your coding agent already uses."
                    framed
                    icon={Cable}
                    title="No CompozyOS servers yet"
                    titleAs="h3"
                  />
                ) : null}
                {shownRuntimes.map(group => (
                  <RuntimeGroup
                    folded={foldedRuntimes}
                    group={group}
                    key={group.runtime}
                    onCopy={setCopying}
                  />
                ))}
              </>
            )}
          </div>
        </SettingsPageFrame>
        </div>
      </div>
      {copying && copyGroup ? (
        <CopyToCompozyDialog
          canTurnOff={copyGroup.runtimeMcp.canDisable}
          onClose={() => setCopying(null)}
          runtime={copyGroup.displayName}
          server={copying}
          which={copy ?? "default"}
        />
      ) : null}
      <Toaster />
    </DraftShell>
  );
}

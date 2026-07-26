import { Button, cn, Pill } from "@agh/ui";
import { Pencil, Plug } from "lucide-react";

import { composeMCPRowStatus } from "../lib/mcp-status-view-model";
import type { SettingsMCPServerEntry } from "../types";

import { MCPStatusCell } from "./mcp-status-cell";
import { mcpProvenanceLine, mcpSourceKindLabel } from "./mcp-server-labels";

const HEADER_COLUMNS = ["Server", "Config", "Authorization", "Runtime", "Probe / tools", "Actions"];

// Desktop grid: server (flex) | config | auth | runtime | probe | actions. On
// mobile the row collapses to a 2-up card (server + actions span both columns).
const ROW_GRID = "md:grid-cols-[minmax(200px,1.4fr)_100px_150px_162px_140px_150px]";

export interface MCPServersTableProps {
  servers: SettingsMCPServerEntry[];
  selectedServer?: string;
  onSelect: (name: string) => void;
  onEdit: (entry: SettingsMCPServerEntry) => void;
  onAuthorize: (entry: SettingsMCPServerEntry) => void;
}

export function MCPServersTable({
  servers,
  selectedServer,
  onSelect,
  onEdit,
  onAuthorize,
}: MCPServersTableProps) {
  return (
    <section
      aria-label="Configured MCP server status matrix"
      data-testid="settings-page-mcp-servers-list"
      className="overflow-hidden rounded-lg border border-line bg-canvas-soft max-md:border-0 max-md:bg-transparent"
    >
      <div
        aria-hidden="true"
        className={cn(
          "hidden min-h-[34px] items-center gap-x-3.5 border-b border-line bg-elevated px-3.5 md:grid",
          ROW_GRID
        )}
      >
        {HEADER_COLUMNS.map((column, index) => (
          <span
            key={column}
            className={cn(
              "eyebrow text-muted",
              index === HEADER_COLUMNS.length - 1 && "text-right"
            )}
          >
            {column}
          </span>
        ))}
      </div>
      <div className="max-md:flex max-md:flex-col max-md:gap-2">
        {servers.map(server => (
          <MCPServerRow
            key={`${server.name}-${server.source_metadata.effective_source.kind}`}
            server={server}
            selected={server.name === selectedServer}
            onSelect={onSelect}
            onEdit={onEdit}
            onAuthorize={onAuthorize}
          />
        ))}
      </div>
    </section>
  );
}

function MCPServerRow({
  server,
  selected,
  onSelect,
  onEdit,
  onAuthorize,
}: {
  server: SettingsMCPServerEntry;
  selected: boolean;
  onSelect: (name: string) => void;
  onEdit: (entry: SettingsMCPServerEntry) => void;
  onAuthorize: (entry: SettingsMCPServerEntry) => void;
}) {
  const status = composeMCPRowStatus(server);
  const endpoint =
    server.transport === "stdio"
      ? [server.command, ...(server.args ?? [])].filter(Boolean).join(" ")
      : server.url;
  const sourceLine =
    mcpProvenanceLine(server.catalog_entry, server.catalog_version) ??
    mcpSourceKindLabel(server.source_metadata.effective_source.kind);
  const rowTestId = `settings-page-mcp-servers-row-${server.name}`;

  return (
    <div
      data-testid={rowTestId}
      className={cn(
        "grid grid-cols-2 gap-x-3.5 gap-y-3 border-t border-line-soft p-3.5 transition-colors",
        ROW_GRID,
        "md:min-h-[68px] md:items-center md:gap-y-0",
        "max-md:rounded-lg max-md:border max-md:border-line max-md:bg-canvas-soft",
        selected
          ? "bg-row-selected shadow-[inset_0_0_0_1px_var(--color-line-strong)]"
          : "hover:bg-row-hover"
      )}
    >
      <div className="col-span-2 flex min-w-0 items-center gap-2.5 md:col-span-1">
        <span
          aria-hidden="true"
          className="grid size-[34px] shrink-0 place-items-center rounded-md bg-elevated text-muted"
        >
          <Plug className="size-[15px]" />
        </span>
        <div className="min-w-0">
          <div className="flex min-w-0 items-center gap-1.5">
            <button
              type="button"
              aria-current={selected ? "true" : undefined}
              onClick={() => onSelect(server.name)}
              data-testid={`${rowTestId}-name`}
              className="min-w-0 truncate text-left font-mono text-small-body font-medium text-fg-strong hover:underline"
            >
              {server.name}
            </button>
            <Pill tone="neutral">{server.transport}</Pill>
          </div>
          <div
            className="mt-0.5 truncate font-mono text-mono-id text-muted"
            title={endpoint ?? undefined}
            data-testid={`${rowTestId}-endpoint`}
          >
            {endpoint || "-"}
          </div>
          <div
            className="mt-0.5 truncate text-micro text-subtle"
            title={sourceLine}
            data-testid={`${rowTestId}-source`}
          >
            {sourceLine}
          </div>
        </div>
      </div>
      <MCPStatusCell cell={status.config} testId={`${rowTestId}-config`} />
      <MCPStatusCell cell={status.auth} testId={`${rowTestId}-auth`} />
      <MCPStatusCell cell={status.runtime} testId={`${rowTestId}-runtime`} />
      <MCPStatusCell cell={status.probe} testId={`${rowTestId}-probe`} />
      <div className="col-span-2 flex items-center justify-end gap-1.5 md:col-span-1">
        {status.repairable && status.authorizeLabel ? (
          <Button
            type="button"
            variant="neutral"
            size="sm"
            onClick={() => onAuthorize(server)}
            data-testid={`${rowTestId}-authorize`}
          >
            {status.authorizeLabel}
          </Button>
        ) : null}
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          onClick={() => onEdit(server)}
          aria-label={`Edit ${server.name}`}
          data-testid={`${rowTestId}-edit`}
        >
          <Pencil className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}

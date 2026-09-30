import { Button, cn, Pill } from "@compozy/ui";
import { Pencil, Plug } from "lucide-react";

import { mcpDefinitionKey } from "../lib/mcp-management-target";
import { composeMCPRowStatus } from "../lib/mcp-status-view-model";
import type { SettingsMCPServerEntry } from "../types";

import {
  mcpOwnerExtensionName,
  mcpServerProvenanceLine,
  mcpServerRowTestId,
} from "./mcp-server-labels";

const HEADER_COLUMNS = ["Server", "Status", "Actions"];

// Desktop grid: server (flex) | status | actions. On mobile the row collapses to a
// 2-up card (server + actions span both columns).
const ROW_GRID = "md:grid-cols-[minmax(0,1fr)_auto_auto]";

export interface MCPServersTableProps {
  servers: SettingsMCPServerEntry[];
  /** Full definition identity (`mcpDefinitionKey`), never a bare name: names repeat across owners. */
  selectedServer?: string;
  onSelect: (entry: SettingsMCPServerEntry) => void;
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
      aria-label="MCP servers"
      data-testid="settings-page-mcp-servers-list"
      className="overflow-hidden rounded-lg bg-canvas shadow-card max-md:bg-transparent max-md:shadow-none"
    >
      <div
        aria-hidden="true"
        className={cn(
          "hidden min-h-9 items-center gap-x-3.5 border-b border-line px-3.5 md:grid",
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
            key={mcpDefinitionKey(server)}
            server={server}
            selected={mcpDefinitionKey(server) === selectedServer}
            onSelect={onSelect}
            onEdit={onEdit}
            onAuthorize={onAuthorize}
          />
        ))}
      </div>
    </section>
  );
}

/**
 * One definition per row. Manual and extension-provided servers of the same name coexist: the
 * owner word after the name and the provenance line tell them apart, and every action carries
 * the full definition, never the name alone.
 */
function MCPServerRow({
  server,
  selected,
  onSelect,
  onEdit,
  onAuthorize,
}: {
  server: SettingsMCPServerEntry;
  selected: boolean;
  onSelect: (entry: SettingsMCPServerEntry) => void;
  onEdit: (entry: SettingsMCPServerEntry) => void;
  onAuthorize: (entry: SettingsMCPServerEntry) => void;
}) {
  const status = composeMCPRowStatus(server);
  const extension = mcpOwnerExtensionName(server.owner);
  const sourceLine = mcpServerProvenanceLine(server);
  const rowTestId = mcpServerRowTestId(server);
  const accessibleName = extension ? `${server.name} from ${extension}` : server.name;

  return (
    <div
      data-testid={rowTestId}
      data-owner={server.owner ?? "manual"}
      className={cn(
        "grid grid-cols-2 gap-x-3.5 gap-y-3 border-t border-line-soft p-3.5 transition-colors",
        ROW_GRID,
        "md:min-h-setting-row md:items-center md:gap-y-0",
        "max-md:rounded-lg max-md:bg-canvas max-md:shadow-card",
        selected ? "bg-selected" : "hover:bg-sunken"
      )}
    >
      <div className="col-span-2 flex min-w-0 items-center gap-2.5 md:col-span-1">
        <span
          aria-hidden="true"
          className="grid size-8 shrink-0 place-items-center rounded-md bg-surface-2 text-muted"
        >
          <Plug className="size-4" />
        </span>
        <div className="min-w-0">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            <button
              type="button"
              aria-current={selected ? "true" : undefined}
              aria-label={`Select ${accessibleName}`}
              onClick={() => onSelect(server)}
              data-testid={`${rowTestId}-name`}
              className="min-w-0 truncate rounded-xs text-left font-mono text-small-body font-medium text-fg-strong hover:underline focus-visible:shadow-focus-ring focus-visible:outline-none"
            >
              {server.name}
            </button>
            {extension ? (
              <span
                className="text-micro whitespace-nowrap text-subtle"
                data-testid={`${rowTestId}-owner`}
              >
                from {extension}
              </span>
            ) : null}
          </div>
          <div
            className="mt-0.5 truncate text-micro text-subtle"
            data-testid={`${rowTestId}-source`}
          >
            {sourceLine}
          </div>
        </div>
      </div>
      <div className="min-w-0" data-testid={`${rowTestId}-status`}>
        <Pill tone={status.summary.tone === "success" ? "neutral" : status.summary.tone}>
          <Pill.Dot tone={status.summary.tone} />
          {status.summary.label}
        </Pill>
      </div>
      <div className="flex items-center justify-end gap-1.5">
        {status.repairable && status.authorizeLabel ? (
          <Button
            type="button"
            variant="neutral"
            size="sm"
            aria-label={`${status.authorizeLabel} ${accessibleName}`}
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
          aria-label={`Edit ${accessibleName}`}
          data-testid={`${rowTestId}-edit`}
        >
          <Pencil className="size-3.5" />
        </Button>
      </div>
    </div>
  );
}

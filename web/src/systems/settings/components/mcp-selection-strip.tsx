import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertTitle,
  Button,
  MetadataList,
  MetadataListRow,
} from "@compozy/ui";
import { Pencil, Trash2 } from "lucide-react";

import { composeMCPRowStatus } from "../lib/mcp-status-view-model";
import type { SettingsMCPServerEntry } from "../types";

import {
  mcpAllocatedRuntimeName,
  mcpOwnerExtensionName,
  mcpServerProvenanceLine,
  mcpServerRowTestId,
  mcpTransportLabel,
} from "./mcp-server-labels";
import { MCPStatusCell } from "./mcp-status-cell";

export interface MCPSelectionStripProps {
  /** The selected definition — full identity, so a same-name row never stands in for it. */
  server: SettingsMCPServerEntry;
  onEdit: (entry: SettingsMCPServerEntry) => void;
  /** Present for manual definitions only; extension-provided servers leave through the extension. */
  onRemove?: (entry: SettingsMCPServerEntry) => void;
  /** Clears the selection when the page model can drop it. */
  onClear?: () => void;
}

/**
 * Selection surface for the row a name click chose: where it comes from, how it runs, the
 * per-signal status breakdown the row's single status summarizes, and the actions that act on
 * exactly that definition. Delete is offered for manual definitions only; an extension-provided
 * server is removed by removing the extension.
 */
export function MCPSelectionStrip({ server, onEdit, onRemove, onClear }: MCPSelectionStripProps) {
  const extension = mcpOwnerExtensionName(server.owner);
  const runtimeName = mcpAllocatedRuntimeName(server);
  const status = composeMCPRowStatus(server);
  const rowTestId = mcpServerRowTestId(server);
  const endpoint =
    server.transport === "stdio"
      ? [server.command, ...(server.args ?? [])].filter(Boolean).join(" ")
      : server.url;
  return (
    <Alert data-testid="settings-page-mcp-selection" role="region" aria-label={server.name}>
      <AlertTitle>
        <span className="font-mono">{server.name}</span>
        {extension ? <span className="ml-1.5 font-normal text-muted">from {extension}</span> : null}
      </AlertTitle>
      <AlertDescription className="flex flex-col gap-3">
        <span data-testid="settings-page-mcp-selection-provenance">
          {mcpServerProvenanceLine(server)}
          {runtimeName ? ` · agents see it as ${runtimeName}` : null}
          {extension
            ? " · your edits are saved on top of the extension; remove the extension to remove the server."
            : null}
        </span>
        <MetadataList>
          <MetadataListRow label={mcpTransportLabel(server.transport)}>
            <span
              className="block truncate font-mono text-mono-id text-muted"
              data-testid={`${rowTestId}-endpoint`}
            >
              {endpoint || "-"}
            </span>
          </MetadataListRow>
          {runtimeName ? (
            <MetadataListRow label="Agents see">
              <span
                className="font-mono text-mono-id text-muted"
                data-testid={`${rowTestId}-runtime-name`}
              >
                {runtimeName}
              </span>
            </MetadataListRow>
          ) : null}
          <MetadataListRow label="Setup">
            <MCPStatusCell cell={status.config} testId={`${rowTestId}-config`} />
          </MetadataListRow>
          <MetadataListRow label="Sign-in">
            <MCPStatusCell cell={status.auth} testId={`${rowTestId}-auth`} />
          </MetadataListRow>
          <MetadataListRow label="Connection">
            <MCPStatusCell cell={status.runtime} testId={`${rowTestId}-runtime`} />
          </MetadataListRow>
          <MetadataListRow label="Tools">
            <MCPStatusCell cell={status.probe} testId={`${rowTestId}-probe`} />
          </MetadataListRow>
        </MetadataList>
      </AlertDescription>
      <AlertActions>
        <Button
          data-testid="settings-page-mcp-selection-edit"
          onClick={() => onEdit(server)}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Pencil aria-hidden="true" className="size-3" />
          Edit
        </Button>
        {onRemove ? (
          <Button
            className="text-danger"
            data-testid="settings-page-mcp-selection-delete"
            onClick={() => onRemove(server)}
            size="sm"
            type="button"
            variant="ghost"
          >
            <Trash2 aria-hidden="true" className="size-3" />
            Delete
          </Button>
        ) : null}
        {onClear ? (
          <Button
            data-testid="settings-page-mcp-selection-clear"
            onClick={onClear}
            size="sm"
            type="button"
            variant="ghost"
          >
            Clear
          </Button>
        ) : null}
      </AlertActions>
    </Alert>
  );
}

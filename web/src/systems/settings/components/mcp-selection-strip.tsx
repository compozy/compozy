import { Alert, AlertActions, AlertDescription, AlertTitle, Button } from "@compozy/ui";
import { CircleCheck, Pencil, Trash2 } from "lucide-react";

import type { SettingsMCPServerEntry } from "../types";

import {
  mcpAllocatedRuntimeName,
  mcpOwnerExtensionName,
  mcpServerProvenanceLine,
} from "./mcp-server-labels";

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
 * Selection surface for the row a name click chose: its provenance sentence and the actions
 * that act on exactly that definition. Delete is offered for manual definitions only; an
 * extension-provided server is removed by removing the extension.
 */
export function MCPSelectionStrip({ server, onEdit, onRemove, onClear }: MCPSelectionStripProps) {
  const extension = mcpOwnerExtensionName(server.owner);
  const runtimeName = mcpAllocatedRuntimeName(server);
  return (
    <Alert variant="info" data-testid="settings-page-mcp-selection">
      <CircleCheck />
      <AlertTitle>
        <span className="font-mono">{server.name}</span>
        {extension ? (
          <span className="ml-1.5 font-mono text-mono-id font-normal text-muted">
            {server.owner}
          </span>
        ) : null}{" "}
        selected
      </AlertTitle>
      <AlertDescription data-testid="settings-page-mcp-selection-provenance">
        {mcpServerProvenanceLine(server)}
        {runtimeName ? ` · runs as ${runtimeName}` : null}
        {extension
          ? " · edits are stored as an override; remove the extension to remove the server."
          : null}
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

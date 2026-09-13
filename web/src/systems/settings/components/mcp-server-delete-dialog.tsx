import { Trash2 } from "lucide-react";

import { ConfirmDialog } from "@compozy/ui";

import { deriveMCPManagementFilter, mcpManagementScopeLabel } from "../lib/mcp-management-target";
import type { SettingsMCPServerEntry } from "../types";

import { mcpTargetLabel } from "./mcp-server-labels";
import { SettingsSourceBadge } from "./settings-source-badge";

interface MCPServerDeleteDialogProps {
  /** A manual definition only; extension-provided servers are never deleted here. */
  target: SettingsMCPServerEntry | null;
  error: string | null;
  isDeleting: boolean;
  onClose: () => void;
  onConfirm: () => void;
}

/**
 * Deletes exactly the definition the daemon resolved for the row — its effective source and
 * scope, not the scope selected on the page — and names what becomes effective afterwards.
 */
export function MCPServerDeleteDialog({
  target,
  error,
  isDeleting,
  onClose,
  onConfirm,
}: MCPServerDeleteDialogProps) {
  const open = Boolean(target);
  const shadowed = target?.source_metadata.shadowed_sources ?? [];
  const hasShadowed = shadowed.length > 0;
  const effective = target?.source_metadata.effective_source;
  const management = target ? deriveMCPManagementFilter(target) : null;
  const scopeLabel = target ? mcpManagementScopeLabel(target) : null;
  const writeTarget = management?.target ? mcpTargetLabel(management.target) : null;

  return (
    <ConfirmDialog
      open={open}
      title={target ? `Delete MCP server "${target.name}"?` : "Delete MCP server"}
      description={
        target
          ? `Removes the definition from ${writeTarget ?? "its effective source"}${
              scopeLabel ? ` in ${scopeLabel}` : ""
            }. Other sources for this server remain untouched.`
          : null
      }
      note={
        target ? (
          <div className="flex flex-col gap-2">
            {effective ? (
              <div className="flex flex-col gap-1">
                <span className="font-medium">Current effective source</span>
                <SettingsSourceBadge
                  data-testid="settings-mcp-servers-delete-effective"
                  source={effective}
                />
              </div>
            ) : null}
            {hasShadowed ? (
              <div
                className="flex flex-col gap-1"
                data-testid="settings-mcp-servers-delete-shadowed"
              >
                <span className="font-medium">After delete, this becomes effective</span>
                <div className="flex flex-wrap items-center gap-1.5">
                  <SettingsSourceBadge source={shadowed[0]} />
                </div>
                <span>
                  Lower-precedence definitions remain on disk and become the next source CompozyOS
                  reads at restart.
                </span>
              </div>
            ) : (
              <span data-testid="settings-mcp-servers-delete-no-shadowed">
                No other sources define this server -- it will be fully removed after delete.
              </span>
            )}
          </div>
        ) : null
      }
      error={error}
      isPending={isDeleting}
      cancelLabel="Cancel"
      confirmLabel="Delete definition"
      confirmIcon={Trash2}
      contentProps={{ "data-testid": "settings-mcp-servers-delete" }}
      noteProps={{ "data-testid": "settings-mcp-servers-delete-fallback" }}
      errorProps={{ "data-testid": "settings-mcp-servers-delete-error" }}
      cancelButtonProps={{
        "data-testid": "settings-mcp-servers-delete-cancel",
        disabled: isDeleting,
      }}
      confirmButtonProps={{
        "data-testid": "settings-mcp-servers-delete-confirm",
      }}
      onConfirm={onConfirm}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    />
  );
}

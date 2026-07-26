import { Trash2 } from "lucide-react";

import type { SettingsMCPServerEntry, SettingsMCPServerTarget } from "../types";
import { ConfirmDialog, NativeSelect, NativeSelectOption } from "@agh/ui";

import { mcpTargetLabel } from "./mcp-server-labels";
import { SettingsSourceBadge } from "./settings-source-badge";

interface MCPServerDeleteDialogProps {
  target: SettingsMCPServerEntry | null;
  selectedTarget: SettingsMCPServerTarget;
  availableTargets: SettingsMCPServerTarget[];
  error: string | null;
  isDeleting: boolean;
  onTargetChange: (target: SettingsMCPServerTarget) => void;
  onClose: () => void;
  onConfirm: () => void;
}

export function MCPServerDeleteDialog({
  target,
  selectedTarget,
  availableTargets,
  error,
  isDeleting,
  onTargetChange,
  onClose,
  onConfirm,
}: MCPServerDeleteDialogProps) {
  const open = Boolean(target);
  const shadowed = target?.source_metadata.shadowed_sources ?? [];
  const hasShadowed = shadowed.length > 0;
  const effective = target?.source_metadata.effective_source;

  return (
    <ConfirmDialog
      open={open}
      title={target ? `Delete MCP server "${target.name}"?` : "Delete MCP server"}
      description={
        target
          ? selectedTarget === "auto"
            ? "Removes the highest-precedence definition in the selected scope. Lower-precedence definitions may become effective again."
            : `Removes the definition from the selected target (${mcpTargetLabel(selectedTarget)}). Other sources for this server remain untouched.`
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
                  Lower-precedence definitions remain on disk and become the next source the daemon
                  reads at restart.
                </span>
              </div>
            ) : (
              <span data-testid="settings-mcp-servers-delete-no-shadowed">
                No other sources define this server -- it will be fully removed after delete.
              </span>
            )}
            <div
              className="flex items-center gap-2"
              data-testid="settings-mcp-servers-delete-target"
            >
              <label
                htmlFor="settings-mcp-servers-delete-target-input"
                className="eyebrow text-muted"
              >
                target
              </label>
              <NativeSelect
                id="settings-mcp-servers-delete-target-input"
                className="w-56 font-mono"
                data-testid="settings-mcp-servers-delete-target-input"
                value={selectedTarget}
                onChange={event => onTargetChange(event.target.value as SettingsMCPServerTarget)}
              >
                {availableTargets.map(candidate => (
                  <NativeSelectOption key={candidate} value={candidate}>
                    {mcpTargetLabel(candidate)}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>
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

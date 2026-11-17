import { Trash2 } from "@compozy/ui/icons";

import { ConfirmDialog } from "@compozy/ui";

import type { SettingsMCPServerEntry } from "../types";

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
 * Removes exactly the definition the daemon resolved for the row — its effective source and
 * scope, not the scope selected on the page. The daemon also deletes the keys and sign-in saved
 * for it; when another source defines the same server, that copy takes over and the note says so.
 */
export function MCPServerDeleteDialog({
  target,
  error,
  isDeleting,
  onClose,
  onConfirm,
}: MCPServerDeleteDialogProps) {
  const next = target?.source_metadata.shadowed_sources?.[0];

  return (
    <ConfirmDialog
      open={Boolean(target)}
      tone="destructive"
      icon={Trash2}
      title={target ? `Remove ${target.name}?` : "Remove server?"}
      description="Sessions stop using it from their next message, and the keys and sign-in saved for it are deleted. You can add it again anytime."
      note={
        target && next ? (
          <span data-testid="settings-mcp-servers-delete-shadowed">
            <SettingsSourceBadge source={next} /> also has a{" "}
            <span className="font-mono">{target.name}</span> server. That one takes over after you
            remove this one.
          </span>
        ) : undefined
      }
      noteTone="info"
      error={error}
      isPending={isDeleting}
      cancelLabel="Cancel"
      confirmLabel="Remove server"
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
      onOpenChange={open => {
        if (!open) onClose();
      }}
    />
  );
}

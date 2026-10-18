import { Copy, FileCode, Pencil, Play, Search, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  TopbarOverflowIcon,
} from "@compozy/ui";

import type { AutomationView } from "../../lib/automation-view";

export interface AutomationRunNowState {
  pending: boolean;
  /** Automations are unavailable: the daemon would refuse the run. */
  disabled: boolean;
}

interface AutomationDetailActionsProps {
  view: AutomationView;
  runNow: AutomationRunNowState;
  onRunNow: () => void;
  onEdit: () => void;
}

/**
 * Route-chrome actions. Run now exists only for schedules (the daemon has no
 * run-now route for triggers) and works while Off. Edit is a ghost button for
 * automations created here; for config/package sources it is absent, not
 * disabled — the API rejects every field but `enabled`.
 */
export function AutomationDetailActions({
  view,
  runNow,
  onRunNow,
  onEdit,
}: AutomationDetailActionsProps) {
  if (!view.canRunNow && !view.canEdit) return null;
  return (
    <>
      {view.canRunNow ? (
        <Button
          data-testid="automation-run-now-btn"
          disabled={runNow.disabled || runNow.pending}
          onClick={onRunNow}
          size="sm"
          type="button"
          variant="neutral"
        >
          <Play aria-hidden="true" />
          {runNow.pending ? "Starting…" : "Run now"}
        </Button>
      ) : null}
      {view.canEdit ? (
        <Button
          data-testid="automation-edit-btn"
          onClick={onEdit}
          size="sm"
          type="button"
          variant="ghost"
        >
          <Pencil aria-hidden="true" />
          Edit
        </Button>
      ) : null}
    </>
  );
}

interface AutomationDetailOverflowProps {
  view: AutomationView;
  onInspect: () => void;
  onDelete: () => void;
}

/**
 * Copy id and Inspect for every automation. Delete only for one created here;
 * a config-owned one says where to edit it instead (BR 11).
 */
export function AutomationDetailOverflow({
  view,
  onInspect,
  onDelete,
}: AutomationDetailOverflowProps) {
  const handleCopyId = () => {
    const clipboard = navigator.clipboard;
    if (!clipboard) {
      toast.error("Could not copy the automation id.");
      return;
    }
    void clipboard
      .writeText(view.id)
      .then(() => toast.success(`Copied ${view.id}.`))
      .catch(() => toast.error("Could not copy the automation id."));
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More actions"
        data-testid="automation-detail-overflow"
        render={<Button size="icon-sm" type="button" variant="ghost" />}
      >
        <TopbarOverflowIcon aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="automation-detail-overflow-menu">
        <DropdownMenuItem data-testid="automation-copy-id-btn" onClick={handleCopyId}>
          <Copy aria-hidden="true" />
          Copy automation id
        </DropdownMenuItem>
        <DropdownMenuItem data-testid="automation-inspect-menu-btn" onClick={onInspect}>
          <Search aria-hidden="true" />
          Inspect
        </DropdownMenuItem>
        {view.source === "config" ? (
          <DropdownMenuItem data-testid="automation-edit-in-config" disabled>
            <FileCode aria-hidden="true" />
            Edit in config.toml
          </DropdownMenuItem>
        ) : null}
        {view.canEdit ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              data-testid="automation-delete-btn"
              onClick={onDelete}
              variant="destructive"
            >
              <Trash2 aria-hidden="true" />
              Delete automation…
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

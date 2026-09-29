import { Pencil, Play } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  TopbarOverflowIcon,
} from "@compozy/ui";

interface AutomationDetailActionsProps {
  onTriggerNow?: () => void;
  triggerDisabled: boolean;
  triggerPending: boolean;
}

function AutomationDetailActions({
  onTriggerNow,
  triggerDisabled,
  triggerPending,
}: AutomationDetailActionsProps) {
  return (
    <div className="flex items-center gap-2" data-testid="automation-detail-actions">
      {onTriggerNow ? (
        <Button
          data-testid="trigger-job-btn"
          disabled={triggerDisabled || triggerPending}
          onClick={onTriggerNow}
          size="sm"
          type="button"
        >
          <Play className="size-3" />
          {triggerPending ? "Starting…" : "Run now"}
        </Button>
      ) : null}
    </div>
  );
}

interface AutomationDetailOverflowProps {
  onDelete: () => void;
  onEdit: () => void;
}

/** Edit and Delete for user-created jobs; the enable switch lives in the page head. */
function AutomationDetailOverflow({ onDelete, onEdit }: AutomationDetailOverflowProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More actions"
        data-testid="automation-detail-overflow"
        render={<Button type="button" variant="ghost" size="icon-sm" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="automation-detail-overflow-menu">
        <DropdownMenuItem data-testid="edit-automation-btn" onClick={onEdit}>
          <Pencil className="size-3" />
          Edit
        </DropdownMenuItem>
        <DropdownMenuItem
          data-testid="delete-automation-btn"
          onClick={onDelete}
          variant="destructive"
        >
          Delete job
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export { AutomationDetailActions, AutomationDetailOverflow };

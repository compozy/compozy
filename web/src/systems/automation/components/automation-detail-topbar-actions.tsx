import { Pencil, Play } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  TopbarOverflowIcon,
} from "@agh/ui";

import type { AutomationJob, AutomationTrigger } from "../types";

interface AutomationDetailActionsProps {
  item: AutomationJob | AutomationTrigger;
  onToggleEnabled: (enabled: boolean) => void;
  onTriggerNow?: () => void;
  state: {
    togglePending: boolean;
    triggerDisabled: boolean;
    triggerPending: boolean;
  };
}

function AutomationDetailActions({
  item,
  onToggleEnabled,
  onTriggerNow,
  state,
}: AutomationDetailActionsProps) {
  const isDynamic = item.source === "dynamic";
  const showOverflow = isDynamic || Boolean(onTriggerNow);
  return (
    <div className="flex items-center gap-2" data-testid="automation-detail-actions">
      {onTriggerNow ? (
        <Button
          data-testid="trigger-job-btn"
          disabled={state.triggerDisabled || state.triggerPending}
          onClick={onTriggerNow}
          size="sm"
          type="button"
        >
          <Play className="size-3" />
          {state.triggerPending ? "Queuing..." : "Run now"}
        </Button>
      ) : null}
      {!showOverflow ? (
        <Button
          data-testid="toggle-automation-btn"
          disabled={state.togglePending}
          onClick={() => onToggleEnabled(!item.enabled)}
          size="sm"
          type="button"
          variant={item.enabled ? "neutral" : "default"}
        >
          {state.togglePending ? "Saving..." : item.enabled ? "Disable" : "Enable"}
        </Button>
      ) : null}
    </div>
  );
}

interface AutomationDetailOverflowProps {
  isTogglePending: boolean;
  item: AutomationJob | AutomationTrigger;
  kind: "jobs" | "triggers";
  onDelete: () => void;
  onEdit: () => void;
  onToggleEnabled: (enabled: boolean) => void;
}

function AutomationDetailOverflow({
  isTogglePending,
  item,
  kind,
  onDelete,
  onEdit,
  onToggleEnabled,
}: AutomationDetailOverflowProps) {
  const isDynamic = item.source === "dynamic";
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
        {isDynamic ? (
          <DropdownMenuItem data-testid="edit-automation-btn" onClick={onEdit}>
            <Pencil className="size-3" />
            Edit
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem
          data-testid="toggle-automation-btn"
          disabled={isTogglePending}
          onClick={() => onToggleEnabled(!item.enabled)}
        >
          {isTogglePending ? "Saving..." : item.enabled ? "Disable" : "Enable"}
        </DropdownMenuItem>
        {isDynamic ? (
          <DropdownMenuItem
            data-testid="delete-automation-btn"
            onClick={onDelete}
            variant="destructive"
          >
            Delete {kind === "jobs" ? "job" : "trigger"}
          </DropdownMenuItem>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export { AutomationDetailActions, AutomationDetailOverflow };

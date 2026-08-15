import { Copy, Pencil } from "lucide-react";
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

import type { AutomationTrigger } from "../../types";

/**
 * Edit sits in the route chrome for a trigger the operator owns. For a
 * config- or package-owned trigger it is absent rather than disabled: the API
 * rejects every field but `enabled`, so there is nothing to open.
 */
export function TriggerDetailActions({
  trigger,
  onEdit,
}: {
  trigger: AutomationTrigger;
  onEdit: () => void;
}) {
  if (trigger.source !== "dynamic") return null;
  return (
    <Button data-testid="edit-trigger-btn" onClick={onEdit} size="sm" type="button" variant="ghost">
      <Pencil className="size-3" />
      Edit
    </Button>
  );
}

interface TriggerDetailOverflowProps {
  trigger: AutomationTrigger;
  isTogglePending: boolean;
  onDelete: () => void;
  onEdit: () => void;
  onToggleEnabled: (enabled: boolean) => void;
}

/** Copy id for every trigger, enable for every trigger, delete only for one you own. */
export function TriggerDetailOverflow({
  trigger,
  isTogglePending,
  onDelete,
  onEdit,
  onToggleEnabled,
}: TriggerDetailOverflowProps) {
  const isDynamic = trigger.source === "dynamic";
  const handleCopyId = () => {
    const clipboard = navigator.clipboard;
    if (!clipboard) {
      toast.error("Could not copy the trigger id.");
      return;
    }
    void clipboard
      .writeText(trigger.id)
      .then(() => toast.success(`Copied ${trigger.id}.`))
      .catch(() => toast.error("Could not copy the trigger id."));
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More actions"
        data-testid="automation-detail-overflow"
        render={<Button size="icon-sm" type="button" variant="ghost" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="automation-detail-overflow-menu">
        <DropdownMenuItem data-testid="copy-trigger-id-btn" onClick={handleCopyId}>
          <Copy className="size-3" />
          Copy trigger id
        </DropdownMenuItem>
        {isDynamic ? (
          <DropdownMenuItem data-testid="edit-automation-btn" onClick={onEdit}>
            <Pencil className="size-3" />
            Edit
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem
          data-testid="toggle-automation-btn"
          disabled={isTogglePending}
          onClick={() => onToggleEnabled(!trigger.enabled)}
        >
          {isTogglePending ? "Saving..." : trigger.enabled ? "Disable" : "Enable"}
        </DropdownMenuItem>
        {isDynamic ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              data-testid="delete-automation-btn"
              onClick={onDelete}
              variant="destructive"
            >
              Delete trigger
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

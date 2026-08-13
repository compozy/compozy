import { useState } from "react";
import { Globe } from "lucide-react";

import { Icon, Toggle, Tooltip, TooltipContent, TooltipTrigger, cn } from "@compozy/ui";

import { GLOBAL_SCOPE_COPY } from "@/systems/workspace";

export interface GlobalScopeToggleProps {
  checked: boolean;
  locked?: boolean;
  /** Why the toggle is locked; composed into the accessible name. */
  lockedReason?: string;
  tooltip: string;
  onCheckedChange: (checked: boolean) => void;
}

/**
 * Menubar owner of Global vs workspace destination. The globe Toggle sits
 * between the mark and the workspace chip, outside `role="menubar"` — it is a
 * mode control, not a menu.
 */
export function GlobalScopeToggle({
  checked,
  locked = false,
  lockedReason,
  tooltip,
  onCheckedChange,
}: GlobalScopeToggleProps) {
  const [prevChecked, setPrevChecked] = useState(checked);
  const [announcement, setAnnouncement] = useState("");
  if (checked !== prevChecked) {
    setPrevChecked(checked);
    setAnnouncement(checked ? GLOBAL_SCOPE_COPY.liveOn : GLOBAL_SCOPE_COPY.liveOff);
  }

  const accessibleName =
    locked && lockedReason
      ? `${GLOBAL_SCOPE_COPY.controlName} — ${lockedReason}`
      : GLOBAL_SCOPE_COPY.controlName;

  const handlePressedChange = (next: boolean) => {
    if (locked) return;
    onCheckedChange(next);
  };

  return (
    <span
      className={cn("inline-flex shrink-0", locked && "cursor-not-allowed")}
      data-slot="os-global-scope-toggle"
    >
      <Tooltip>
        <TooltipTrigger
          render={
            <Toggle
              aria-disabled={locked || undefined}
              aria-label={accessibleName}
              className={cn(
                "size-7 min-w-7 p-0 text-muted hover:bg-btn-default-fill hover:text-fg-strong",
                "aria-pressed:bg-elevated aria-pressed:text-accent aria-pressed:shadow-highlight",
                "aria-pressed:hover:text-accent data-[state=on]:bg-elevated data-[state=on]:text-accent",
                "data-[state=on]:shadow-highlight data-[state=on]:hover:text-accent",
                "aria-disabled:hover:bg-transparent aria-disabled:hover:text-muted",
                "aria-disabled:aria-pressed:text-accent aria-disabled:aria-pressed:hover:text-accent",
                locked && "opacity-50"
              )}
              data-testid="os-global-scope-toggle"
              onPressedChange={handlePressedChange}
              pressed={checked}
            >
              <Icon as={Globe} size="lg" />
            </Toggle>
          }
        />
        <TooltipContent align="start" side="bottom">
          {tooltip}
        </TooltipContent>
      </Tooltip>
      <span aria-live="polite" className="sr-only">
        {announcement}
      </span>
    </span>
  );
}

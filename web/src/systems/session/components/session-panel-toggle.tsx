import { Button, cn, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
import { List, PanelRight } from "lucide-react";

const PANELS = {
  sidebar: { label: "sessions sidebar", testId: "session-sidebar-toggle", Glyph: List },
  inspector: { label: "context sidebar", testId: "session-inspector-toggle", Glyph: PanelRight },
} as const;

export function SessionPanelToggle({
  panel,
  open,
  onToggle,
}: {
  panel: keyof typeof PANELS;
  open: boolean;
  onToggle: () => void;
}) {
  const { label, testId, Glyph } = PANELS[panel];
  const name = `${open ? "Close" : "Open"} ${label}`;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={name}
            aria-pressed={open}
            className={cn(
              "size-11 focus-visible:shadow-focus-inset",
              open ? "bg-selected text-fg" : null
            )}
            data-state={open ? "open" : "closed"}
            data-testid={testId}
            onClick={onToggle}
          />
        }
      >
        <Glyph aria-hidden="true" className="size-3.5" />
      </TooltipTrigger>
      <TooltipContent>{name}</TooltipContent>
    </Tooltip>
  );
}

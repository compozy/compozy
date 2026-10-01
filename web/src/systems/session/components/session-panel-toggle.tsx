import { Button, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
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
            variant="quiet"
            size="icon-sm"
            aria-label={name}
            // Pressed reads as the held hover plate (`surface-2` + `fg`), from the variant.
            aria-pressed={open}
            data-state={open ? "open" : "closed"}
            data-testid={testId}
            onClick={onToggle}
          />
        }
      >
        <Glyph aria-hidden="true" className="size-4" />
      </TooltipTrigger>
      <TooltipContent>{name}</TooltipContent>
    </Tooltip>
  );
}

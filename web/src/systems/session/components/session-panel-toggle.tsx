import { Button, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
import { FileDiff, PanelLeft, PanelRight } from "@compozy/ui/icons";

/**
 * A toggle's accessible name: "Open"/"Close" plus the panel, or one fixed `name` (the Changes
 * toggle reads "Changes" either way; `aria-pressed` carries its state).
 */
const PANELS = {
  sidebar: { label: "sessions sidebar", testId: "session-sidebar-toggle", Glyph: PanelLeft },
  review: { name: "Changes", testId: "session-review-toggle", Glyph: FileDiff },
  inspector: { label: "context sidebar", testId: "session-inspector-toggle", Glyph: PanelRight },
} as const;

function panelName(panel: (typeof PANELS)[keyof typeof PANELS], open: boolean): string {
  return "name" in panel ? panel.name : `${open ? "Close" : "Open"} ${panel.label}`;
}

export function SessionPanelToggle({
  panel,
  open,
  onToggle,
}: {
  panel: keyof typeof PANELS;
  open: boolean;
  onToggle: () => void;
}) {
  const { testId, Glyph } = PANELS[panel];
  const name = panelName(PANELS[panel], open);
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="quiet"
            size="icon-sm"
            aria-label={name}
            // Pressed reads as the glyph in brand ink with no plate, from the quiet variant.
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

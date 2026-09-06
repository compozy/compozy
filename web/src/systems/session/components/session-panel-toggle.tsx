import { Button, cn } from "@compozy/ui";
import { List, PanelRight } from "lucide-react";

const PANELS = {
  sidebar: { label: "sessions sidebar", testId: "session-sidebar-toggle", Glyph: List },
  inspector: { label: "session inspector", testId: "session-inspector-toggle", Glyph: PanelRight },
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
  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      aria-label={`${open ? "Close" : "Open"} ${label}`}
      aria-pressed={open}
      className={cn(
        "size-11 focus-visible:shadow-focus-inset",
        open ? "bg-elevated text-fg" : null
      )}
      data-state={open ? "open" : "closed"}
      data-testid={testId}
      onClick={onToggle}
    >
      <Glyph aria-hidden="true" className="size-3" />
    </Button>
  );
}

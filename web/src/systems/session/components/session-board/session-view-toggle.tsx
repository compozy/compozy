import { List, SquareKanban } from "@compozy/ui/icons";

import { ToggleGroup, ToggleGroupItem } from "@compozy/ui";

import type { SessionWindowViewMode } from "../../hooks/use-session-sidebar-state";

export interface SessionViewToggleProps {
  value: SessionWindowViewMode;
  onValueChange: (view: SessionWindowViewMode) => void;
}

function isViewMode(value: unknown): value is SessionWindowViewMode {
  return value === "list" || value === "kanban";
}

/**
 * "List | Kanban" in the session window identity zone (US-011, `_uiux.md` S3).
 * A controlled single-select group: arrow keys move focus, Enter or Space
 * selects, and an empty selection is ignored so exactly one mode stays on.
 */
export function SessionViewToggle({ value, onValueChange }: SessionViewToggleProps) {
  return (
    <ToggleGroup
      aria-label="View"
      data-testid="session-view-toggle"
      multiple={false}
      onValueChange={next => {
        const selected: unknown = next[0];
        if (isViewMode(selected) && selected !== value) onValueChange(selected);
      }}
      size="sm"
      value={[value]}
    >
      <ToggleGroupItem data-testid="session-view-toggle-list" value="list">
        <List aria-hidden="true" data-icon="inline-start" />
        List
      </ToggleGroupItem>
      <ToggleGroupItem data-testid="session-view-toggle-kanban" value="kanban">
        <SquareKanban aria-hidden="true" data-icon="inline-start" />
        Kanban
      </ToggleGroupItem>
    </ToggleGroup>
  );
}

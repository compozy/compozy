import { Pencil, Play } from "@compozy/ui/icons";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  MenuTreeItems,
  TopbarOverflowIcon,
} from "@compozy/ui";

import { automationMenuNodes } from "../../lib/automation-menu";
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
  runNow: AutomationRunNowState;
  onRunNow: () => void;
  onEdit: () => void;
  onInspect: () => void;
  onDelete: () => void;
}

/** The detail page's `⋯`: the automation menu the list row's `…` also renders. */
export function AutomationDetailOverflow({
  view,
  runNow,
  onRunNow,
  onEdit,
  onInspect,
  onDelete,
}: AutomationDetailOverflowProps) {
  const nodes = automationMenuNodes(
    view,
    {
      runDisabled: runNow.disabled,
      runPending: runNow.pending,
      onRunNow,
      onEdit,
      onInspect,
      onDelete,
    },
    {
      editInConfig: "automation-edit-in-config",
      inspect: "automation-inspect-menu-btn",
      copyId: "automation-copy-id-btn",
      delete: "automation-delete-btn",
    }
  );
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
        <MenuTreeItems host="dropdown" nodes={nodes} />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

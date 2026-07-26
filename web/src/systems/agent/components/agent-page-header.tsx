import { Plus, Settings2 } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Pill,
  TopbarOverflowIcon,
} from "@agh/ui";

export interface AgentPageStatusPillProps {
  activeCount: number;
}

export function AgentPageStatusPill({ activeCount }: AgentPageStatusPillProps) {
  const status =
    activeCount > 0
      ? { label: "Active", tone: "success" as const }
      : { label: "Idle", tone: "neutral" as const };
  return (
    <Pill tone={status.tone} data-testid="agent-page-status">
      <Pill.Dot tone={status.tone} size="sm" />
      {status.label}
    </Pill>
  );
}

export interface AgentPageActionsProps {
  onNewSession: () => void;
  isCreatingSession: boolean;
  newSessionDisabled: boolean;
}

/** The single primary action in the agent-detail window head. */
export function AgentPageActions({
  onNewSession,
  isCreatingSession,
  newSessionDisabled,
}: AgentPageActionsProps) {
  return (
    <div className="flex items-center gap-2" data-testid="agent-page-toolbar">
      <Button
        type="button"
        variant="default"
        size="sm"
        onClick={onNewSession}
        disabled={newSessionDisabled}
        aria-busy={isCreatingSession}
        data-testid="agent-page-new-session"
      >
        <Plus aria-hidden="true" className="size-3" />
        New session
      </Button>
    </div>
  );
}

export interface AgentPageOverflowProps {
  onEditSettings: () => void;
  onDuplicate: () => void;
  onDelete: () => void;
}

/** Secondary agent-detail actions in the window-head overflow. */
export function AgentPageOverflow({
  onEditSettings,
  onDuplicate,
  onDelete,
}: AgentPageOverflowProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More agent actions"
        data-testid="agent-page-overflow"
        render={<Button type="button" variant="ghost" size="icon-sm" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="agent-page-overflow-menu">
        <DropdownMenuItem data-testid="agent-page-edit-settings" onClick={onEditSettings}>
          <Settings2 aria-hidden="true" className="size-3" />
          Edit settings
        </DropdownMenuItem>
        <DropdownMenuItem data-testid="agent-page-duplicate" onClick={onDuplicate}>
          Duplicate
        </DropdownMenuItem>
        <DropdownMenuItem variant="destructive" data-testid="agent-page-delete" onClick={onDelete}>
          Delete…
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

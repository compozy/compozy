import { ChevronRight, TriangleAlert } from "lucide-react";

import { cn, CollapsibleTrigger, Switch } from "@agh/ui";

import type { RoleViewModel } from "../lib/roles-view-model";
import { RoleStatusBadges } from "./role-status-badges";

export interface RolePanelHeaderProps {
  vm: RoleViewModel;
  bodyId: string;
  disabled?: boolean;
  testId: string;
  onEnabledChange: (enabled: boolean) => void;
}

/**
 * The collapsed read of one role: what it does, how it resolves, whether it is
 * on. The switch sits beside the disclosure trigger rather than inside it, so
 * the most common action never requires expanding the row.
 */
export function RolePanelHeader({
  vm,
  bodyId,
  disabled,
  testId,
  onEnabledChange,
}: RolePanelHeaderProps) {
  return (
    <div className="flex items-center gap-2 pr-4">
      <CollapsibleTrigger
        aria-controls={bodyId}
        className={cn(
          "group/role flex min-w-0 flex-1 items-start gap-2.5 py-3 pl-4 text-left",
          "transition-colors duration-base hover:bg-row-hover",
          "focus-visible:outline-none focus-visible:shadow-focus-inset"
        )}
        data-testid={`${testId}-toggle`}
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className="mt-0.5 size-3.5 shrink-0 text-faint transition-transform duration-base group-data-panel-open/role:rotate-90"
        />
        <span className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="text-ws-name font-medium text-fg-strong">{vm.label}</span>
          <span className="flex min-w-0 flex-wrap items-center gap-x-1.5 text-form-label text-muted">
            <span>{vm.description}</span>
            <span aria-hidden="true" className="text-faint">
              ·
            </span>
            <span className="text-subtle" data-testid={`${testId}-resolution`}>
              {vm.resolutionLine}
            </span>
          </span>
        </span>
      </CollapsibleTrigger>
      {vm.hasDiagnostics ? (
        <TriangleAlert
          aria-label="This role has a configuration warning"
          className="size-3.5 shrink-0 text-warning"
          data-testid={`${testId}-warning-mark`}
        />
      ) : null}
      {vm.routeSummary ? (
        <span
          className="hidden max-w-56 shrink-0 truncate font-mono text-badge text-subtle md:block"
          data-testid={`${testId}-route`}
        >
          {vm.routeSummary}
        </span>
      ) : null}
      <RoleStatusBadges badges={vm.badges} data-testid={`${testId}-badges`} />
      <Switch
        aria-label={`Enable ${vm.label}`}
        checked={vm.enabled}
        disabled={disabled}
        data-testid={`${testId}-enabled-switch`}
        onCheckedChange={onEnabledChange}
      />
    </div>
  );
}

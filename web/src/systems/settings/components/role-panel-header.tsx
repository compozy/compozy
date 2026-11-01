import { ChevronRight, TriangleAlert } from "@compozy/ui/icons";

import { cn, CollapsibleTrigger, Skeleton, SkeletonText, Switch } from "@compozy/ui";

import type { RoleViewModel } from "../lib/roles-view-model";

const HEADER_ROW_CLASS = "flex items-center gap-2 pr-4";
const TRIGGER_LAYOUT_CLASS = "flex min-w-0 flex-1 items-start gap-2.5 py-3 pl-4 text-left";
const TEXT_STACK_CLASS = "flex min-w-0 flex-1 flex-col gap-0.5";

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
    <div className={HEADER_ROW_CLASS}>
      <CollapsibleTrigger
        aria-controls={bodyId}
        className={cn(
          "group/role",
          TRIGGER_LAYOUT_CLASS,
          "transition-colors duration-base hover:bg-surface-2",
          "focus-visible:outline-none focus-visible:shadow-focus-inset"
        )}
        data-testid={`${testId}-toggle`}
        type="button"
      >
        <ChevronRight
          aria-hidden="true"
          className="mt-0.5 size-3.5 shrink-0 text-faint transition-transform duration-base group-data-panel-open/role:rotate-90"
        />
        <span className={TEXT_STACK_CLASS}>
          <span className="text-body font-medium text-fg">{vm.label}</span>
          <span className="flex min-w-0 flex-wrap items-center gap-x-1.5 text-form-label text-muted-foreground">
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

/** The collapsed read of one role in skeleton form: chevron, name, one-line summary and switch. */
export function RolePanelHeaderSkeleton() {
  return (
    <div aria-hidden="true" className={HEADER_ROW_CLASS}>
      <div className={TRIGGER_LAYOUT_CLASS}>
        <Skeleton className="mt-0.5 size-3.5 shrink-0 rounded-xs" />
        <span className={TEXT_STACK_CLASS}>
          <SkeletonText className="w-32 text-body font-medium" />
          <SkeletonText className="w-80 text-form-label" />
        </span>
      </div>
      <Skeleton className="h-switch-default w-switch-default rounded-pill" />
    </div>
  );
}

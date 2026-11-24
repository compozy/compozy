import { XIcon } from "@compozy/ui/icons";

import { Button, cn, StatusDot, Tabs, TabsList, TabsTrigger } from "@compozy/ui";

import { useComputerDetailsPanel } from "../hooks/use-computer-details-panel";
import type {
  ComputerFixture,
  ComputerProviderAuthFixture,
  RemoteScope,
} from "../mocks/remote-computers-fixtures";
import type { UpdateRunState } from "../mocks/remote-computers-fixtures-lane-b";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { ComputerStatusChip } from "./computer-status-chip";
import { ComputerDetailsStatusTab } from "./computer-details-status-tab";
import {
  ComputerDetailsProvidersTab,
  ComputerDetailsUpdateTab,
} from "./computer-details-providers-update-tabs";
import { ComputerDetailsPermissionsTab, ComputerDetailsRoutesTab } from "./computer-details-tabs";
import type { StepProgressItem } from "./step-progress-list";

export type ComputerDetailsTabId = "status" | "routes" | "permissions" | "providers" | "update";

export interface ComputerDetailsPanelProps {
  computer: ComputerFixture;
  open: boolean;
  initialTab?: ComputerDetailsTabId;
  /**
   * Positioning mode:
   * - `"viewport"` (default, used by standalone prototype drafts below global chrome)
   * - `"pane"` (docked flush inside `SettingsSectionOutlet` at `top-0`)
   */
  placement?: "viewport" | "pane";
  /** Enables interactive scope switches in the Permissions tab. */
  editablePermissions?: boolean;
  currentDeviceName?: string;
  /** Optional provider already expanded with the inline remote sign-in terminal preview. */
  initialSignInProviderId?: ComputerProviderAuthFixture["id"] | null;
  /** Initial state of the SSH update stepper when `status === "incompatible"`. */
  initialUpdateState?: UpdateRunState;
  liveUpdateState?: UpdateRunState;
  liveUpdateSteps?: readonly StepProgressItem[];
  showRouteReorder?: boolean;
  onRemoveRoute?: (routeId: string) => void;
  onRetryConnection?: () => void;
  onToggleScope?: (scope: RemoteScope, next: boolean) => void;
  onStartUpdate?: () => void;
  onCheckVersion?: () => void;
  onOpenChange?: (open: boolean) => void;
}

/**
 * Right-docked Computer details inspector inside the Settings window (`_uiux.md` U7):
 * - Opaque `bg-canvas` surface with a crisp left hairline, 460px wide, full window height
 *   below the Settings window header, and zero enter animation so states render cleanly.
 * - Tabs: Status, Routes, Permissions, Providers, plus conditional Update tab ONLY when incompatible.
 */
export function ComputerDetailsPanel({
  computer,
  open,
  initialTab = "status",
  placement = "viewport",
  editablePermissions = false,
  currentDeviceName,
  initialSignInProviderId = null,
  initialUpdateState = "idle",
  liveUpdateState,
  liveUpdateSteps,
  showRouteReorder = true,
  onRemoveRoute,
  onRetryConnection,
  onToggleScope,
  onStartUpdate,
  onCheckVersion,
  onOpenChange,
}: ComputerDetailsPanelProps) {
  const panel = useComputerDetailsPanel({
    computer,
    open,
    initialTab,
    onRemoveRoute,
    onToggleScope,
    onOpenChange,
  });

  if (!open) return null;

  const hasSignedOutProvider = computer.providers.some(item => item.status === "signed_out");
  const hasRouteMismatch = panel.routes.some(item => item.identityMismatch);
  const liveComputer: ComputerFixture = {
    ...computer,
    routes: panel.routes,
    scopes: panel.scopes,
  };

  return (
    <aside
      aria-label={`${computer.name} details`}
      className={cn(
        "absolute right-0 bottom-0 z-40 isolate flex w-[460px] max-w-[calc(100%-1.5rem)] flex-col overflow-hidden border-l border-line bg-canvas text-fg shadow-overlay",
        placement === "pane"
          ? "top-0"
          : "top-[calc(var(--height-menubar)+2.5rem+var(--height-window-head)+1px)]"
      )}
      data-slot="computer-details-panel"
    >
      <header className="flex shrink-0 items-start gap-3 border-b border-line px-5 pt-4 pb-3.5">
        <ComputerPlatformIcon
          className="mt-0.5 shrink-0"
          iconKind={computer.iconKind}
          platform={computer.platform}
          size="md"
          tone="muted"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <h2
              className="min-w-0 truncate text-item-title font-medium text-fg-strong"
              title={computer.name}
            >
              {computer.name}
            </h2>
            <ComputerStatusChip status={computer.status} />
          </div>
          <p className="truncate text-meta text-muted-foreground">
            {computer.version ? `CompozyOS ${computer.version}` : "CompozyOS"}
            {computer.routeInUse
              ? ` · ${computer.routeInUse.label}`
              : ` · Last connected ${computer.lastConnectedLabel}`}
          </p>
        </div>
        <Button
          aria-label="Close computer details"
          className="-mt-0.5 -mr-1.5 shrink-0 text-subtle hover:text-fg"
          onClick={() => onOpenChange?.(false)}
          size="icon-sm"
          type="button"
          variant="quiet"
        >
          <XIcon aria-hidden="true" className="size-4" />
        </Button>
      </header>

      <Tabs
        className="flex min-h-0 flex-1 flex-col gap-0"
        onValueChange={next => {
          if (typeof next === "string") panel.setActiveTab(next as ComputerDetailsTabId);
        }}
        value={panel.activeTab}
      >
        <TabsList className="shrink-0 overflow-x-auto px-5">
          <TabsTrigger value="status">Status</TabsTrigger>
          <TabsTrigger count={panel.routes.length} value="routes">
            <span className="inline-flex items-center gap-1.5">
              <span>Routes</span>
              {hasRouteMismatch ? <StatusDot size="sm" tone="warning" /> : null}
            </span>
          </TabsTrigger>
          <TabsTrigger value="permissions">Permissions</TabsTrigger>
          <TabsTrigger value="providers">
            <span className="inline-flex items-center gap-1.5">
              <span>Providers</span>
              {hasSignedOutProvider ? <StatusDot size="sm" tone="warning" /> : null}
            </span>
          </TabsTrigger>
          {panel.showUpdateTab ? (
            <TabsTrigger value="update">
              <span className="inline-flex items-center gap-1.5">
                <span>Update</span>
                <StatusDot size="sm" tone="warning" />
              </span>
            </TabsTrigger>
          ) : null}
        </TabsList>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4 [scrollbar-gutter:stable]">
          {panel.activeTab === "status" ? (
            <ComputerDetailsStatusTab
              computer={liveComputer}
              onRetry={onRetryConnection}
              onSelectTab={next => panel.setActiveTab(next)}
            />
          ) : null}

          {panel.activeTab === "routes" ? (
            <ComputerDetailsRoutesTab
              computerName={computer.name}
              onMoveRoute={panel.handleMoveRoute}
              onRemoveRoute={panel.handleRemoveRoute}
              routes={panel.routes}
              showReorder={showRouteReorder}
            />
          ) : null}

          {panel.activeTab === "permissions" ? (
            <ComputerDetailsPermissionsTab
              computerName={computer.name}
              currentDeviceName={currentDeviceName}
              editable={editablePermissions}
              onToggleScope={panel.handleToggleScope}
              scopes={panel.scopes}
              suppressToast={Boolean(onToggleScope)}
            />
          ) : null}

          {panel.activeTab === "providers" ? (
            <ComputerDetailsProvidersTab
              computerName={computer.name}
              initialSignInProviderId={initialSignInProviderId}
              providers={computer.providers}
            />
          ) : null}

          {panel.activeTab === "update" && panel.showUpdateTab ? (
            <ComputerDetailsUpdateTab
              computer={liveComputer}
              initialUpdateState={initialUpdateState}
              liveUpdateState={liveUpdateState}
              liveUpdateSteps={liveUpdateSteps}
              onCheckVersion={onCheckVersion}
              onStartUpdate={onStartUpdate}
            />
          ) : null}
        </div>
      </Tabs>
    </aside>
  );
}

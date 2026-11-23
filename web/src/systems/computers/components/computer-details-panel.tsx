import { useEffect, useState } from "react";
import { XIcon } from "@compozy/ui/icons";

import { Button, StatusDot, Tabs, TabsList, TabsTrigger, toast } from "@compozy/ui";

import type {
  ComputerFixture,
  ComputerProviderAuthFixture,
  ComputerRouteFixture,
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

export type ComputerDetailsTabId = "status" | "routes" | "permissions" | "providers" | "update";

export interface ComputerDetailsPanelProps {
  computer: ComputerFixture;
  open: boolean;
  initialTab?: ComputerDetailsTabId;
  /** Enables interactive scope switches in the Permissions tab. */
  editablePermissions?: boolean;
  /** Optional provider already expanded with the inline remote sign-in terminal preview. */
  initialSignInProviderId?: ComputerProviderAuthFixture["id"] | null;
  /** Initial state of the SSH update stepper when `status === "incompatible"`. */
  initialUpdateState?: UpdateRunState;
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
  editablePermissions = false,
  initialSignInProviderId = null,
  initialUpdateState = "idle",
  onOpenChange,
}: ComputerDetailsPanelProps) {
  const showUpdateTab = computer.status === "incompatible";
  const resolvedInitialTab: ComputerDetailsTabId =
    initialTab === "update" && !showUpdateTab ? "status" : initialTab;

  const [activeTab, setActiveTab] = useState<ComputerDetailsTabId>(resolvedInitialTab);
  const [routes, setRoutes] = useState<ComputerRouteFixture[]>([...computer.routes]);
  const [scopes, setScopes] = useState<RemoteScope[]>([...computer.scopes]);

  useEffect(() => {
    if (!open || !onOpenChange) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onOpenChange(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, onOpenChange]);

  if (!open) return null;

  const hasSignedOutProvider = computer.providers.some(item => item.status === "signed_out");
  const hasRouteMismatch = routes.some(item => item.identityMismatch);

  const handleMoveRoute = (routeId: string, direction: "up" | "down") => {
    setRoutes(current => {
      const index = current.findIndex(item => item.id === routeId);
      if (index < 0) return current;
      const targetIndex = direction === "up" ? index - 1 : index + 1;
      if (targetIndex < 0 || targetIndex >= current.length) return current;
      const copy = [...current];
      const [moved] = copy.splice(index, 1);
      if (!moved) return current;
      copy.splice(targetIndex, 0, moved);
      return copy;
    });
    toast.message("Updated route priority order.");
  };

  const handleRemoveRoute = (routeId: string) => {
    setRoutes(current => current.filter(item => item.id !== routeId));
    toast.message(`Removed learned route from ${computer.name}.`);
  };

  const handleToggleScope = (scope: RemoteScope, next: boolean) => {
    setScopes(current => {
      const set = new Set(current);
      if (next) set.add(scope);
      else set.delete(scope);
      return [...set];
    });
  };

  const liveComputer: ComputerFixture = {
    ...computer,
    routes,
    scopes,
  };

  return (
    <aside
      aria-label={`${computer.name} details`}
      className="absolute top-[calc(var(--height-menubar)+2.5rem+var(--height-window-head)+1px)] right-0 bottom-0 z-40 isolate flex w-[460px] max-w-[calc(100%-1.5rem)] flex-col overflow-hidden border-l border-line bg-canvas text-fg shadow-overlay"
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
            CompozyOS {computer.version}
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
          if (typeof next === "string") setActiveTab(next as ComputerDetailsTabId);
        }}
        value={activeTab}
      >
        <TabsList className="shrink-0 overflow-x-auto px-5">
          <TabsTrigger value="status">Status</TabsTrigger>
          <TabsTrigger count={routes.length} value="routes">
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
          {showUpdateTab ? (
            <TabsTrigger value="update">
              <span className="inline-flex items-center gap-1.5">
                <span>Update</span>
                <StatusDot size="sm" tone="warning" />
              </span>
            </TabsTrigger>
          ) : null}
        </TabsList>

        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4 [scrollbar-gutter:stable]">
          {activeTab === "status" ? (
            <ComputerDetailsStatusTab
              computer={liveComputer}
              onSelectTab={next => setActiveTab(next)}
            />
          ) : null}

          {activeTab === "routes" ? (
            <ComputerDetailsRoutesTab
              computerName={computer.name}
              onMoveRoute={handleMoveRoute}
              onRemoveRoute={handleRemoveRoute}
              routes={routes}
            />
          ) : null}

          {activeTab === "permissions" ? (
            <ComputerDetailsPermissionsTab
              computerName={computer.name}
              editable={editablePermissions}
              onToggleScope={handleToggleScope}
              scopes={scopes}
            />
          ) : null}

          {activeTab === "providers" ? (
            <ComputerDetailsProvidersTab
              computerName={computer.name}
              initialSignInProviderId={initialSignInProviderId}
              providers={computer.providers}
            />
          ) : null}

          {activeTab === "update" && showUpdateTab ? (
            <ComputerDetailsUpdateTab
              computer={liveComputer}
              initialUpdateState={initialUpdateState}
            />
          ) : null}
        </div>
      </Tabs>
    </aside>
  );
}

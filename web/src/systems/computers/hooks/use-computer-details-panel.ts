import { useEffect, useState } from "react";

import { toast } from "@compozy/ui";

import type {
  ComputerFixture,
  ComputerRouteFixture,
  RemoteScope,
} from "../mocks/remote-computers-fixtures";
import type { ComputerDetailsTabId } from "../components/computer-details-panel";

interface UseComputerDetailsPanelInput {
  computer: ComputerFixture;
  open: boolean;
  initialTab: ComputerDetailsTabId;
  onRemoveRoute?: (routeId: string) => void;
  onToggleScope?: (scope: RemoteScope, next: boolean) => void;
  onOpenChange?: (open: boolean) => void;
}

export function useComputerDetailsPanel({
  computer,
  open,
  initialTab,
  onRemoveRoute,
  onToggleScope,
  onOpenChange,
}: UseComputerDetailsPanelInput) {
  const showUpdateTab = computer.status === "incompatible";
  const resolvedInitialTab: ComputerDetailsTabId =
    initialTab === "update" && !showUpdateTab ? "status" : initialTab;

  const [tabState, setTabState] = useState<{
    computerId: string;
    initialTab: ComputerDetailsTabId;
    tab: ComputerDetailsTabId;
  } | null>(null);
  const [routesState, setRoutesState] = useState<{
    computerId: string;
    source: readonly ComputerRouteFixture[];
    routes: ComputerRouteFixture[];
  } | null>(null);
  const [scopesState, setScopesState] = useState<{
    computerId: string;
    source: readonly RemoteScope[];
    scopes: RemoteScope[];
  } | null>(null);

  useEffect(() => {
    if (!open || !onOpenChange) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onOpenChange(false);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [open, onOpenChange]);

  const activeTab =
    tabState && tabState.computerId === computer.id && tabState.initialTab === resolvedInitialTab
      ? tabState.tab
      : resolvedInitialTab;

  const routes = onRemoveRoute
    ? computer.routes
    : routesState &&
        routesState.computerId === computer.id &&
        routesState.source === computer.routes
      ? routesState.routes
      : computer.routes;

  const scopes = onToggleScope
    ? computer.scopes
    : scopesState &&
        scopesState.computerId === computer.id &&
        scopesState.source === computer.scopes
      ? scopesState.scopes
      : computer.scopes;

  const setActiveTab = (tab: ComputerDetailsTabId) => {
    setTabState({ computerId: computer.id, initialTab: resolvedInitialTab, tab });
  };

  const handleMoveRoute = (routeId: string, direction: "up" | "down") => {
    const current = [...routes];
    const index = current.findIndex(item => item.id === routeId);
    if (index < 0) return;
    const targetIndex = direction === "up" ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= current.length) return;
    const [moved] = current.splice(index, 1);
    if (!moved) return;
    current.splice(targetIndex, 0, moved);
    setRoutesState({ computerId: computer.id, source: computer.routes, routes: current });
    toast.message("Updated route priority order.");
  };

  const handleRemoveRoute = (routeId: string) => {
    if (onRemoveRoute) {
      onRemoveRoute(routeId);
      return;
    }
    const next = routes.filter(item => item.id !== routeId);
    setRoutesState({ computerId: computer.id, source: computer.routes, routes: next });
    toast.message(`Removed learned route from ${computer.name}.`);
  };

  const handleToggleScope = (scope: RemoteScope, next: boolean) => {
    if (onToggleScope) {
      onToggleScope(scope, next);
      return;
    }
    const set = new Set(scopes);
    if (next) set.add(scope);
    else set.delete(scope);
    setScopesState({ computerId: computer.id, source: computer.scopes, scopes: [...set] });
  };

  return {
    showUpdateTab,
    activeTab,
    setActiveTab,
    routes,
    scopes,
    handleMoveRoute,
    handleRemoveRoute,
    handleToggleScope,
  };
}

import { AlertCircle, SearchCheck, Waypoints } from "lucide-react";

import { Button, cn, DataSurface, Eyebrow, PAGE_CONTENT_GUTTER, Skeleton } from "@agh/ui";

import { formatBridgeProviderConfig } from "../lib/bridge-formatters";
import type { BridgeSetupProjection } from "../lib/bridge-setup";
import type {
  BridgeHealth,
  BridgeProvider,
  BridgeRoute,
  BridgeSecretBinding,
  BridgeStatus,
  BridgeSummary,
} from "../types";
import { BridgeDetailConfiguration } from "./bridge-detail-configuration";
import { BridgeDetailHeader } from "./bridge-detail-header";
import { BridgeDetailMetrics } from "./bridge-detail-metrics";
import { BridgeEventStreamSection } from "./bridge-event-stream-section";
import { BridgeProviderRuntimeSection } from "./bridge-provider-runtime-section";
import { BridgeSetupChecklist } from "./bridge-setup-checklist";
import {
  BridgeTargetDirectorySection,
  type BridgeTargetDirectoryState,
} from "./bridge-target-directory-section";

interface BridgeDetailPanelProps {
  bridge: BridgeSummary | undefined;
  emptyMessage?: string;
  error: Error | null;
  health: BridgeHealth | undefined;
  onDeleteSecretBinding?: (bindingName: string) => void;
  onDisableBridge?: () => void;
  onEnableBridge?: () => void;
  onOpenEdit?: () => void;
  /** Opens the editor in Advanced, where the delivery test lives (D2). */
  onOpenDeliveryTest: () => void;
  onRestartBridge?: () => void;
  onSaveSecretBinding?: (bindingName: string) => void;
  onSecretDraftChange?: (bindingName: string, value: string) => void;
  provider?: BridgeProvider;
  restartRequired?: boolean;
  routes: BridgeRoute[];
  secretBindings?: BridgeSecretBinding[];
  secretInputValues?: Record<string, string>;
  setup: {
    isLifecyclePending: boolean;
    isRegistering: boolean;
    isVerifying: boolean;
    onRegisterWebhook: () => void;
    onVerify: () => void;
    projection: BridgeSetupProjection | null;
  };
  state: {
    isLifecyclePending?: boolean;
    isLoading: boolean;
    isProviderLoading?: boolean;
    isRoutesLoading: boolean;
    isSecretBindingPending?: boolean;
    isSecretBindingsLoading?: boolean;
    providerError?: Error | null;
    secretBindingsError?: Error | null;
  };
  targetDirectory?: BridgeTargetDirectoryState;
  workspaceName?: string | null;
}

const EMPTY_SECRET_BINDINGS: BridgeSecretBinding[] = [];
const EMPTY_SECRET_INPUT_VALUES: Record<string, string> = {};
const EMPTY_CHECKS_BY_SECRET_SLOT: BridgeSetupProjection["checksBySecretSlot"] = {};

export function BridgeDetailPanel({
  bridge,
  emptyMessage = "Select a bridge to inspect configuration, routes, and delivery health.",
  error,
  health,
  onDeleteSecretBinding,
  onDisableBridge,
  onEnableBridge,
  onOpenEdit,
  onOpenDeliveryTest,
  onRestartBridge,
  onSaveSecretBinding,
  onSecretDraftChange,
  provider,
  restartRequired = false,
  routes,
  secretBindings = EMPTY_SECRET_BINDINGS,
  secretInputValues = EMPTY_SECRET_INPUT_VALUES,
  setup,
  state,
  targetDirectory,
  workspaceName,
}: BridgeDetailPanelProps) {
  const {
    isLifecyclePending: stateLifecyclePending = false,
    isLoading,
    isProviderLoading = false,
    isRoutesLoading,
    isSecretBindingPending = false,
    isSecretBindingsLoading = false,
    providerError = null,
    secretBindingsError = null,
  } = state;
  const isLifecyclePending =
    setup.isLifecyclePending || stateLifecyclePending || setup.isRegistering || setup.isVerifying;

  if (isLoading) {
    return <BridgeDetailSkeleton />;
  }

  if (error || !bridge) {
    const surfaceState = error ? "error" : "empty";
    return (
      <DataSurface
        className="flex min-h-0 flex-1 items-center justify-center py-10"
        state={surfaceState}
      >
        <DataSurface.Error
          className="max-w-md"
          data-testid="bridge-detail-error"
          description={error?.message ?? "Failed to load bridge details"}
          icon={AlertCircle}
          title="Unable to load bridge"
        />
        <DataSurface.Empty
          className="max-w-md"
          data-testid="bridge-detail-empty"
          description={emptyMessage}
          icon={Waypoints}
          title="Select a bridge"
        />
      </DataSurface>
    );
  }

  const providerConfig = formatBridgeProviderConfig(bridge.provider_config);
  const effectiveStatus = (health?.status ?? bridge.status) as BridgeStatus;
  const bindingsByName = new Map(secretBindings.map(binding => [binding.binding_name, binding]));

  return (
    <section
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden")}
      data-testid="bridge-detail-panel"
    >
      <BridgeDetailHeader
        bridge={bridge}
        effectiveStatus={effectiveStatus}
        isLifecyclePending={isLifecyclePending}
        onDisableBridge={onDisableBridge}
        onEnableBridge={onEnableBridge}
        onOpenEdit={onOpenEdit}
        onRestartBridge={onRestartBridge}
      />

      <div className="min-h-0 flex-1 space-y-6 overflow-y-auto py-5">
        <BridgeSetupChecklist
          isLifecyclePending={isLifecyclePending}
          isRegistering={setup.isRegistering}
          isVerifying={setup.isVerifying}
          onEnableBridge={onEnableBridge}
          onOpenEdit={onOpenEdit}
          onRegisterWebhook={setup.onRegisterWebhook}
          onRestartBridge={onRestartBridge}
          onVerify={setup.onVerify}
          projection={setup.projection}
        />

        {restartRequired ? (
          <div
            className="rounded-md border border-warning/40 bg-warning-tint px-4 py-3 text-small-body text-warning"
            data-testid="bridge-restart-required"
          >
            Pending runtime changes require a restart or enable action before the provider picks
            them up.
          </div>
        ) : null}

        <BridgeDetailMetrics health={health} routes={routes} />
        <BridgeDetailConfiguration bridge={bridge} workspaceName={workspaceName} />
        <BridgeProviderRuntimeSection
          bindingsByName={bindingsByName}
          checksBySecretSlot={setup.projection?.checksBySecretSlot ?? EMPTY_CHECKS_BY_SECRET_SLOT}
          isProviderLoading={isProviderLoading}
          isSecretBindingPending={isSecretBindingPending}
          isSecretBindingsLoading={isSecretBindingsLoading}
          onDeleteSecretBinding={onDeleteSecretBinding}
          onSaveSecretBinding={onSaveSecretBinding}
          onSecretDraftChange={onSecretDraftChange}
          provider={provider}
          providerError={providerError}
          providerConfig={providerConfig}
          secretBindingsError={secretBindingsError}
          secretInputValues={secretInputValues}
        />
        <BridgeTargetDirectorySection state={targetDirectory} />
        <BridgeEventStreamSection isRoutesLoading={isRoutesLoading} routes={routes} />
        <div className="flex flex-wrap items-center justify-between gap-4 rounded-md border border-line bg-canvas-soft px-5 py-4">
          <div className="min-w-0 flex-1 space-y-1">
            <Eyebrow className="block text-muted">Delivery checks</Eyebrow>
            <p className="text-small-body text-muted">
              Resolve a target without provider side effects, or send one real message — both live
              in the editor&apos;s Advanced tier.
            </p>
          </div>
          <Button
            data-testid="open-test-delivery-btn"
            onClick={onOpenDeliveryTest}
            size="sm"
            type="button"
            variant="outline"
          >
            <SearchCheck className="size-3" />
            Test delivery
          </Button>
        </div>
      </div>
    </section>
  );
}

function BridgeDetailSkeleton() {
  return (
    <div
      aria-busy="true"
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden")}
      data-testid="bridge-detail-loading"
      role="status"
    >
      <div className="flex flex-col gap-2 border-b border-line py-4">
        <div className="flex items-center gap-3">
          <Skeleton className="size-9" />
          <Skeleton className="h-5 w-48" />
          <Skeleton className="ml-auto h-7 w-24" />
        </div>
        <Skeleton className="h-3 w-64" />
      </div>
      <div className="min-h-0 flex-1 space-y-6 overflow-hidden py-5">
        <Skeleton className="h-28 w-full" />
        <div className="grid grid-cols-3 gap-3">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-40 w-full" />
      </div>
      <span className="sr-only">Loading bridge details</span>
    </div>
  );
}

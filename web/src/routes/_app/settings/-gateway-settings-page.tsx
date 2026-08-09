import { AlertCircle } from "lucide-react";

import { Button, Spinner } from "@compozy/ui";

import {
  GatewayAuditPanel,
  GatewayDeviceList,
  GatewayExposureSection,
  GatewayPairingDialog,
  GatewayProviderSection,
  useGatewaySettingsPage,
  useGatewayAccessTier,
  type GatewaySettingsViewModel,
} from "@/systems/gateway";
import { SettingsPageFrame, useSettingsTopbar } from "@/systems/settings";

/**
 * The gateway operator surface: how this daemon is reachable, which devices
 * hold a session, and what the self-audit says about the current posture.
 */
export function GatewaySettingsPage() {
  const view = useGatewaySettingsPage();
  const listenerTier = useGatewayAccessTier();
  useSettingsTopbar("gateway");

  if (view.page.isLoading) {
    return (
      <div
        aria-label="Loading gateway settings"
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-gateway-loading"
        role="status"
      >
        <Spinner aria-hidden="true" className="size-5 text-subtle" />
      </div>
    );
  }

  if (view.page.error || !view.page.exposure) {
    return <GatewayLoadFailure view={view} />;
  }

  return (
    <SettingsPageFrame
      description="Everything here is off until you turn it on, and turning it off takes effect immediately."
      meta={[
        {
          key: "reach",
          content: view.page.exposure.localOnly ? (
            <span data-testid="gateway-local-only">local only</span>
          ) : (
            <span data-testid="gateway-reachable">
              <span className="font-medium text-muted">
                {view.page.status?.addresses.length ?? 0}
              </span>{" "}
              verified addresses
            </span>
          ),
        },
        {
          key: "devices",
          content: (
            <span>
              <span className="font-medium text-muted">{view.page.activeDevices.length}</span>{" "}
              paired devices
            </span>
          ),
        },
      ]}
      slug="gateway"
      width="wide"
    >
      <GatewayExposureSection
        error={view.exposure.error}
        exposure={view.page.exposure}
        isSubmitting={view.exposure.isSubmitting}
        onSetSurface={view.exposure.setSurface}
      />
      <GatewayProviderSection
        actionError={view.providers.actionError}
        inventoryError={view.providers.inventoryError}
        isLoading={view.providers.isLoading}
        isSubmitting={view.providers.isSubmitting}
        model={view.providers.model}
        onDisable={view.providers.disable}
        onEnable={view.providers.enable}
      />
      <GatewayDeviceList
        devices={view.page.devices}
        error={view.devices.error}
        isBusy={view.devices.isBusy}
        {...(listenerTier === "public" ? {} : { onPair: view.pairing.start })}
        onRename={view.devices.rename}
        onRevoke={view.devices.revoke}
      />
      <GatewayAuditPanel
        error={view.audit.error?.message ?? null}
        hasRun={view.audit.hasRun}
        isRunning={view.audit.isRunning}
        onRun={view.audit.run}
        report={view.audit.report}
      />
      {listenerTier === "public" ? null : (
        <GatewayPairingDialog
          artifact={view.pairing.artifact}
          error={view.pairing.error}
          isMinting={view.pairing.isMinting}
          onMint={view.pairing.mint}
          onOpenChange={view.pairing.setOpen}
          open={view.pairing.open}
          {...(view.pairing.address ? { pairingAddress: view.pairing.address } : {})}
        />
      )}
    </SettingsPageFrame>
  );
}

function GatewayLoadFailure({ view }: { view: GatewaySettingsViewModel }) {
  return (
    <div
      className="flex flex-1 items-center justify-center"
      data-testid="settings-page-gateway-error"
    >
      <div className="flex flex-col items-center gap-2 text-center">
        <AlertCircle className="size-6 text-danger" />
        <p className="text-sm text-subtle">
          {view.page.error?.message ?? "Failed to load gateway status"}
        </p>
        <Button onClick={view.page.refetch} size="sm" type="button" variant="neutral">
          Retry
        </Button>
      </div>
    </div>
  );
}

import {
  GatewayAuditPanel,
  GatewayDeviceList,
  GatewayExposureSection,
  GatewayPairingDialog,
  GatewayProviderSection,
  useGatewaySettingsPage,
  useGatewayAccessTier,
} from "@/systems/gateway";
import { SettingsPageFrame, SettingsPageState, useSettingsTopbar } from "@/systems/settings";

/**
 * The Remote access surface: how CompozyOS is reachable, which devices
 * hold a session, and what the self-audit says about the current posture.
 */
export function GatewaySettingsPage() {
  const view = useGatewaySettingsPage();
  const listenerTier = useGatewayAccessTier();
  useSettingsTopbar("gateway");

  if (view.page.isLoading) {
    return <SettingsPageState slug="gateway" state="loading" />;
  }

  if (view.page.error || !view.page.exposure) {
    return (
      <SettingsPageState
        error={view.page.error}
        onRetry={view.page.refetch}
        slug="gateway"
        state="error"
      />
    );
  }

  return (
    <SettingsPageFrame
      description="Everything here is off until you turn it on, and turning it off takes effect immediately."
      meta={[
        {
          key: "reach",
          content: view.page.exposure.localOnly ? (
            <span data-testid="gateway-local-only">This computer only</span>
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

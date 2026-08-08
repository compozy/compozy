import { useState } from "react";

import type { GatewayExposureRow } from "../lib/gateway-exposure-model";
import type { GatewayDevice, GatewayPairingArtifact } from "../types";
import { useGatewayAudit, type GatewayAuditViewModel } from "./use-gateway-audit";
import {
  useMintGatewayPairing,
  useRenameGatewayDevice,
  useRevokeGatewayDevice,
  useSetGatewaySurface,
} from "./use-gateway-mutations";
import { useGatewayPage, type GatewayPageViewModel } from "./use-gateway-page";
import { useGatewayProviders, type GatewayProvidersViewModel } from "./use-gateway-providers";

export interface GatewayPairingViewModel {
  open: boolean;
  isMinting: boolean;
  error: string | null;
  artifact: GatewayPairingArtifact | null;
  /** Verified private-overlay address a new device should open, when one exists. */
  address?: string;
  setOpen: (open: boolean) => void;
  start: () => void;
  mint: () => void;
}

export interface GatewaySettingsViewModel {
  page: GatewayPageViewModel;
  audit: GatewayAuditViewModel;
  providers: GatewayProvidersViewModel;
  exposure: {
    isSubmitting: boolean;
    error: string | null;
    setSurface: (row: GatewayExposureRow, desired: boolean, consent: boolean) => void;
  };
  devices: {
    isBusy: boolean;
    error: string | null;
    rename: (id: string, name: string) => void;
    revoke: (device: GatewayDevice) => void;
  };
  pairing: GatewayPairingViewModel;
}

/**
 * Everything the gateway settings page needs, as one flat view model.
 *
 * Each action applies immediately — exposure and revocation are decisions the
 * runtime executes at once, so there is no draft to hold or save.
 */
export function useGatewaySettingsPage(): GatewaySettingsViewModel {
  const page = useGatewayPage();
  const audit = useGatewayAudit();
  const providers = useGatewayProviders(page.status);
  const setSurface = useSetGatewaySurface();
  const mintPairing = useMintGatewayPairing();
  const renameDevice = useRenameGatewayDevice();
  const revokeDevice = useRevokeGatewayDevice();
  const [pairingOpen, setPairingOpen] = useState(false);

  // A new device is admitted through the private overlay or on this machine —
  // never through the public address — so the pairing link points there.
  const pairingAddress = page.exposure?.rows
    .find(row => row.id === "private-operator-ui")
    ?.addresses.at(0)?.address;

  return {
    page,
    audit,
    providers,
    exposure: {
      isSubmitting: setSurface.isPending,
      error: errorMessage(setSurface.error),
      setSurface: (row, desired, consent) =>
        setSurface.mutate({
          consent,
          desired: desired ? "enabled" : "disabled",
          expected_generation: row.generation,
          surface: row.surface,
          tier: row.tier,
        }),
    },
    devices: {
      isBusy: renameDevice.isPending || revokeDevice.isPending,
      error: errorMessage(renameDevice.error ?? revokeDevice.error),
      rename: (id, name) => renameDevice.mutate({ id, name }),
      revoke: device => revokeDevice.mutate(device.id),
    },
    pairing: {
      open: pairingOpen,
      isMinting: mintPairing.isPending,
      error: errorMessage(mintPairing.error),
      artifact: mintPairing.data ?? null,
      ...(pairingAddress ? { address: pairingAddress } : {}),
      setOpen: setPairingOpen,
      start: () => {
        setPairingOpen(true);
        mintPairing.mutate();
      },
      mint: () => mintPairing.mutate(),
    },
  };
}

function errorMessage(error: Error | null | undefined): string | null {
  return error?.message ?? null;
}

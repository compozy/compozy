import { useQuery } from "@tanstack/react-query";

import {
  buildGatewayExposureModel,
  type GatewayExposureModel,
} from "../lib/gateway-exposure-model";
import { gatewayDevicesOptions, gatewayStatusOptions } from "../lib/query-options";
import type { GatewayDevice, GatewayStatus } from "../types";

export interface GatewayPageViewModel {
  isLoading: boolean;
  error: Error | null;
  status: GatewayStatus | null;
  exposure: GatewayExposureModel | null;
  devices: readonly GatewayDevice[];
  activeDevices: readonly GatewayDevice[];
  revokedDevices: readonly GatewayDevice[];
  refetch: () => void;
}

/**
 * The page's single read boundary: posture and inventory are two independent
 * daemon reads, flattened here so components receive a view model and never
 * touch the query layer themselves.
 */
export function useGatewayPage(): GatewayPageViewModel {
  const statusQuery = useQuery(gatewayStatusOptions());
  const devicesQuery = useQuery(gatewayDevicesOptions());
  const status = statusQuery.data ?? null;
  const devices = devicesQuery.data ?? [];

  return {
    isLoading: statusQuery.isLoading || devicesQuery.isLoading,
    error: statusQuery.error ?? devicesQuery.error,
    status,
    exposure: status ? buildGatewayExposureModel(status) : null,
    devices,
    activeDevices: devices.filter(device => !device.revoked_at),
    revokedDevices: devices.filter(device => Boolean(device.revoked_at)),
    refetch: () => {
      void statusQuery.refetch();
      void devicesQuery.refetch();
    },
  };
}

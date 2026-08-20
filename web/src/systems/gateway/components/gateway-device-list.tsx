import { Alert, AlertDescription, AlertTitle, Button, Empty } from "@compozy/ui";

import type { GatewayDevice } from "../types";
import { GatewayDeviceRow } from "./gateway-device-row";
import { SettingsGroup } from "@/systems/settings";

export interface GatewayDeviceListProps {
  devices: readonly GatewayDevice[];
  isBusy: boolean;
  error?: string | null;
  onPair?: () => void;
  onRename: (id: string, name: string) => void;
  onRevoke: (device: GatewayDevice) => void;
}

/**
 * The living device inventory. Revocation is immediate and terminal — it also
 * cancels that device's live streams before returning — so it is offered as a
 * direct action with no soft "disable" in between.
 */
export function GatewayDeviceList({
  devices,
  isBusy,
  error,
  onPair,
  onRename,
  onRevoke,
}: GatewayDeviceListProps) {
  return (
    <SettingsGroup
      action={
        onPair ? (
          <Button data-testid="gateway-pair-device" onClick={onPair} size="sm" type="button">
            Pair a device
          </Button>
        ) : undefined
      }
      data-testid="gateway-device-list"
      title="Devices"
    >
      {error ? (
        <div className="px-4 py-3">
          <Alert variant="danger">
            <AlertTitle>That device change was not applied</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        </div>
      ) : null}
      {devices.length === 0 ? (
        <Empty
          className="py-8"
          description={
            onPair
              ? "Pair a device to reach CompozyOS from somewhere other than this machine."
              : "Pair devices from this machine's private address, or from the machine running CompozyOS."
          }
          title="No paired devices yet"
        />
      ) : (
        devices.map(device => (
          <GatewayDeviceRow
            device={device}
            isBusy={isBusy}
            key={device.id}
            onRename={onRename}
            onRevoke={onRevoke}
          />
        ))
      )}
    </SettingsGroup>
  );
}

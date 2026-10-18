import { useState } from "react";
import { Check, Pencil, Smartphone, Terminal, X } from "lucide-react";

import { Button, ConfirmDialog, Input, ListingRow, Pill, Time } from "@compozy/ui";

import { deviceActorLabel, deviceOriginLabel } from "../lib/gateway-copy";
import type { GatewayDevice } from "../types";

export interface GatewayDeviceRowProps {
  device: GatewayDevice;
  isBusy: boolean;
  onRename: (id: string, name: string) => void;
  onRevoke: (device: GatewayDevice) => void;
}

/**
 * One paired device: what it is called, where it was paired from, and when it
 * was last active. Revoked rows stay in the inventory because revocation is
 * terminal and auditable — they simply lose their actions.
 */
export function GatewayDeviceRow({ device, isBusy, onRename, onRevoke }: GatewayDeviceRowProps) {
  const [draft, setDraft] = useState<string | null>(null);
  const [confirmingRevoke, setConfirmingRevoke] = useState(false);
  const isRevoked = Boolean(device.revoked_at);
  const lastActivity = device.last_seen_at ?? device.created_at;

  const commit = () => {
    const next = draft?.trim() ?? "";
    if (next !== "" && next !== device.name) onRename(device.id, next);
    setDraft(null);
  };

  return (
    <ListingRow data-testid={`gateway-device-${device.id}`} interactive={false}>
      <ListingRow.Icon>
        {device.actor_kind === "cli_profile" ? (
          <Terminal className="size-4" />
        ) : (
          <Smartphone className="size-4" />
        )}
      </ListingRow.Icon>
      <ListingRow.Main>
        {draft === null ? (
          <ListingRow.Title>
            <ListingRow.Name>{device.name}</ListingRow.Name>
          </ListingRow.Title>
        ) : (
          <Input
            aria-label={`Rename ${device.name}`}
            autoFocus
            data-testid={`gateway-device-${device.id}-name-input`}
            onChange={event => setDraft(event.target.value)}
            onKeyDown={event => {
              if (event.key === "Enter") commit();
              if (event.key === "Escape") setDraft(null);
            }}
            value={draft}
          />
        )}
        <ListingRow.Meta>
          <span>{deviceActorLabel(device.actor_kind)}</span>
          <span>Paired from {deviceOriginLabel(device.pairing_origin)}</span>
          {isRevoked ? (
            <span>
              Revoked <Time iso={device.revoked_at ?? device.created_at} mode="relative" />
            </span>
          ) : (
            <span>
              Last active <Time iso={lastActivity} mode="relative" />
            </span>
          )}
        </ListingRow.Meta>
      </ListingRow.Main>
      <ListingRow.Trail>
        {isRevoked ? (
          <Pill form="plain" tone="neutral">
            <Pill.Dot />
            Revoked
          </Pill>
        ) : draft === null ? (
          <>
            <Button
              aria-label={`Rename ${device.name}`}
              data-testid={`gateway-device-${device.id}-rename`}
              disabled={isBusy}
              onClick={() => setDraft(device.name)}
              size="icon-sm"
              type="button"
              variant="quiet"
            >
              <Pencil aria-hidden="true" />
            </Button>
            <Button
              data-testid={`gateway-device-${device.id}-revoke`}
              disabled={isBusy}
              onClick={() => setConfirmingRevoke(true)}
              size="sm"
              type="button"
              variant="destructive"
            >
              Revoke
            </Button>
            <ConfirmDialog
              cancelLabel="Keep this device"
              confirmButtonProps={{
                "data-testid": `gateway-device-${device.id}-revoke-confirm`,
              }}
              confirmLabel="Revoke access"
              contentProps={{ "data-testid": `gateway-device-${device.id}-revoke-dialog` }}
              description={`${device.name} loses its session immediately and its live streams are closed. This cannot be undone — the device needs a new pairing code, and it will come back as a new device.`}
              isPending={isBusy}
              onConfirm={() => {
                setConfirmingRevoke(false);
                onRevoke(device);
              }}
              onOpenChange={setConfirmingRevoke}
              open={confirmingRevoke}
              title={`Revoke ${device.name}?`}
              tone="danger"
            />
          </>
        ) : (
          <>
            <Button
              aria-label="Save device name"
              data-testid={`gateway-device-${device.id}-rename-save`}
              disabled={isBusy}
              onClick={commit}
              size="icon-sm"
              type="button"
              variant="quiet"
            >
              <Check aria-hidden="true" />
            </Button>
            <Button
              aria-label="Cancel rename"
              onClick={() => setDraft(null)}
              size="icon-sm"
              type="button"
              variant="quiet"
            >
              <X aria-hidden="true" />
            </Button>
          </>
        )}
      </ListingRow.Trail>
    </ListingRow>
  );
}

import { Switch } from "@compozy/ui";

import { SettingRow } from "@/systems/settings";

import { EXPOSURE_ROW_COPY, REACHABILITY_COPY } from "../lib/gateway-copy";
import type { GatewayExposureRow } from "../lib/gateway-exposure-model";
import { GatewayAddressesList } from "./gateway-addresses-list";
import { GatewayStatusChip } from "./gateway-status-chip";

export interface GatewayExposureRowProps {
  row: GatewayExposureRow;
  disabled: boolean;
  onToggle: (row: GatewayExposureRow, desired: boolean) => void;
}

/**
 * One rung of the exposure ladder: what it does, what it exposes, whether the
 * runtime has actually reached it, and the verified addresses if it has.
 *
 * The status chip reads verified runtime state, so a row the operator asked for
 * but the runtime has not established shows `Establishing` or `Degraded` beside
 * an on switch — intent and reality are never collapsed into one signal.
 */
export function GatewayExposureRowItem({ row, disabled, onToggle }: GatewayExposureRowProps) {
  const copy = EXPOSURE_ROW_COPY[row.id];
  const reachability = REACHABILITY_COPY[row.reachability];
  const showStatus = row.desired || row.served;

  return (
    <SettingRow
      control={
        <Switch
          checked={row.desired}
          data-testid={`gateway-exposure-${row.id}-switch`}
          disabled={disabled}
          onCheckedChange={next => onToggle(row, next)}
        />
      }
      data-testid={`gateway-exposure-${row.id}`}
      description={
        <span className="flex flex-col gap-2">
          <span>{copy.risk}</span>
          {showStatus ? (
            <span className="flex flex-wrap items-center gap-2">
              {/* The detail is rendered visibly beside the chip, so it is not
                  repeated as screen-reader-only text. */}
              <GatewayStatusChip
                data-testid={`gateway-exposure-${row.id}-status`}
                label={reachability.label}
                tone={reachability.tone}
              />
              <span>{reachability.detail}</span>
            </span>
          ) : null}
          {row.cause ? (
            <span className="text-danger" data-testid={`gateway-exposure-${row.id}-cause`}>
              {row.cause}
            </span>
          ) : null}
          <GatewayAddressesList
            addresses={row.addresses}
            data-testid={`gateway-exposure-${row.id}-addresses`}
          />
        </span>
      }
      label={copy.title}
    />
  );
}

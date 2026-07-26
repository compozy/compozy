import { useNavigate } from "@tanstack/react-router";
import { Pencil, Power, RotateCw } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Pill,
  TopbarOverflowIcon,
  type PillTone,
  useTopbarSlot,
} from "@agh/ui";

import { bridgeStatusLabel, bridgeStatusTone } from "../lib/bridge-formatters";
import type { BridgeStatus, BridgeSummary } from "../types";

interface BridgeDetailHeaderProps {
  bridge: BridgeSummary;
  effectiveStatus: BridgeStatus;
  isLifecyclePending: boolean;
  onDisableBridge?: () => void;
  onEnableBridge?: () => void;
  onOpenEdit?: () => void;
  onRestartBridge?: () => void;
}

function statusToPillTone(status: BridgeStatus): PillTone {
  return status === "disabled" ? "danger" : bridgeStatusTone(status);
}

function BridgeDetailActions({
  enabled,
  isLifecyclePending,
  onEnableBridge,
  onOpenEdit,
}: {
  enabled: boolean;
  isLifecyclePending: boolean;
  onEnableBridge?: () => void;
  onOpenEdit?: () => void;
}) {
  return (
    <div className="flex items-center" data-testid="bridge-detail-actions">
      {enabled ? (
        <Button
          data-testid="edit-bridge-btn"
          disabled={isLifecyclePending}
          onClick={onOpenEdit}
          size="sm"
          type="button"
          variant="neutral"
        >
          <Pencil className="size-3" />
          Edit
        </Button>
      ) : (
        <Button
          data-testid="enable-bridge-btn"
          disabled={isLifecyclePending}
          onClick={onEnableBridge}
          size="sm"
          type="button"
        >
          <Power className="size-3" />
          Enable
        </Button>
      )}
    </div>
  );
}

function BridgeDetailOverflow({
  enabled,
  isLifecyclePending,
  onDisableBridge,
  onOpenEdit,
  onRestartBridge,
}: {
  enabled: boolean;
  isLifecyclePending: boolean;
  onDisableBridge?: () => void;
  onOpenEdit?: () => void;
  onRestartBridge?: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More actions"
        data-testid="bridge-detail-overflow"
        render={<Button type="button" variant="ghost" size="icon-sm" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="bridge-detail-overflow-menu">
        {!enabled ? (
          <DropdownMenuItem
            data-testid="edit-bridge-btn"
            disabled={isLifecyclePending}
            onClick={onOpenEdit}
          >
            <Pencil className="size-3" />
            Edit
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem
          data-testid="restart-bridge-btn"
          disabled={isLifecyclePending}
          onClick={onRestartBridge}
        >
          <RotateCw className="size-3" />
          Restart
        </DropdownMenuItem>
        {enabled ? (
          <DropdownMenuItem
            data-testid="disable-bridge-btn"
            disabled={isLifecyclePending}
            onClick={onDisableBridge}
          >
            <Power className="size-3" />
            Disable
          </DropdownMenuItem>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export function BridgeDetailHeader({
  bridge,
  effectiveStatus,
  isLifecyclePending,
  onDisableBridge,
  onEnableBridge,
  onOpenEdit,
  onRestartBridge,
}: BridgeDetailHeaderProps) {
  const statusTone = statusToPillTone(effectiveStatus);
  const pills = (
    <>
      <Pill mono tone={bridge.scope === "workspace" ? "info" : "neutral"}>
        {bridge.scope}
      </Pill>
    </>
  );

  const navigate = useNavigate();
  const backToBridges = () => {
    void navigate({ to: "/bridges" });
  };

  useTopbarSlot({
    onBack: backToBridges,
    crumbs: [{ id: "bridges", label: "Bridges", onSelect: backToBridges }],
    crumb: bridge.display_name,
    status: (
      <Pill mono tone={statusTone}>
        <Pill.Dot pulse={effectiveStatus === "starting"} tone={statusTone} />
        {bridgeStatusLabel(effectiveStatus)}
      </Pill>
    ),
    actions: (
      <BridgeDetailActions
        enabled={bridge.enabled}
        isLifecyclePending={isLifecyclePending}
        onEnableBridge={onEnableBridge}
        onOpenEdit={onOpenEdit}
      />
    ),
    overflow: (
      <BridgeDetailOverflow
        enabled={bridge.enabled}
        isLifecyclePending={isLifecyclePending}
        onDisableBridge={onDisableBridge}
        onOpenEdit={onOpenEdit}
        onRestartBridge={onRestartBridge}
      />
    ),
  });

  return (
    <div className="flex flex-col gap-2 pt-4" data-testid="bridge-detail-header">
      <div className="flex flex-wrap items-center gap-1.5">{pills}</div>
      <span className="text-small-body text-subtle" data-testid="bridge-detail-meta-platform">
        {bridge.platform} / {bridge.extension_name}
      </span>
    </div>
  );
}

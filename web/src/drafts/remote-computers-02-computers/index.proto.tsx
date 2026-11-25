import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  ACCOUNT_PRESETS,
  ACCOUNT_UNADDED_COMPUTERS,
  ACCOUNT_UNADDED_FREE,
  COMPUTERS_DOWNGRADED,
  COMPUTERS_FREE,
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
  UPGRADE_NOTICE_FIXTURE,
  makeScaleComputers,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { ComputersSettingsPage, type ComputersPageProps } from "./computers-page";

export const meta = definePrototype({
  title: "Settings → Computers",
  spec: "remote-computers",
  surface: "U1 U10",
  route: "/settings/computers",
  states: [
    { id: "pro-populated", label: "Pro · Fleet with every row status" },
    { id: "free-slot-used", label: "Free · 1 of 1 remote computer slot used" },
    { id: "downgraded-needs-pro", label: "Downgraded · 1 active, 2 need Pro" },
    { id: "empty", label: "Empty · No other computers yet" },
    { id: "cloud-unreachable", label: "Cloud unreachable · Local & LAN unaffected" },
    { id: "upgrade-notice", label: "Upgrade notice · Gateway removed & webhook review" },
    { id: "hosted-route-degraded", label: "Hosted route · Port 7844 blocked & Devices open" },
    { id: "hosted-route-starting", label: "Hosted route · Provisioning in progress" },
    { id: "load-balancing-open", label: "Load balancing · Weights & live capacity" },
    { id: "stop-local-agents-dialog", label: "Stop local agents · Confirm with running sessions" },
    { id: "remove-dialog", label: "Remove dialog · Free slot & SSH uninstall options" },
    { id: "revoke-device-safeguard", label: "Revoke device · Local approval safeguard" },
    { id: "scale-50", label: "Scale · 50 computers in flat filterable list" },
  ],
});

const SCALE_50_COMPUTERS = makeScaleComputers(50);

function propsFor(state: string): ComputersPageProps {
  const proBase: ComputersPageProps = {
    account: ACCOUNT_PRESETS.pro,
    local: LOCAL_COMPUTER,
    computers: COMPUTERS_PRO,
    unadded: ACCOUNT_UNADDED_COMPUTERS,
  };

  switch (state) {
    case "free-slot-used":
      return {
        account: ACCOUNT_PRESETS.free,
        local: {
          ...LOCAL_COMPUTER,
          hostedRouteEnabled: false,
          hostedRouteState: "off",
        },
        computers: COMPUTERS_FREE,
        unadded: ACCOUNT_UNADDED_FREE,
      };
    case "downgraded-needs-pro":
      return {
        account: ACCOUNT_PRESETS.downgraded,
        local: {
          ...LOCAL_COMPUTER,
          hostedRouteEnabled: false,
          hostedRouteState: "off",
        },
        computers: COMPUTERS_DOWNGRADED,
        unadded: [],
      };
    case "empty":
      return {
        account: {
          ...ACCOUNT_PRESETS.free,
          remoteComputersUsed: 0,
        },
        local: {
          ...LOCAL_COMPUTER,
          hostedRouteEnabled: false,
          hostedRouteState: "off",
        },
        computers: [],
        unadded: [],
      };
    case "cloud-unreachable":
      return {
        ...proBase,
        account: ACCOUNT_PRESETS.cloud_unreachable,
      };
    case "upgrade-notice":
      return {
        ...proBase,
        upgradeNotice: UPGRADE_NOTICE_FIXTURE,
      };
    case "hosted-route-degraded":
      return {
        ...proBase,
        local: {
          ...LOCAL_COMPUTER,
          hostedRouteEnabled: true,
          hostedRouteState: "unreachable",
          hostedRouteError: "Hosted route can't reach Cloudflare (port 7844 blocked)",
        },
        devicesOpen: true,
      };
    case "hosted-route-starting":
      return {
        ...proBase,
        local: {
          ...LOCAL_COMPUTER,
          hostedRouteEnabled: true,
          hostedRouteState: "starting",
        },
      };
    case "load-balancing-open":
      return {
        ...proBase,
        loadBalancingEnabled: true,
        loadBalancingOpen: true,
      };
    case "stop-local-agents-dialog":
      return {
        ...proBase,
        initialDisableLocalOpen: true,
      };
    case "remove-dialog":
      return {
        account: ACCOUNT_PRESETS.free,
        local: {
          ...LOCAL_COMPUTER,
          hostedRouteEnabled: false,
          hostedRouteState: "off",
        },
        computers: COMPUTERS_FREE,
        unadded: [],
        initialRemoveTargetId: "cmp_7f3k2q9x",
      };
    case "revoke-device-safeguard":
      return {
        ...proBase,
        local: {
          ...LOCAL_COMPUTER,
          requireDeviceApproval: false,
        },
        devicesOpen: true,
        initialRevokeDeviceId: "dev_5k1p9z",
      };
    case "scale-50":
      return {
        ...proBase,
        account: {
          ...ACCOUNT_PRESETS.pro,
          remoteComputersUsed: 50,
        },
        computers: SCALE_50_COMPUTERS,
        unadded: [],
      };
    default:
      return proBase;
  }
}

export default function RemoteComputersSettingsDraft({ state }: PrototypeProps) {
  return <ComputersSettingsPage key={state} {...propsFor(state)} />;
}

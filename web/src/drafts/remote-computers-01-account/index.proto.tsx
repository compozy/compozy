import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { ACCOUNT_DEVICES, ACCOUNT_PRESETS, PENDING_APPROVALS } from "../remote-computers-shared";
import {
  ACCOUNT_APPROVALS_EXTENDED,
  ACCOUNT_NON_ADMIN_DEVICES,
  ACCOUNT_PRO_HOSTED_CAP,
} from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";
import { AccountSettingsPage, type AccountPageProps } from "./account-page";

export const meta = definePrototype({
  title: "Settings → Account",
  spec: "remote-computers",
  surface: "U3 U9",
  route: "/settings/account",
  states: [
    { id: "pro-populated", label: "Pro · Plan, usage, pending approvals & devices" },
    { id: "free", label: "Free · 1 of 1 remote computer slot used" },
    { id: "downgraded", label: "Downgraded · Pro ended, 1 active & 2 need Pro" },
    { id: "signed-out", label: "Signed out · Local work unaffected" },
    { id: "signing-in", label: "Signing in · Waiting for browser & device code" },
    { id: "pending-approvals", label: "Approvals · Pending, delivering & recent decisions" },
    { id: "non-admin-device", label: "Standard device · Admin controls guarded" },
    { id: "payment-issue", label: "Payment issue · Past due card banner" },
    { id: "ending-subscription", label: "Ending · Pro cancels on date & hosted cap" },
    { id: "cloud-unreachable", label: "Cloud unreachable · Local work unaffected" },
    { id: "upgrade-sheet", label: "Upgrade sheet · Annual vs monthly over Account" },
    { id: "upgrade-waiting", label: "Upgrade sheet · Waiting for checkout" },
    { id: "upgrade-done", label: "Upgrade sheet · You're on Pro" },
    { id: "revoke-device-dialog", label: "Revoke device · Confirm with approval safeguard" },
  ],
});

function propsFor(state: string): AccountPageProps {
  const proBase: AccountPageProps = {
    account: ACCOUNT_PRESETS.pro,
    devices: ACCOUNT_DEVICES,
    pendingApprovals: PENDING_APPROVALS,
  };

  switch (state) {
    case "free":
      return {
        account: ACCOUNT_PRESETS.free,
        devices: ACCOUNT_DEVICES.slice(0, 2),
        pendingApprovals: [],
      };
    case "downgraded":
      return {
        account: ACCOUNT_PRESETS.downgraded,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        downgradedNeedsProCount: 2,
      };
    case "signed-out":
      return {
        account: ACCOUNT_PRESETS.signed_out,
        devices: [],
        pendingApprovals: [],
      };
    case "signing-in":
      return {
        account: ACCOUNT_PRESETS.signed_out,
        signingIn: true,
        devices: [],
        pendingApprovals: [],
      };
    case "pending-approvals":
      return {
        account: ACCOUNT_PRESETS.pro,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: ACCOUNT_APPROVALS_EXTENDED,
      };
    case "non-admin-device":
      return {
        account: ACCOUNT_PRESETS.pro,
        devices: ACCOUNT_NON_ADMIN_DEVICES,
        pendingApprovals: PENDING_APPROVALS.map(item => ({ ...item, canDecide: false })),
      };
    case "payment-issue":
      return {
        account: ACCOUNT_PRESETS.past_due,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
      };
    case "ending-subscription":
      return {
        account: {
          ...ACCOUNT_PRO_HOSTED_CAP,
          cancelAtPeriodEnd: true,
          renewsAt: null,
          endsAt: ACCOUNT_PRESETS.ending.endsAt,
        },
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
      };
    case "cloud-unreachable":
      return {
        account: ACCOUNT_PRESETS.cloud_unreachable,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
      };
    case "upgrade-sheet":
      return {
        account: ACCOUNT_PRESETS.free,
        devices: ACCOUNT_DEVICES.slice(0, 2),
        pendingApprovals: [],
        initialUpgradeOpen: true,
        initialUpgradePhase: "select",
      };
    case "upgrade-waiting":
      return {
        account: ACCOUNT_PRESETS.free,
        devices: ACCOUNT_DEVICES.slice(0, 2),
        pendingApprovals: [],
        initialUpgradeOpen: true,
        initialUpgradePhase: "waiting",
      };
    case "upgrade-done":
      return {
        account: ACCOUNT_PRESETS.pro,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        initialUpgradeOpen: true,
        initialUpgradePhase: "done",
      };
    case "revoke-device-dialog":
      return {
        account: ACCOUNT_PRESETS.pro,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        localRequireDeviceApproval: false,
        initialRevokeDeviceId: "dev_5k1p9z",
      };
    default:
      return proBase;
  }
}

export default function RemoteComputersAccountDraft({ state }: PrototypeProps) {
  return <AccountSettingsPage key={state} {...propsFor(state)} />;
}

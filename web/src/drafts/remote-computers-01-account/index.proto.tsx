import { definePrototype, useSearch, type PrototypeProps } from "@compozy/proto";

import type { UpgradeSheetPhase } from "@/systems/account/components/account-upgrade-sheet";
import {
  ACCOUNT_APPROVALS_EXTENDED,
  ACCOUNT_NON_ADMIN_DEVICES,
  ACCOUNT_TOKENS,
  CREATED_ACCOUNT_TOKEN_FIXTURE,
} from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";

import {
  ACCOUNT_DEVICES,
  ACCOUNT_PRESETS,
  PENDING_APPROVALS,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { AccountSettingsPage, type AccountPageProps } from "./account-page";

export const meta = definePrototype({
  title: "Settings → Account",
  spec: "remote-computers",
  surface: "U3 U9",
  route: "/settings/account",
  states: [
    { id: "pro-populated", label: "Pro · Profile, plan, approvals & devices" },
    { id: "free", label: "Free · 1 of 1 remote computer slot used" },
    { id: "downgraded", label: "Downgraded · Pro ended, 1 active & 2 need Pro" },
    { id: "pending-approvals", label: "Approvals · Pending, delivering & recent decisions" },
    { id: "non-admin-device", label: "Standard device · Admin controls & delete guarded" },
    { id: "payment-issue", label: "Payment issue · Past due card banner" },
    { id: "cloud-unreachable", label: "Cloud unreachable · Cached profile & local unaffected" },
    {
      id: "link-only-installation",
      label: "Link-only · Linked for remote access & other-account notice",
    },
    { id: "email-and-tokens", label: "Email, tokens & delete · Pro account lower blocks" },
    { id: "upgrade-sheet", label: "Upgrade sheet · Annual vs monthly (click for waiting)" },
    { id: "token-created", label: "Account token created · Copy secret once" },
    { id: "revoke-device-dialog", label: "Revoke device · Confirm with local approval safeguard" },
    { id: "sign-out-dialog", label: "Sign out · Also unlinks this computer" },
    { id: "delete-account-dialog", label: "Delete account · Impact summary & email confirmation" },
  ],
});

function resolveUpgradePhase(raw: string | null): UpgradeSheetPhase {
  if (raw === "waiting" || raw === "waiting_timeout" || raw === "done") return raw;
  return "select";
}

function propsFor(state: string, searchPhase: string | null): AccountPageProps {
  const proBase: AccountPageProps = {
    account: ACCOUNT_PRESETS.pro,
    devices: ACCOUNT_DEVICES,
    pendingApprovals: PENDING_APPROVALS,
    tokens: ACCOUNT_TOKENS,
  };

  switch (state) {
    case "free":
      return {
        account: ACCOUNT_PRESETS.free,
        devices: ACCOUNT_DEVICES.slice(0, 2),
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS.slice(0, 1),
      };
    case "downgraded":
      return {
        account: ACCOUNT_PRESETS.downgraded,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS.slice(0, 1),
        downgradedNeedsProCount: 2,
      };
    case "pending-approvals":
      return {
        account: ACCOUNT_PRESETS.pro,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: ACCOUNT_APPROVALS_EXTENDED,
        tokens: ACCOUNT_TOKENS,
      };
    case "non-admin-device":
      return {
        account: ACCOUNT_PRESETS.pro,
        devices: ACCOUNT_NON_ADMIN_DEVICES,
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS.slice(0, 1),
        scrollToLower: true,
      };
    case "payment-issue":
      return {
        account: ACCOUNT_PRESETS.past_due,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS,
      };
    case "cloud-unreachable":
      return {
        account: ACCOUNT_PRESETS.cloud_unreachable,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS,
      };
    case "link-only-installation":
      return {
        account: ACCOUNT_PRESETS.link_only_installation,
        devices: [],
        pendingApprovals: [],
        tokens: [],
        initialLinkMismatch: true,
      };
    case "email-and-tokens":
      return {
        account: ACCOUNT_PRESETS.pro,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS,
        scrollToLower: true,
      };
    case "upgrade-sheet":
      return {
        account: ACCOUNT_PRESETS.free,
        devices: ACCOUNT_DEVICES.slice(0, 2),
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS.slice(0, 1),
        initialUpgradeOpen: true,
        initialUpgradePhase: resolveUpgradePhase(searchPhase),
      };
    case "token-created":
      return {
        account: ACCOUNT_PRESETS.pro,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS,
        scrollToLower: true,
        initialCreatedToken: CREATED_ACCOUNT_TOKEN_FIXTURE,
      };
    case "revoke-device-dialog":
      return {
        account: ACCOUNT_PRESETS.pro,
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS,
        localRemoteEnabled: true,
        localRequireDeviceApproval: false,
        initialRevokeDeviceId: "dev_5k1p9z",
      };
    case "sign-out-dialog":
      return {
        account: { ...ACCOUNT_PRESETS.pro, installationLinked: true },
        devices: ACCOUNT_DEVICES,
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS,
        initialSignOutOpen: true,
      };
    case "delete-account-dialog":
      return {
        account: ACCOUNT_PRESETS.free,
        devices: ACCOUNT_DEVICES.slice(0, 2),
        pendingApprovals: [],
        tokens: ACCOUNT_TOKENS.slice(0, 1),
        scrollToLower: true,
        initialDeleteOpen: true,
      };
    default:
      return proBase;
  }
}

export default function RemoteComputersAccountDraft({ state }: PrototypeProps) {
  const search = useSearch();
  const phaseParam = new URLSearchParams(search).get("phase");
  return (
    <AccountSettingsPage key={`${state}-${phaseParam ?? ""}`} {...propsFor(state, phaseParam)} />
  );
}

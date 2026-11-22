import { useState } from "react";
import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { toast } from "@compozy/ui";

import {
  ComputerDetailsPanel,
  type ComputerDetailsPanelProps,
} from "@/systems/computers/components/computer-details-panel";
import {
  ACCOUNT_PRESETS,
  ACCOUNT_UNADDED_COMPUTERS,
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  DETAILS_DEVBOX,
  DETAILS_GPU_VM,
  DETAILS_INCOMPATIBLE_MANUAL,
  DETAILS_LONG_VOLUME,
  DETAILS_MINI_BUILD,
  DETAILS_SIGNIN_APPROVAL,
  DETAILS_WORKSTATION,
} from "@/systems/computers/mocks/remote-computers-fixtures-lane-b";

import { ComputersSettingsPage } from "../remote-computers-02-computers/computers-page";

export const meta = definePrototype({
  title: "Computer details",
  spec: "remote-computers",
  surface: "U7",
  route: "/settings/computers",
  states: [
    {
      id: "connected-overview",
      label: "Connected · Capacity, connection check & About",
    },
    {
      id: "needs-attention-diagnosis",
      label: "Needs attention · Linger off with fix command",
    },
    {
      id: "offline-diagnosis",
      label: "Offline · Last seen, 4-hop diagnosis & Retry",
    },
    {
      id: "diagnosis-signin-approval",
      label: "Needs attention · Sign-in hop waiting for approval",
    },
    {
      id: "routes-mismatch",
      label: "Routes · Priority order, In use & different computer warning",
    },
    {
      id: "permissions-readonly",
      label: "Permissions · Effective scopes for this device",
    },
    {
      id: "permissions-editing",
      label: "Permissions · Editing scopes for this device",
    },
    {
      id: "providers-sign-in",
      label: "Providers · Sign in on devbox (remote terminal)",
    },
    {
      id: "incompatible-ssh-update",
      label: "Incompatible · SSH Update steps running",
    },
    {
      id: "incompatible-manual",
      label: "Incompatible · Manual compozy update command",
    },
    {
      id: "long-names-volume",
      label: "Long names & volume · 6 routes and long hostname",
    },
  ],
});

type PanelConfig = Omit<ComputerDetailsPanelProps, "open" | "onOpenChange">;

function configFor(state: string): PanelConfig {
  switch (state) {
    case "needs-attention-diagnosis":
      return {
        computer: DETAILS_MINI_BUILD,
        initialTab: "status",
      };
    case "offline-diagnosis":
      return {
        computer: DETAILS_WORKSTATION,
        initialTab: "status",
      };
    case "diagnosis-signin-approval":
      return {
        computer: DETAILS_SIGNIN_APPROVAL,
        initialTab: "status",
      };
    case "routes-mismatch":
      return {
        computer: DETAILS_WORKSTATION,
        initialTab: "routes",
      };
    case "permissions-readonly":
      return {
        computer: DETAILS_DEVBOX,
        initialTab: "permissions",
        editablePermissions: false,
      };
    case "permissions-editing":
      return {
        computer: DETAILS_DEVBOX,
        initialTab: "permissions",
        editablePermissions: true,
      };
    case "providers-sign-in":
      return {
        computer: DETAILS_DEVBOX,
        initialTab: "providers",
        initialSignInProviderId: "codex",
      };
    case "incompatible-ssh-update":
      return {
        computer: DETAILS_GPU_VM,
        initialTab: "update",
        initialUpdateState: "running",
      };
    case "incompatible-manual":
      return {
        computer: DETAILS_INCOMPATIBLE_MANUAL,
        initialTab: "update",
      };
    case "long-names-volume":
      return {
        computer: DETAILS_LONG_VOLUME,
        initialTab: "routes",
      };
    default:
      return {
        computer: DETAILS_DEVBOX,
        initialTab: "status",
      };
  }
}

export default function ComputerDetailsDraft({ state }: PrototypeProps) {
  const [open, setOpen] = useState(true);
  const config = configFor(state);
  const backdropComputers =
    state === "long-names-volume" ? [DETAILS_LONG_VOLUME, ...COMPUTERS_PRO] : COMPUTERS_PRO;

  return (
    <div className="relative h-dvh w-full overflow-hidden">
      <ComputersSettingsPage
        account={ACCOUNT_PRESETS.pro}
        computers={backdropComputers}
        key={`backdrop-${state}`}
        local={LOCAL_COMPUTER}
        selectedComputerId={open ? config.computer.id : null}
        unadded={ACCOUNT_UNADDED_COMPUTERS}
      />
      <ComputerDetailsPanel
        key={`panel-${state}`}
        {...config}
        onOpenChange={next => {
          setOpen(next);
          if (!next) toast("Details panel closed — switch state to reopen.");
        }}
        open={open}
      />
    </div>
  );
}

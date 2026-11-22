import { useState } from "react";
import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { toast } from "@compozy/ui";

import {
  AddComputerDialog,
  type AddComputerDialogProps,
} from "@/systems/computers/components/add-computer-dialog";
import {
  ACCOUNT_PRESETS,
  ACCOUNT_UNADDED_COMPUTERS,
  COMPUTERS_FREE,
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
  OWNER_EMAIL,
  SSH_HOST_SUGGESTIONS,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import { ComputersSettingsPage } from "../remote-computers-02-computers/computers-page";

export const meta = definePrototype({
  title: "Add computer",
  spec: "remote-computers",
  surface: "U2",
  route: "/settings/computers",
  states: [
    { id: "ssh-picker", label: "SSH · Pick from ~/.ssh/config or enter a host" },
    { id: "ssh-running", label: "SSH · Steps running (Installing Compozy 0.8.0)" },
    { id: "ssh-trust-host", label: "SSH · Trust this host? (SHA256 fingerprint)" },
    { id: "ssh-host-key-changed", label: "SSH · Host key changed (blocked)" },
    { id: "ssh-password", label: "SSH · Password or one-time code prompt" },
    { id: "ssh-agent-secret", label: "SSH · Agent-started setup waiting for password" },
    { id: "ssh-plan-limit", label: "Plan limit · Free 1-computer cap before touching host" },
    { id: "ssh-failed", label: "Failed · Download error with last line and Retry" },
    { id: "ssh-cancel-confirm", label: "Cancel confirmation · Stop setup and free slot" },
    { id: "ssh-unsupported", label: "Unsupported platform · Link with command guidance" },
    { id: "account-path", label: "On your account · Add already-linked computers" },
    { id: "command-path", label: "Link with a command · Copyable command and watcher" },
    {
      id: "command-path-detected",
      label: "Link with a command · Computer linked and ready to add",
    },
    { id: "done", label: "Done · Connected via SSH with Linux linger note" },
  ],
});

interface StateConfig {
  isFreePlan?: boolean;
  dialog: Omit<AddComputerDialogProps, "open" | "onClose" | "onUpgrade" | "onOpenComputer">;
}

function configFor(state: string): StateConfig {
  const baseDialog: StateConfig["dialog"] = {
    accountEmail: OWNER_EMAIL,
    isFreePlan: false,
    sshSuggestions: SSH_HOST_SUGGESTIONS,
    unaddedComputers: ACCOUNT_UNADDED_COMPUTERS,
    initialMethod: "ssh",
    initialSshStage: "picker",
  };

  switch (state) {
    case "ssh-running":
      return {
        dialog: {
          ...baseDialog,
          initialSshStage: "running",
        },
      };
    case "ssh-trust-host":
      return {
        dialog: {
          ...baseDialog,
          initialSshStage: "trust_host",
        },
      };
    case "ssh-host-key-changed":
      return {
        dialog: {
          ...baseDialog,
          initialSshStage: "host_key_changed",
        },
      };
    case "ssh-password":
      return {
        dialog: {
          ...baseDialog,
          initialSshStage: "password",
        },
      };
    case "ssh-agent-secret":
      return {
        dialog: {
          ...baseDialog,
          initialSshStage: "agent_secret",
        },
      };
    case "ssh-plan-limit":
      return {
        isFreePlan: true,
        dialog: {
          ...baseDialog,
          isFreePlan: true,
          initialSshStage: "plan_limit",
        },
      };
    case "ssh-failed":
      return {
        dialog: {
          ...baseDialog,
          initialSshStage: "failed",
        },
      };
    case "ssh-cancel-confirm":
      return {
        dialog: {
          ...baseDialog,
          initialSshStage: "cancel_confirm",
        },
      };
    case "ssh-unsupported":
      return {
        dialog: {
          ...baseDialog,
          initialSshStage: "unsupported",
          initialHostDraft: {
            host: "win-vm",
            user: "admin",
            port: "22",
            name: "win-vm",
          },
        },
      };
    case "account-path":
      return {
        dialog: {
          ...baseDialog,
          initialMethod: "account",
        },
      };
    case "command-path":
      return {
        dialog: {
          ...baseDialog,
          initialMethod: "command",
        },
      };
    case "command-path-detected":
      return {
        dialog: {
          ...baseDialog,
          initialMethod: "command",
          initialCommandDetected: true,
        },
      };
    case "done":
      return {
        dialog: {
          ...baseDialog,
          initialSshStage: "done",
        },
      };
    default:
      return {
        dialog: baseDialog,
      };
  }
}

export default function AddComputerDraft({ state }: PrototypeProps) {
  const [open, setOpen] = useState(true);
  const config = configFor(state);

  return (
    <>
      <ComputersSettingsPage
        account={config.isFreePlan ? ACCOUNT_PRESETS.free : ACCOUNT_PRESETS.pro}
        computers={config.isFreePlan ? COMPUTERS_FREE : COMPUTERS_PRO}
        key={`backdrop-${state}`}
        local={
          config.isFreePlan
            ? { ...LOCAL_COMPUTER, hostedRouteEnabled: false, hostedRouteState: "off" }
            : LOCAL_COMPUTER
        }
        unadded={config.isFreePlan ? [] : ACCOUNT_UNADDED_COMPUTERS}
      />
      <AddComputerDialog
        key={`dialog-${state}`}
        {...config.dialog}
        onClose={() => {
          setOpen(false);
          toast("Dialog closed — switch state to reopen.");
        }}
        onOpenComputer={name => {
          setOpen(false);
          toast.success(`Opened ${name} details.`);
        }}
        onUpgrade={() => toast("Opens Upgrade sheet (Screen 01 · U9)")}
        open={open}
      />
    </>
  );
}

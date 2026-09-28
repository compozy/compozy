import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";
import { useState } from "react";

import type { VaultSecret } from "@/systems/vault/types";

import { VaultSecretSheet } from "../vault-secret-sheet";

const secret: VaultSecret = {
  ref: "vault:automation/nightly-deploy/webhook_secret",
  namespace: "automation",
  kind: "webhook_secret",
  present: true,
  created_at: "2026-05-12T10:00:00Z",
  updated_at: "2026-07-15T10:00:00Z",
};

function SheetHarness() {
  const [replaceValue, setReplaceValue] = useState("");
  return (
    <div className="min-h-[640px] bg-canvas">
      <VaultSecretSheet
        deleteIsDisabled={false}
        onOpenChange={fn()}
        onReplace={fn()}
        onReplaceValueChange={setReplaceValue}
        onRequestDelete={fn()}
        open
        replaceError={null}
        replaceIsPending={false}
        replaceIsValid={replaceValue.trim() !== ""}
        replaceValue={replaceValue}
        secret={secret}
      />
    </div>
  );
}

const meta: Meta<typeof SheetHarness> = {
  title: "systems/vault/components/VaultSecretSheet",
  component: SheetHarness,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Right-side inspect sheet for a vault secret — friendly title, copyable ref, updated/created facts, masked value with a Saved/Missing word, replace, and delete.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = {
  args: {},
};

import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { Pill } from "../pill";
import { SecretField, type SecretFieldMode } from "../secret-field";

const meta: Meta<typeof SecretField> = {
  title: "components/custom/SecretField",
  component: SecretField,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Write-only secret control. Plaintext enters once and is never read back: create renders a single write input, edit renders presence plus an explicit rotation, and cancelling restores the existing binding untouched.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const BADGES = (
  <>
    <Pill mono size="xs">
      secret
    </Pill>
    <Pill mono size="xs" tone="warning">
      required
    </Pill>
  </>
);

function Frame({ children }: { children: React.ReactNode }) {
  return <div className="w-(--width-modal-sm) max-w-full p-6">{children}</div>;
}

export const Absent: Story = {
  args: { id: "api-key", label: "API key", onValueChange: () => {}, value: "" },
  render: () => {
    const [value, setValue] = useState("");
    return (
      <Frame>
        <SecretField
          badges={BADGES}
          description="Stored write-only. AGH returns presence, never the value."
          id="api-key"
          label="API key"
          onValueChange={setValue}
          required
          value={value}
        />
      </Frame>
    );
  },
};

export const PresentAndRotating: Story = {
  args: { id: "api-key", label: "API key", onValueChange: () => {}, value: "" },
  parameters: {
    docs: { description: { story: "Replace opens the write input; Cancel restores presence." } },
  },
  render: () => {
    const [value, setValue] = useState("");
    const [editing, setEditing] = useState(false);
    return (
      <Frame>
        <SecretField
          badges={BADGES}
          editing={editing}
          id="api-key"
          label="API key"
          onEditingChange={setEditing}
          onValueChange={setValue}
          present
          presenceLabel="vault:mcp/github"
          value={value}
        />
      </Frame>
    );
  },
};

export const Editing: Story = {
  args: { id: "api-key", label: "API key", onValueChange: () => {}, value: "" },
  parameters: {
    docs: {
      description: {
        story:
          "Rotation in progress. The write input never carries the stored value, and Cancel restores presence with the existing binding untouched.",
      },
    },
  },
  render: () => (
    <Frame>
      <SecretField
        badges={BADGES}
        editing
        id="api-key"
        label="API key"
        onEditingChange={() => {}}
        onValueChange={() => {}}
        present
        presenceLabel="vault:mcp/github"
        value=""
      />
    </Frame>
  ),
};

export const Invalid: Story = {
  args: { id: "api-key", label: "API key", onValueChange: () => {}, value: "" },
  render: () => (
    <Frame>
      <SecretField
        badges={BADGES}
        error="Enter a value or bind an existing reference."
        id="api-key"
        label="API key"
        onValueChange={() => {}}
        required
        value=""
      />
    </Frame>
  ),
};

export const Saving: Story = {
  args: { id: "api-key", label: "API key", onValueChange: () => {}, value: "" },
  render: () => (
    <Frame>
      <SecretField id="api-key" label="API key" onValueChange={() => {}} saving value="pending" />
    </Frame>
  ),
};

export const Rotated: Story = {
  args: { id: "api-key", label: "API key", onValueChange: () => {}, value: "" },
  render: () => (
    <Frame>
      <SecretField
        id="api-key"
        label="API key"
        onValueChange={() => {}}
        present
        presenceLabel="vault:mcp/github"
        rotated
        value=""
      />
    </Frame>
  ),
};

export const BoundToStoredReference: Story = {
  args: { id: "api-key", label: "API key", onValueChange: () => {}, value: "" },
  parameters: {
    docs: {
      description: {
        story:
          "Fields backed by the secret store can bind an existing reference instead of a typed value. A missing reference cannot be selected.",
      },
    },
  },
  render: () => {
    const [mode, setMode] = useState<SecretFieldMode>("source");
    const [selectedRef, setSelectedRef] = useState("vault:mcp/github");
    const [createOpen, setCreateOpen] = useState(false);
    const [createValue, setCreateValue] = useState("");
    return (
      <Frame>
        <SecretField
          badges={BADGES}
          binding={{
            create: {
              onOpenChange: setCreateOpen,
              onSubmit: () => setCreateOpen(false),
              onValueChange: setCreateValue,
              open: createOpen,
              ref: "vault:mcp/api-key",
              value: createValue,
            },
            namespaceLabel: "namespace=mcp",
            onSelectRef: setSelectedRef,
            selectedRef,
            sources: [
              { present: true, ref: "vault:mcp/github" },
              { present: false, ref: "vault:mcp/legacy" },
            ],
          }}
          id="api-key"
          label="API key"
          mode={mode}
          onModeChange={setMode}
          onValueChange={() => {}}
          sourceModeLabel="Use Vault"
          value=""
        />
      </Frame>
    );
  },
};

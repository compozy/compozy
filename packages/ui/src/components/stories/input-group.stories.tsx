import type { Meta, StoryObj } from "@storybook/react-vite";
import { CommandIcon, MailIcon, SearchIcon } from "lucide-react";

import { Field, FieldDescription, FieldError, FieldLabel } from "../field";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  InputGroupText,
  InputGroupTextarea,
} from "../input-group";
import { Kbd } from "../kbd";
import { Label } from "../label";

const meta: Meta<typeof InputGroup> = {
  title: "components/ui/InputGroup",
  component: InputGroup,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Composed text input with inline and block addons. Compose Input, Button, and Textarea slots to build search, unit, and multi-line controls.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const WithLabelAndHelper: Story = {
  args: {},
  render: () => (
    <div className="w-88">
      <Field>
        <FieldLabel htmlFor="input-group-email">Email</FieldLabel>
        <InputGroup>
          <InputGroupAddon>
            <MailIcon />
          </InputGroupAddon>
          <InputGroupInput id="input-group-email" type="email" placeholder="you@example.com" />
        </InputGroup>
        <FieldDescription>We only use this address for workspace invites.</FieldDescription>
      </Field>
    </div>
  ),
};

export const ErrorState: Story = {
  args: {},
  render: () => (
    <div className="w-88">
      <Field data-invalid>
        <FieldLabel htmlFor="input-group-invalid">Email</FieldLabel>
        <InputGroup>
          <InputGroupAddon>
            <MailIcon />
          </InputGroupAddon>
          <InputGroupInput
            id="input-group-invalid"
            type="email"
            aria-invalid
            aria-describedby="input-group-error"
            defaultValue="not-an-email"
          />
        </InputGroup>
        <FieldError id="input-group-error">Enter a valid email address.</FieldError>
      </Field>
    </div>
  ),
};

export const WithTrailingShortcut: Story = {
  args: {},
  render: () => (
    <div className="w-88">
      <Label htmlFor="input-group-search" className="sr-only">
        Search sessions
      </Label>
      <InputGroup>
        <InputGroupAddon>
          <SearchIcon />
        </InputGroupAddon>
        <InputGroupInput id="input-group-search" placeholder="Search sessions" />
        <InputGroupAddon align="inline-end">
          <Kbd>
            <CommandIcon />K
          </Kbd>
        </InputGroupAddon>
      </InputGroup>
    </div>
  ),
};

export const TextareaWithActions: Story = {
  args: {},
  render: () => (
    <div className="w-md">
      <Field>
        <FieldLabel htmlFor="input-group-prompt">Prompt</FieldLabel>
        <InputGroup>
          <InputGroupTextarea
            id="input-group-prompt"
            placeholder="Describe what the agent should do..."
            defaultValue="Summarize the last 10 events and recommend a next step."
          />
          <InputGroupAddon align="block-end">
            <InputGroupText>2 attachments</InputGroupText>
            <InputGroupButton size="sm" variant="outline" className="ml-auto">
              Send
            </InputGroupButton>
          </InputGroupAddon>
        </InputGroup>
        <FieldDescription>Press ⌘⏎ to submit or continue typing to refine.</FieldDescription>
      </Field>
    </div>
  ),
};

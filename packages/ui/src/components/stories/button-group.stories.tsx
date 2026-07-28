import type { Meta, StoryObj } from "@storybook/react-vite";
import { AlignCenterIcon, AlignLeftIcon, AlignRightIcon, CopyIcon, TrashIcon } from "lucide-react";

import { Button } from "../button";
import { ButtonGroup, ButtonGroupSeparator, ButtonGroupText } from "../button-group";
import { Field, FieldDescription, FieldError, FieldLabel } from "../field";
import { InputGroup, InputGroupAddon, InputGroupInput } from "../input-group";

const meta: Meta<typeof ButtonGroup> = {
  title: "components/ui/ButtonGroup",
  component: ButtonGroup,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Visually joined button cluster. Pair with label and helper text for settings, or stack vertically for side panels.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  render: () => (
    <ButtonGroup>
      <Button variant="outline">
        <AlignLeftIcon />
      </Button>
      <Button variant="outline">
        <AlignCenterIcon />
      </Button>
      <Button variant="outline">
        <AlignRightIcon />
      </Button>
    </ButtonGroup>
  ),
};

export const WithLabelAndHelper: Story = {
  args: {},
  render: () => (
    <div className="w-88">
      <Field>
        <FieldLabel htmlFor="button-group-port">Daemon port</FieldLabel>
        <ButtonGroup>
          <InputGroup className="rounded-r-none">
            <InputGroupInput id="button-group-port" defaultValue="2123" />
            <InputGroupAddon align="inline-end">TCP</InputGroupAddon>
          </InputGroup>
          <Button variant="outline">Test</Button>
        </ButtonGroup>
        <FieldDescription>Apply the port after restarting the daemon.</FieldDescription>
      </Field>
    </div>
  ),
};

export const ErrorState: Story = {
  args: {},
  render: () => (
    <div className="w-88">
      <Field data-invalid>
        <FieldLabel htmlFor="button-group-invalid">Daemon port</FieldLabel>
        <ButtonGroup>
          <InputGroup className="rounded-r-none">
            <InputGroupInput
              id="button-group-invalid"
              aria-invalid
              aria-describedby="button-group-error"
              defaultValue="0"
            />
          </InputGroup>
          <Button variant="outline">Test</Button>
        </ButtonGroup>
        <FieldError id="button-group-error">Port must be between 1024 and 65535.</FieldError>
      </Field>
    </div>
  ),
};

export const WithSeparator: Story = {
  args: {},
  render: () => (
    <ButtonGroup>
      <Button variant="outline">
        <CopyIcon />
        Duplicate
      </Button>
      <ButtonGroupSeparator />
      <Button variant="outline" aria-label="Delete">
        <TrashIcon />
      </Button>
      <ButtonGroupText>3 selected</ButtonGroupText>
    </ButtonGroup>
  ),
};

export const Vertical: Story = {
  args: {},
  render: () => (
    <ButtonGroup orientation="vertical">
      <Button variant="outline">Approve</Button>
      <Button variant="outline">Reject</Button>
      <Button variant="outline">Request changes</Button>
    </ButtonGroup>
  ),
};

import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { Label } from "../label";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "../select";

const meta: Meta<typeof Select> = {
  title: "components/ui/Select",
  component: Select,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Base UI powered select with keyboard navigation. Arrow keys move between options; Enter selects; Escape closes. Item labels and groups compose into the popover.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const agents = [
  { value: "claude", label: "Claude Code" },
  { value: "codex", label: "Codex CLI" },
  { value: "gemini", label: "Gemini CLI" },
];

export const Default: Story = {
  render: () => (
    <div className="w-[18rem]">
      <Label htmlFor="select-agent" className="mb-1 block">
        Agent driver
      </Label>
      <Select defaultValue={agents[0].value}>
        <SelectTrigger id="select-agent" className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {agents.map(agent => (
            <SelectItem key={agent.value} value={agent.value}>
              {agent.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  ),
};

export const Grouped: Story = {
  parameters: {
    docs: {
      description: {
        story: "Use `SelectGroup` + `SelectLabel` + `SelectSeparator` to organize options.",
      },
    },
  },
  render: () => (
    <div className="w-[18rem]">
      <Select defaultValue="claude">
        <SelectTrigger className="w-full">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            <SelectLabel>Local</SelectLabel>
            <SelectItem value="claude">Claude Code</SelectItem>
            <SelectItem value="codex">Codex CLI</SelectItem>
          </SelectGroup>
          <SelectSeparator />
          <SelectGroup>
            <SelectLabel>Remote</SelectLabel>
            <SelectItem value="gemini">Gemini CLI</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
    </div>
  ),
};

export const Small: Story = {
  render: () => (
    <Select defaultValue={agents[0].value}>
      <SelectTrigger size="sm" className="w-48">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {agents.map(agent => (
          <SelectItem key={agent.value} value={agent.value}>
            {agent.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  ),
};

export const KeyboardNavigation: Story = {
  render: () => (
    <div className="w-[18rem]">
      <Select>
        <SelectTrigger className="w-full" aria-label="Agent">
          <SelectValue placeholder="Pick an agent" />
        </SelectTrigger>
        <SelectContent>
          {agents.map(agent => (
            <SelectItem key={agent.value} value={agent.value}>
              {agent.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = await canvas.findByRole("combobox", { name: "Agent" });
    await userEvent.click(trigger);
    await waitFor(() => expect(within(document.body).getByRole("listbox")).toBeInTheDocument());
    await userEvent.keyboard("{ArrowDown}");
    await userEvent.keyboard("{Escape}");
    await waitFor(
      () => expect(within(document.body).queryByRole("listbox")).not.toBeInTheDocument(),
      { timeout: 2000 }
    );
  },
};

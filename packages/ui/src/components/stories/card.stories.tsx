import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../button";
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "../card";

const meta: Meta<typeof Card> = {
  title: "components/ui/Card",
  component: Card,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Surface container with header, content, footer, and action slots.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[420px] bg-background p-4 text-foreground">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  render: () => (
    <Card>
      <CardHeader>
        <CardTitle>Session summary</CardTitle>
        <CardDescription>
          A composed card showing the full header / content / footer surface.
        </CardDescription>
        <CardAction>
          <Button size="sm" variant="outline">
            View
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent>
        <p className="text-muted-foreground">
          Agents completed 12 steps with 3 retries and 0 manual interruptions.
        </p>
      </CardContent>
      <CardFooter>
        <span className="text-xs text-muted-foreground">session / primary</span>
      </CardFooter>
    </Card>
  ),
};

export const Small: Story = {
  args: {},
  render: () => (
    <Card size="sm">
      <CardHeader>
        <CardTitle>Compact tile</CardTitle>
        <CardDescription>Denser spacing for list-adjacent surfaces.</CardDescription>
      </CardHeader>
      <CardContent>
        <p className="text-muted-foreground">Useful for sidebars and inline summaries.</p>
      </CardContent>
    </Card>
  ),
};

export const Selected: Story = {
  name: "Selected",
  args: {},
  render: () => (
    <div className="flex flex-col gap-3">
      <Card size="sm" className="bg-surface-glaze shadow-inset-strong">
        <CardHeader>
          <CardTitle>Agent · claude-sonnet</CardTitle>
          <CardDescription>Selected — neutral glaze fill + inset rim.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground">
            The selected surface reads via glaze + inset, never a decorative accent bar.
          </p>
        </CardContent>
      </Card>
      <Card size="sm">
        <CardHeader>
          <CardTitle>Agent · codex</CardTitle>
          <CardDescription>Idle, last activity 4m ago.</CardDescription>
        </CardHeader>
        <CardContent>
          <p className="text-muted-foreground">Unselected cards stay fully neutral.</p>
        </CardContent>
      </Card>
    </div>
  ),
};

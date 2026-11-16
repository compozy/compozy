import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { CircleCheck, KeyRound, ShieldCheck } from "../../../icons";
import { Button } from "../../button";
import { Popover, PopoverTrigger } from "../../popover";
import { Disclosure } from "../disclosure";
import { PopoverPanel, PopoverPanelContent, type PopoverPanelTone } from "../popover-panel";

const meta: Meta<typeof PopoverPanel> = {
  title: "components/custom/PopoverPanel",
  component: PopoverPanel,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "A decision panel beside the control that opened it: an identity well whose tone states the situation, a title with one quiet subtitle, the body, an optional ruled aside, and a ruled footer with the actions. Render inside `PopoverPanelContent`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Example({
  tone,
  icon,
  title,
  aside,
}: {
  tone: PopoverPanelTone;
  icon: typeof KeyRound;
  title: string;
  aside?: boolean;
}) {
  return (
    <Popover defaultOpen modal={false}>
      <PopoverTrigger render={<Button variant="secondary" size="sm" />}>Open panel</PopoverTrigger>
      <PopoverPanelContent side="bottom" align="end">
        <PopoverPanel
          icon={icon}
          tone={tone}
          title={title}
          subtitle="example.com"
          onClose={() => undefined}
          aside={
            aside ? (
              <Disclosure label="More options">
                <p className="text-meta text-muted-foreground">The folded fallback.</p>
              </Disclosure>
            ) : undefined
          }
          footer={
            <>
              <Button type="button" size="sm" variant="ghost" className="ml-auto">
                Not now
              </Button>
              <Button type="button" size="sm">
                Continue
              </Button>
            </>
          }
        >
          <p className="text-small-body text-fg">One fact the person needs to decide.</p>
        </PopoverPanel>
      </PopoverPanelContent>
    </Popover>
  );
}

export const Neutral: Story = {
  render: () => <Example tone="neutral" icon={KeyRound} title="Sign in to example" aside />,
};

export const Warning: Story = {
  render: () => <Example tone="warning" icon={ShieldCheck} title="Allow example?" />,
};

export const Success: Story = {
  tags: ["play-fn"],
  render: () => <Example tone="success" icon={CircleCheck} title="Connected to example" />,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    const panel = await body.findByRole("dialog", { name: "Connected to example" });
    await expect(panel).toBeVisible();
    await userEvent.click(within(panel).getByRole("button", { name: "Continue" }));
  },
};

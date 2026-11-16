import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { CircleCheck, KeyRound, ShieldCheck } from "../../../icons";
import { Button } from "../../button";
import { Popover, PopoverTrigger } from "../../popover";
import { Disclosure } from "../disclosure";
import { KindIcon } from "../kind-icon";
import { PopoverPanel, PopoverPanelContent, type PopoverPanelTone } from "../popover-panel";

// Render-only stories: the identity union (glyph or mark) has no single args shape.
const meta: Meta = {
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
type Story = StoryObj;

function Example({
  tone,
  icon,
  mark,
  title,
  aside,
}: {
  tone: PopoverPanelTone;
  icon?: typeof KeyRound;
  mark?: React.ReactNode;
  title: string;
  aside?: boolean;
}) {
  const identity = icon ? { icon } : { mark };
  return (
    <Popover defaultOpen modal={false}>
      <PopoverTrigger render={<Button variant="secondary" size="sm" />}>Open panel</PopoverTrigger>
      <PopoverPanelContent side="bottom" align="end">
        <PopoverPanel
          {...identity}
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

export const WithMark: Story = {
  render: () => (
    <Example
      tone="neutral"
      mark={<KindIcon aria-hidden="true" tone="well" kind="claude" size="sm" />}
      title="Add example to CompozyOS"
    />
  ),
};

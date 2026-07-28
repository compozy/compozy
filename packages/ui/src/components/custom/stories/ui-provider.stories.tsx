import type { Meta, StoryObj } from "@storybook/react-vite";
import { useReducedMotionConfig } from "motion/react";
import { expect, within } from "storybook/test";

import { UIProvider } from "../ui-provider";

function ReducedMotionProbe() {
  const shouldReduce = useReducedMotionConfig();
  return (
    <output
      data-testid="reduced-motion"
      className="inline-flex h-8 items-center rounded-md border border-border bg-surface px-3 font-mono text-xs text-foreground"
    >
      {`reduced-motion: ${String(shouldReduce ?? false)}`}
    </output>
  );
}

const meta: Meta<typeof UIProvider> = {
  title: "components/custom/UIProvider",
  component: UIProvider,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Root provider wiring MotionConfig with `reducedMotion='user'` by default. Pass `reducedMotion='always'` to globally suppress motion (tests, print, demos) or `never` to force motion regardless of OS preference.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default configuration , `reducedMotion='user'` defers to the OS preference.
 */
export const Default: Story = {
  args: { reducedMotion: "user" },
  render: args => (
    <UIProvider {...args}>
      <ReducedMotionProbe />
    </UIProvider>
  ),
};

/**
 * Verifies that `reducedMotion='always'` forwards to MotionConfig and makes
 * `useReducedMotion()` return `true` for every consumer, independent of OS
 * settings.
 */
export const ReducedMotionAlways: Story = {
  args: { reducedMotion: "always" },
  render: args => (
    <UIProvider {...args}>
      <ReducedMotionProbe />
    </UIProvider>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const probe = await canvas.findByTestId("reduced-motion");
    await expect(probe).toHaveTextContent("reduced-motion: true");
  },
};

/**
 * Explicit opt-out , `reducedMotion='never'` keeps motion enabled even when
 * the OS requests reduced motion. Verified by the probe reporting `false`.
 */
export const ReducedMotionNever: Story = {
  args: { reducedMotion: "never" },
  render: args => (
    <UIProvider {...args}>
      <ReducedMotionProbe />
    </UIProvider>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const probe = await canvas.findByTestId("reduced-motion");
    await expect(probe).toHaveTextContent("reduced-motion: false");
  },
};

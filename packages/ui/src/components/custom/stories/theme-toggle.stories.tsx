import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { ThemeToggle } from "../theme-toggle";

const meta: Meta<typeof ThemeToggle> = {
  title: "components/custom/ThemeToggle",
  component: ThemeToggle,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Rail-foot light/dark switch: a 40px quiet icon button (`muted`, hover steps to `surface-2`) that shows the theme it switches to and names it (`Switch to light mode`). Presentational — the shell passes the resolved theme and persists the flip.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Painted dark: offers light (sun). */
export const Dark: Story = {
  args: { resolvedTheme: "dark" },
};

/** Painted light: offers dark (moon). */
export const Light: Story = {
  args: { resolvedTheme: "light" },
};

function ThemeToggleHarness() {
  const [theme, setTheme] = useState<"light" | "dark">("dark");
  return (
    <ThemeToggle
      resolvedTheme={theme}
      onClick={() => setTheme(current => (current === "dark" ? "light" : "dark"))}
    />
  );
}

/** Each press flips the theme and the accessible name follows. */
export const Toggling: Story = {
  args: { resolvedTheme: "dark" },
  render: () => <ThemeToggleHarness />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Switch to light mode" }));
    await waitFor(() =>
      expect(canvas.getByRole("button", { name: "Switch to dark mode" })).toBeInTheDocument()
    );
  },
};

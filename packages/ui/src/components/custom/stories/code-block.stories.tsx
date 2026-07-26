import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { CodeBlock, CopyIconButton } from "../code-block";

const meta: Meta<typeof CodeBlock> = {
  title: "components/custom/CodeBlock",
  component: CodeBlock,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Terminal-style code block per DESIGN.md §4. Canvas-deep container, JetBrains Mono body at 14px/1.6, optional accent `$ ` prompt, Vitesse syntax highlighting via Shiki, optional language eyebrow, and a ghost copy button with success/failure feedback.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const ShellCommand: Story = {
  args: {
    code: "agh start",
  },
  parameters: {
    docs: {
      description: {
        story: "Default shell prompt: single command with the accent `$ ` prompt.",
      },
    },
  },
};

export const MultilineWithoutPrompt: Story = {
  args: {
    showPrompt: false,
    language: "typescript",
    themeMode: "dark",
    code: `export function greet(name: string) {
  return \`Hello, \${name}\`;
}`,
  },
  parameters: {
    docs: {
      description: {
        story: "Source-code block with prompts disabled, rendered as plain code.",
      },
    },
  },
};

export const LanguageLabel: Story = {
  args: {
    caption: "agh network",
    language: "bash",
    code: `# discover peers, send one task
agh network status
agh network peers
agh network send reviewer --kind direct \\
    --body '{"task":"review PR #482"}'`,
  },
  parameters: {
    docs: {
      description: {
        story:
          "Language eyebrow in the top-left. Comment (`#`) and indented continuation lines skip the prompt.",
      },
    },
  },
};

export const UnknownLanguageFallback: Story = {
  args: {
    showPrompt: false,
    language: "not-a-language",
    code: ['<unsafe-tag data-value="render as text">', "  escaped: true", "</unsafe-tag>"].join(
      "\n"
    ),
  },
  parameters: {
    docs: {
      description: {
        story:
          "Unsupported language labels remain visible, but the body falls back to escaped plain text.",
      },
    },
  },
};

export const LineNumbersAndHighlights: Story = {
  args: {
    showPrompt: false,
    showLineNumbers: true,
    highlightLines: [2, 4],
    language: "typescript",
    code: [
      "type AgentState = 'idle' | 'running' | 'blocked';",
      "const state: AgentState = 'running';",
      "const canResume = state !== 'blocked';",
      "console.log({ state, canResume });",
    ].join("\n"),
  },
};

export const WrappedLongLine: Story = {
  args: {
    showPrompt: false,
    wrapLines: true,
    language: "json",
    code: JSON.stringify(
      {
        event: "receipt",
        channel: "agh-network/v0",
        summary:
          "This deliberately long value wraps inside the block without forcing a horizontal scroll.",
      },
      null,
      2
    ),
  },
};

export const CopyDisabled: Story = {
  args: {
    copyable: false,
    code: "agh start",
  },
  parameters: {
    docs: {
      description: {
        story: "Static code block with the copy affordance hidden.",
      },
    },
  },
};

export const CopyInteraction: Story = {
  args: {
    code: "agh network status",
  },
  parameters: {
    docs: {
      description: {
        story: "Interaction test: click copy, assert checkmark appears, assert revert after 1.5s.",
      },
    },
  },
  play: async ({ canvasElement, step }) => {
    const canvas = within(canvasElement);
    const writeText = fn(async (_value: string) => undefined);
    const original = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    try {
      await step("Copy button renders with the copy glyph", async () => {
        const root = canvasElement.querySelector<HTMLElement>('[data-slot="code-block"]');
        await expect(root).not.toBeNull();
        const button = await canvas.findByRole("button", { name: "Copy to clipboard" });
        await expect(button.querySelector("svg.lucide-copy")).not.toBeNull();
      });
      await step("Clicking copy invokes navigator.clipboard.writeText", async () => {
        const button = await canvas.findByRole("button", { name: "Copy to clipboard" });
        await userEvent.click(button);
        await waitFor(() => expect(writeText).toHaveBeenCalledWith("agh network status"));
      });
      await step("Button swaps to the check glyph", async () => {
        const success = await canvas.findByRole("button", { name: "Copied" });
        await expect(success.getAttribute("data-copied")).toBe("true");
        await expect(success.querySelector("svg.lucide-check")).not.toBeNull();
      });
      await step("Button reverts to copy glyph after ~1.5s", async () => {
        await waitFor(
          async () => {
            const reverted = await canvas.findByRole("button", { name: "Copy to clipboard" });
            await expect(reverted.getAttribute("data-copied")).toBeNull();
            await expect(reverted.querySelector("svg.lucide-copy")).not.toBeNull();
          },
          { timeout: 2500 }
        );
      });
    } finally {
      if (original) {
        Object.defineProperty(navigator, "clipboard", original);
      } else {
        Reflect.deleteProperty(navigator as unknown as { clipboard?: unknown }, "clipboard");
      }
    }
  },
};

export const WarningToneTruncated: Story = {
  args: {
    code: [
      "First warning line",
      "Second warning line",
      "Third warning line",
      "Fourth warning line",
    ].join("\n"),
    showPrompt: false,
    tone: "warning",
    truncateLines: 2,
  },
};

export const CopyFailure: Story = {
  args: {
    code: "agh network status",
  },
  play: async ({ canvasElement, step }) => {
    const canvas = within(canvasElement);
    const original = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: fn(async (_value: string) => {
          throw new Error("blocked");
        }),
      },
    });
    try {
      await step("Clicking copy exposes failure feedback", async () => {
        const button = await canvas.findByRole("button", { name: "Copy to clipboard" });
        await userEvent.click(button);
        const failed = await canvas.findByRole("button", { name: "Copy failed" });
        await expect(failed.getAttribute("data-copy-state")).toBe("failed");
      });
    } finally {
      if (original) {
        Object.defineProperty(navigator, "clipboard", original);
      } else {
        Reflect.deleteProperty(navigator as unknown as { clipboard?: unknown }, "clipboard");
      }
    }
  },
};

export const StandaloneCopyButton: Story = {
  args: {
    code: "agh network status",
  },
  render: args => (
    <div className="flex items-center gap-3 rounded-md border border-line bg-canvas-soft p-3">
      <span className="font-mono text-small-body text-muted">{args.code}</span>
      <CopyIconButton value={args.code} />
    </div>
  ),
};

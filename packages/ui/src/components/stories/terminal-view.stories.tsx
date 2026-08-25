import type { Meta, StoryObj } from "@storybook/react-vite";
import * as React from "react";

import { TerminalView, type TerminalViewHandle } from "../terminal/terminal-view";

/**
 * The stories drive the real emulator: the engine loader is left at its default
 * so what renders here is what renders in the app, byte for byte.
 */
const meta: Meta<typeof TerminalView> = {
  title: "components/ui/TerminalView",
  component: TerminalView,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Live byte-stream grid. Bytes arrive through the imperative handle, the size it can host is proposed but never self-applied, and read-only suppresses local input without claiming authority over who may write.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Written out rather than pasted raw, so the sequences stay readable in source. */
const ESC = "\u001b";

function sgr(code: string, text: string): string {
  return `${ESC}[${code}m${text}${ESC}[0m`;
}

const SESSION_TRANSCRIPT = [
  `${sgr("32", "$")} bun run dev\r\n`,
  `${sgr("1;32", "  VITE v6.0.3")}  ready in ${sgr("1", "412 ms")}\r\n\r\n`,
  `  ${sgr("32", "➜")}  ${sgr("1", "Local:")}   ${sgr("36", "http://localhost:5173/")}\r\n`,
  `  ${sgr("32", "➜")}  ${sgr("1", "Network:")} ${sgr("36", "http://192.168.0.14:5173/")}\r\n\r\n`,
  `${sgr("90", "12:41:03")} ${sgr("36", "[vite]")} hmr update ${sgr("34", "/src/systems/terminal/terminal-pane.tsx")}\r\n`,
  `${sgr("90", "12:41:09")} ${sgr("36", "[vite]")} page reload ${sgr("34", "src/routes/_app/terminal/index.tsx")}\r\n`,
  `${sgr("32", "$")} `,
].join("");

const PALETTE_PROBE = buildPaletteProbe();

/** Writes a fixture transcript once the emulator is attached. */
function useSeededTerminal(payload: string) {
  const handleRef = React.useRef<TerminalViewHandle>(null);
  React.useEffect(() => {
    let cancelled = false;
    const timer = setTimeout(() => {
      if (!cancelled) void handleRef.current?.write(payload);
    }, 0);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [payload]);
  return handleRef;
}

function TerminalStage({
  instanceId,
  payload,
  readOnly,
  label,
}: {
  instanceId: string;
  payload: string;
  readOnly?: boolean;
  label: string;
}) {
  const handleRef = useSeededTerminal(payload);
  return (
    <div className="flex h-[420px] w-full flex-col bg-terminal-bg font-mono text-[12.5px] leading-[1.5] tracking-[0.02em]">
      <TerminalView
        aria-label={label}
        className="px-3.5 pt-2.5 pb-3"
        handleRef={handleRef}
        instanceId={instanceId}
        readOnly={readOnly}
      />
    </div>
  );
}

export const Writable: Story = {
  name: "Writable — solid cursor",
  render: () => (
    <TerminalStage
      instanceId="story-terminal-writable"
      label="Terminal output"
      payload={SESSION_TRANSCRIPT}
    />
  ),
};

export const ReadOnly: Story = {
  name: "Read-only — hollow cursor",
  render: () => (
    <TerminalStage
      instanceId="story-terminal-readonly"
      label="Terminal output — watching"
      payload={SESSION_TRANSCRIPT}
      readOnly
    />
  ),
};

export const AnsiRamp: Story = {
  name: "ANSI ramp",
  parameters: {
    docs: {
      description: {
        story:
          "Every index of the ramp, painted by the emulator from the `--terminal-ansi-*` tokens. Indices 1, 2 and 3 ride the danger, success and warning lanes; index 8 is dim by terminal convention and is never a legal colour for UI copy.",
      },
    },
  },
  render: () => (
    <TerminalStage
      instanceId="story-terminal-ansi"
      label="ANSI colour ramp"
      payload={PALETTE_PROBE}
      readOnly
    />
  ),
};

function buildPaletteProbe(): string {
  const standard = Array.from({ length: 8 }, (_unused, index) =>
    sgr(`3${index}`, ` ${String(index).padStart(2, " ")} `)
  ).join("");
  const bright = Array.from({ length: 8 }, (_unused, index) =>
    sgr(`9${index}`, ` ${String(index + 8).padStart(2, " ")} `)
  ).join("");
  return [
    "ANSI 0-7\r\n",
    `${standard}\r\n\r\n`,
    "ANSI 8-15\r\n",
    `${bright}\r\n\r\n`,
    `${sgr("31", "fail")}  ${sgr("32", "pass")}  ${sgr("33", "approx")}  ${sgr("90", "dim")}\r\n`,
  ].join("");
}

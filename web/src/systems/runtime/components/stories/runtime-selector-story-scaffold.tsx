import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";

import type { RuntimeSpeed } from "@/lib/api-contract";

import {
  RuntimeSelector,
  type RuntimeSelectorProps,
  type RuntimeSelectorValue,
} from "../runtime-selector";

export function ControlledRuntimeSelector({ value: initial, ...props }: RuntimeSelectorProps) {
  const [value, setValue] = useState<RuntimeSelectorValue>(initial);
  // Speed state is controlled only when the story wires the request (mirrors
  // real surfaces: unwired selectors never show the switch).
  const [speed, setSpeed] = useState<RuntimeSpeed>(props.speed ?? "normal");
  const speedProps = props.onSpeedChange
    ? {
        speed,
        onSpeedChange: (next: RuntimeSpeed) => {
          props.onSpeedChange?.(next);
          setSpeed(next);
        },
      }
    : {};
  return (
    <RuntimeSelector
      {...props}
      {...speedProps}
      value={value}
      onChange={next => {
        props.onChange(next);
        setValue(next);
      }}
    />
  );
}

export function RuntimeSelectorStoryFrame(props: RuntimeSelectorProps) {
  return (
    // Fullscreen story envelope (no arbitrary fixed geometry); the popup portals to
    // the body and floats from the trigger, so the frame just needs viewport height.
    <div className="flex min-h-dvh items-start p-6">
      <ControlledRuntimeSelector {...props} />
    </div>
  );
}

export async function openPopup(canvasElement: HTMLElement) {
  const canvas = within(canvasElement);
  await userEvent.click(canvas.getByRole("button", { name: /^Runtime:/ }));
  const body = within(canvasElement.ownerDocument.body);
  await expect(await body.findByTestId("runtime-selector-popup")).toBeInTheDocument();
  return body;
}

/** Story docs parameters for a single story, keeping the description beside its config. */
export function storyDocs(story: string) {
  return { docs: { description: { story } } };
}

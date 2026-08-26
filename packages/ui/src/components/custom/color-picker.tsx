import * as React from "react";
import { HexColorPicker } from "react-colorful";

import { cn } from "../../lib/utils";

export interface ColorPickerProps extends Omit<React.ComponentProps<"div">, "onChange"> {
  /** Current color as `#rrggbb`. */
  value: string;
  onChange: (next: string) => void;
}

// react-colorful injects its stylesheet outside Tailwind's cascade layers, so
// every override that collides with it must carry `!` to win.
const PICKER_CLASS = [
  "[&_.react-colorful]:h-color-picker-area! [&_.react-colorful]:w-full! [&_.react-colorful]:gap-2",
  "[&_.react-colorful__saturation]:rounded-md! [&_.react-colorful__saturation]:border-b-0!",
  "[&_.react-colorful__saturation]:shadow-none!",
  "[&_.react-colorful__hue]:h-2.5! [&_.react-colorful__hue]:rounded-full!",
  "[&_.react-colorful__pointer]:size-3.5! [&_.react-colorful__pointer]:shadow-none!",
  "[&_.react-colorful__interactive]:focus-visible:shadow-focus-ring",
].join(" ");

/** Saturation area plus hue slider; hex I/O stays with the caller. */
export function ColorPicker({ className, value, onChange, ...props }: ColorPickerProps) {
  return (
    <div data-slot="color-picker" className={cn(PICKER_CLASS, className)} {...props}>
      <HexColorPicker color={value} onChange={onChange} />
    </div>
  );
}

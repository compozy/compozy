"use client";

import { Slider as SliderPrimitive } from "@base-ui/react/slider";
import { useId } from "react";

import { cn } from "../lib/utils";

/**
 * shadcn `base-nova` slider, repainted onto the token ramp and the house focus
 * contract, with two corrections to the upstream snippet:
 *
 * - Thumb count follows the value's own shape. Upstream fell back to
 *   `[min, max]`, which renders a range slider for anyone passing one number.
 * - The accessible name reaches the thumb. Base UI puts `role="slider"` on the
 *   thumb, so a label left on the root names a group nobody can operate.
 */
type SliderProps<Value extends number | readonly number[]> = SliderPrimitive.Root.Props<Value> & {
  /** Gives each thumb an explicit name when a range needs domain-specific language. */
  getAriaLabel?: NonNullable<SliderPrimitive.Thumb.Props["getAriaLabel"]>;
};

function Slider<Value extends number | readonly number[] = number>({
  className,
  defaultValue,
  value,
  min = 0,
  max = 100,
  "aria-label": ariaLabel,
  "aria-labelledby": ariaLabelledBy,
  getAriaLabel,
  ...props
}: SliderProps<Value>) {
  const resolved = value ?? defaultValue;
  const thumbCount = Array.isArray(resolved) ? resolved.length : 1;
  const thumbDescriptionId = useId();
  const thumbDescriptor = (index: number) => {
    if (thumbCount === 2) return index === 0 ? "Minimum" : "Maximum";
    return thumbCount === 1 ? "Value" : `Value ${index + 1}`;
  };
  const resolvedGetAriaLabel =
    getAriaLabel ??
    (ariaLabelledBy
      ? undefined
      : (index: number) =>
          thumbCount === 1
            ? (ariaLabel ?? "Slider")
            : `${ariaLabel ?? "Slider"}: ${thumbDescriptor(index)}`);

  return (
    <SliderPrimitive.Root
      className={cn("data-horizontal:w-full data-vertical:h-full", className)}
      data-slot="slider"
      defaultValue={defaultValue}
      max={max}
      min={min}
      thumbAlignment="edge"
      value={value}
      {...props}
    >
      <SliderPrimitive.Control className="relative flex w-full touch-none items-center select-none data-disabled:cursor-not-allowed data-disabled:opacity-50 data-vertical:h-full data-vertical:min-h-40 data-vertical:w-auto data-vertical:flex-col">
        <SliderPrimitive.Track
          className="relative grow overflow-hidden rounded-pill bg-bar-fill select-none data-horizontal:h-1 data-horizontal:w-full data-vertical:h-full data-vertical:w-1"
          data-slot="slider-track"
        >
          <SliderPrimitive.Indicator
            className="bg-accent select-none data-horizontal:h-full data-vertical:w-full"
            data-slot="slider-range"
          />
        </SliderPrimitive.Track>
        {Array.from({ length: thumbCount }, (_, index) => (
          <span className="contents" key={index}>
            {ariaLabelledBy && thumbCount > 1 ? (
              <span className="sr-only" id={`${thumbDescriptionId}-${index}`}>
                {thumbDescriptor(index)}
              </span>
            ) : null}
            <SliderPrimitive.Thumb
              aria-labelledby={
                ariaLabelledBy
                  ? thumbCount > 1
                    ? `${ariaLabelledBy} ${thumbDescriptionId}-${index}`
                    : ariaLabelledBy
                  : undefined
              }
              className={cn(
                "relative block size-3 shrink-0 rounded-pill border border-line-strong bg-fg-strong",
                "transition-[box-shadow] duration-fast ease-out select-none after:absolute after:-inset-2",
                "focus-visible:shadow-focus-ring focus-visible:outline-none",
                "data-disabled:pointer-events-none"
              )}
              data-slot="slider-thumb"
              getAriaLabel={resolvedGetAriaLabel}
            />
          </span>
        ))}
      </SliderPrimitive.Control>
    </SliderPrimitive.Root>
  );
}

export { Slider };

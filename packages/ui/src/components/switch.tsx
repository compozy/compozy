"use client";

import { Switch as SwitchPrimitive } from "@base-ui/react/switch";

import { cn } from "../lib/utils";

function Switch({
  className,
  size = "default",
  ...props
}: SwitchPrimitive.Root.Props & {
  size?: "sm" | "default";
}) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      data-size={size}
      className={cn(
        "peer group/switch relative inline-flex shrink-0 items-center rounded-full border border-transparent transition-[background-color,border-color,box-shadow,opacity] duration-fast ease-out outline-none after:absolute after:-inset-x-3 after:-inset-y-2 focus-visible:outline-none focus-visible:shadow-focus-ring aria-invalid:border-danger data-[size=default]:h-switch-default data-[size=default]:w-switch-default data-[size=sm]:h-switch-sm data-[size=sm]:w-switch-sm data-checked:bg-accent data-unchecked:bg-elevated data-disabled:cursor-not-allowed data-disabled:opacity-50",
        className
      )}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className="pointer-events-none block rounded-full bg-fg-strong ring-0 transition-transform group-data-[size=default]/switch:size-4 group-data-[size=sm]/switch:size-3 group-data-[size=default]/switch:data-checked:translate-x-[calc(100%-var(--space-switch-thumb-inset))] group-data-[size=sm]/switch:data-checked:translate-x-[calc(100%-var(--space-switch-thumb-inset))] group-data-[size=default]/switch:data-unchecked:translate-x-0 group-data-[size=sm]/switch:data-unchecked:translate-x-0"
      />
    </SwitchPrimitive.Root>
  );
}

export { Switch };

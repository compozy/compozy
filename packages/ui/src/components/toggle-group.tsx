"use client";

import * as React from "react";
import { Toggle as TogglePrimitive } from "@base-ui/react/toggle";
import { ToggleGroup as ToggleGroupPrimitive } from "@base-ui/react/toggle-group";
import { type VariantProps } from "class-variance-authority";

import { cn } from "../lib/utils";
import { toggleVariants } from "./toggle-variants";

type ToggleVariant = VariantProps<typeof toggleVariants>["variant"];
type ToggleSize = VariantProps<typeof toggleVariants>["size"];

const ToggleGroupVariantContext = React.createContext<ToggleVariant>("default");
const ToggleGroupSizeContext = React.createContext<ToggleSize>("default");
const ToggleGroupSpacingContext = React.createContext(0);

function ToggleGroup({
  className,
  variant,
  size,
  spacing = 0,
  orientation = "horizontal",
  children,
  ...props
}: ToggleGroupPrimitive.Props &
  VariantProps<typeof toggleVariants> & {
    spacing?: number;
    orientation?: "horizontal" | "vertical";
  }) {
  return (
    <ToggleGroupPrimitive
      data-slot="toggle-group"
      data-variant={variant}
      data-size={size}
      data-spacing={spacing}
      data-orientation={orientation}
      style={{ "--gap": spacing } as React.CSSProperties}
      className={cn(
        "group/toggle-group flex w-fit flex-row items-center gap-[--spacing(var(--gap))] rounded-md data-vertical:flex-col data-vertical:items-stretch",
        "data-[spacing=0]:bg-canvas-soft data-[spacing=0]:border data-[spacing=0]:border-line data-[spacing=0]:p-0.5",
        className
      )}
      {...props}
    >
      <ToggleGroupVariantContext value={variant}>
        <ToggleGroupSizeContext value={size}>
          <ToggleGroupSpacingContext value={spacing}>{children}</ToggleGroupSpacingContext>
        </ToggleGroupSizeContext>
      </ToggleGroupVariantContext>
    </ToggleGroupPrimitive>
  );
}

function ToggleGroupItem({
  className,
  children,
  variant = "default",
  size = "default",
  ...props
}: TogglePrimitive.Props & VariantProps<typeof toggleVariants>) {
  const contextVariant = React.use(ToggleGroupVariantContext);
  const contextSize = React.use(ToggleGroupSizeContext);
  const contextSpacing = React.use(ToggleGroupSpacingContext);

  return (
    <TogglePrimitive
      data-slot="toggle-group-item"
      data-variant={contextVariant || variant}
      data-size={contextSize || size}
      data-spacing={contextSpacing}
      className={cn(
        "shrink-0 group-data-[spacing=0]/toggle-group:rounded-sm group-data-[spacing=0]/toggle-group:px-2 group-data-[spacing=0]/toggle-group:bg-transparent group-data-[spacing=0]/toggle-group:hover:bg-hover focus:z-10 focus-visible:z-10 group-data-[spacing=0]/toggle-group:has-data-[icon=inline-end]:pr-1.5 group-data-[spacing=0]/toggle-group:has-data-[icon=inline-start]:pl-1.5 group-data-horizontal/toggle-group:data-[spacing=0]:data-[variant=outline]:border-l-0 group-data-vertical/toggle-group:data-[spacing=0]:data-[variant=outline]:border-t-0 group-data-horizontal/toggle-group:data-[spacing=0]:data-[variant=outline]:first:border-l group-data-vertical/toggle-group:data-[spacing=0]:data-[variant=outline]:first:border-t",
        toggleVariants({
          variant: contextVariant || variant,
          size: contextSize || size,
        }),
        className
      )}
      {...props}
    >
      {children}
    </TogglePrimitive>
  );
}

export { ToggleGroup, ToggleGroupItem };

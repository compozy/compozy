"use client";

import * as React from "react";
import { Menubar as MenubarPrimitive } from "@base-ui/react/menubar";

import { cn } from "../lib/utils";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
  DropdownMenuTrigger,
} from "./dropdown-menu";

/**
 * Persistent menu bar: `role="menubar"` with arrow-key traversal, Home/End, and
 * hover-to-switch once one menu is open. Modality is declared once here — a
 * `modal` prop on a menubar-parented `MenubarMenu` is ignored by base-ui and
 * logs a warning, so never forward one.
 *
 * Every popup part delegates to the `DropdownMenu` primitive, so both menu
 * surfaces stay one visual system; only the bar and its triggers are new.
 */
function Menubar({ className, ...props }: MenubarPrimitive.Props) {
  return (
    <MenubarPrimitive
      data-slot="menubar"
      className={cn("flex items-center gap-0.5", className)}
      {...props}
    />
  );
}

function MenubarMenu({ ...props }: React.ComponentProps<typeof DropdownMenu>) {
  return <DropdownMenu data-slot="menubar-menu" {...props} />;
}

function MenubarTrigger({ className, ...props }: React.ComponentProps<typeof DropdownMenuTrigger>) {
  return (
    <DropdownMenuTrigger
      data-slot="menubar-trigger"
      className={cn(
        "flex cursor-default items-center rounded-md px-2 py-1 text-small-body text-muted transition-colors duration-fast outline-none select-none hover:bg-elevated hover:text-fg-strong focus-visible:shadow-focus-ring data-popup-open:bg-elevated data-popup-open:text-fg-strong",
        className
      )}
      {...props}
    />
  );
}

function MenubarContent({
  className,
  sideOffset = 6,
  ...props
}: React.ComponentProps<typeof DropdownMenuContent>) {
  return (
    <DropdownMenuContent
      data-slot="menubar-content"
      sideOffset={sideOffset}
      // Menubar triggers are narrow, so the anchor width the dropdown inherits
      // would clip long labels; menu popups size to their content instead.
      className={cn("w-auto min-w-56", className)}
      {...props}
    />
  );
}

function MenubarGroup({ ...props }: React.ComponentProps<typeof DropdownMenuGroup>) {
  return <DropdownMenuGroup data-slot="menubar-group" {...props} />;
}

function MenubarLabel({ ...props }: React.ComponentProps<typeof DropdownMenuLabel>) {
  return <DropdownMenuLabel data-slot="menubar-label" {...props} />;
}

function MenubarItem({ ...props }: React.ComponentProps<typeof DropdownMenuItem>) {
  return <DropdownMenuItem data-slot="menubar-item" {...props} />;
}

function MenubarCheckboxItem({ ...props }: React.ComponentProps<typeof DropdownMenuCheckboxItem>) {
  return <DropdownMenuCheckboxItem data-slot="menubar-checkbox-item" {...props} />;
}

function MenubarRadioGroup({ ...props }: React.ComponentProps<typeof DropdownMenuRadioGroup>) {
  return <DropdownMenuRadioGroup data-slot="menubar-radio-group" {...props} />;
}

function MenubarRadioItem({ ...props }: React.ComponentProps<typeof DropdownMenuRadioItem>) {
  return <DropdownMenuRadioItem data-slot="menubar-radio-item" {...props} />;
}

function MenubarSeparator({ ...props }: React.ComponentProps<typeof DropdownMenuSeparator>) {
  return <DropdownMenuSeparator data-slot="menubar-separator" {...props} />;
}

function MenubarShortcut({ ...props }: React.ComponentProps<typeof DropdownMenuShortcut>) {
  return <DropdownMenuShortcut data-slot="menubar-shortcut" {...props} />;
}

function MenubarSub({ ...props }: React.ComponentProps<typeof DropdownMenuSub>) {
  return <DropdownMenuSub data-slot="menubar-sub" {...props} />;
}

function MenubarSubTrigger({ ...props }: React.ComponentProps<typeof DropdownMenuSubTrigger>) {
  return <DropdownMenuSubTrigger data-slot="menubar-sub-trigger" {...props} />;
}

function MenubarSubContent({ ...props }: React.ComponentProps<typeof DropdownMenuSubContent>) {
  return <DropdownMenuSubContent data-slot="menubar-sub-content" {...props} />;
}

export {
  Menubar,
  MenubarCheckboxItem,
  MenubarContent,
  MenubarGroup,
  MenubarItem,
  MenubarLabel,
  MenubarMenu,
  MenubarRadioGroup,
  MenubarRadioItem,
  MenubarSeparator,
  MenubarShortcut,
  MenubarSub,
  MenubarSubContent,
  MenubarSubTrigger,
  MenubarTrigger,
};

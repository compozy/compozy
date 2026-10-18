import * as React from "react";
import { Command as CommandPrimitive } from "cmdk";
import { CheckIcon, SearchIcon } from "lucide-react";

import { cn } from "../lib/utils";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "./dialog";

function Command({ className, ...props }: React.ComponentProps<typeof CommandPrimitive>) {
  return (
    <CommandPrimitive
      data-slot="command"
      className={cn(
        "flex size-full flex-col overflow-hidden rounded-lg bg-popover p-1.5 text-fg",
        className
      )}
      {...props}
    />
  );
}

function CommandDialog({
  title = "Command Palette",
  description = "Search for a command to run...",
  children,
  className,
  showCloseButton = false,
  ...props
}: Omit<React.ComponentProps<typeof Dialog>, "children"> & {
  title?: string;
  description?: string;
  className?: string;
  showCloseButton?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Dialog {...props}>
      <DialogHeader className="sr-only">
        <DialogTitle>{title}</DialogTitle>
        <DialogDescription>{description}</DialogDescription>
      </DialogHeader>
      <DialogContent
        className={cn("top-1/3 translate-y-0 overflow-hidden rounded-lg p-0 shadow-pop", className)}
        showCloseButton={showCloseButton}
        unframed
      >
        {children}
      </DialogContent>
    </Dialog>
  );
}

export type CommandInputVariant = "default" | "quiet";

const COMMAND_INPUT_GROUP_CLASS: Record<CommandInputVariant, string> = {
  // Boxed field for form-like pickers.
  default:
    "h-control-compact gap-2 rounded-md border border-line bg-canvas px-2 text-small-body focus-within:border-line-strong focus-within:shadow-focus-ring",
  // Palette head: borderless and ringless on the canvas, the list below owns focus.
  quiet: "h-10 gap-2.5 bg-transparent px-2.5 text-body",
};

function CommandInput({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<typeof CommandPrimitive.Input> & { variant?: CommandInputVariant }) {
  return (
    <div
      data-slot="command-input-wrapper"
      data-variant={variant}
      className={variant === "quiet" ? "px-1" : "p-1 pb-0"}
    >
      <div
        data-slot="command-input-group"
        className={cn(
          "flex w-full min-w-0 items-center text-fg transition-colors outline-none",
          COMMAND_INPUT_GROUP_CLASS[variant]
        )}
      >
        <SearchIcon aria-hidden="true" className="size-4 shrink-0 text-subtle" />
        <CommandPrimitive.Input
          data-slot="command-input"
          className={cn(
            "w-full border-0 bg-transparent text-fg outline-none placeholder:text-subtle disabled:cursor-not-allowed disabled:opacity-50",
            className
          )}
          {...props}
        />
      </div>
    </div>
  );
}

function CommandList({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.List>) {
  return (
    <CommandPrimitive.List
      data-slot="command-list"
      className={cn(
        "no-scrollbar max-h-72 scroll-py-1 overflow-x-hidden overflow-y-auto outline-none",
        className
      )}
      {...props}
    />
  );
}

function CommandEmpty({
  className,
  ...props
}: React.ComponentProps<typeof CommandPrimitive.Empty>) {
  return (
    <CommandPrimitive.Empty
      data-slot="command-empty"
      className={cn("py-6 text-center text-small-body text-muted", className)}
      {...props}
    />
  );
}

function CommandGroup({
  className,
  ...props
}: React.ComponentProps<typeof CommandPrimitive.Group>) {
  return (
    <CommandPrimitive.Group
      data-slot="command-group"
      className={cn(
        "overflow-hidden p-1 text-fg **:[[cmdk-group-heading]]:eyebrow **:[[cmdk-group-heading]]:px-2 **:[[cmdk-group-heading]]:py-1.5 **:[[cmdk-group-heading]]:text-muted",
        className
      )}
      {...props}
    />
  );
}

function CommandSeparator({
  className,
  ...props
}: React.ComponentProps<typeof CommandPrimitive.Separator>) {
  return (
    <CommandPrimitive.Separator
      data-slot="command-separator"
      className={cn("mx-0.5 h-px bg-line-soft", className)}
      {...props}
    />
  );
}

function CommandItem({
  className,
  children,
  ...props
}: React.ComponentProps<typeof CommandPrimitive.Item>) {
  return (
    <CommandPrimitive.Item
      data-slot="command-item"
      className={cn(
        "group/command-item relative flex w-full min-w-0 cursor-default items-center min-h-8 gap-2 rounded-md px-2.5 py-1.5 text-small-body text-fg outline-hidden select-none data-[disabled=true]:pointer-events-none data-[disabled=true]:opacity-50 data-selected:bg-surface-2 data-selected:text-fg [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4 data-selected:*:[svg]:text-fg",
        className
      )}
      {...props}
    >
      {children}
      <CheckIcon
        aria-hidden="true"
        className="ms-auto hidden group-data-[checked=true]/command-item:block group-has-data-[slot=command-shortcut]/command-item:hidden"
      />
    </CommandPrimitive.Item>
  );
}

/**
 * Trailing layout slot for a row's shortcut. Pass `Kbd`/`KbdGroup` children —
 * they keep their own cap styling; the key type here only dresses bare text.
 */
function CommandShortcut({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="command-shortcut"
      className={cn(
        "ms-auto inline-flex shrink-0 items-center gap-1 font-keys text-kbd tracking-kbd text-subtle",
        className
      )}
      {...props}
    />
  );
}

export {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  CommandShortcut,
};

import * as React from "react";
import { ChevronsUpDown, XIcon } from "lucide-react";

import { cn } from "../../lib/utils";
import { Command, CommandGroup, CommandInput } from "../command";
import { Popover, PopoverContent, PopoverTrigger } from "../popover";

type CommandSelectProps = React.ComponentProps<typeof Popover>;

interface CommandSelectTriggerProps extends Omit<
  React.ComponentProps<typeof PopoverTrigger>,
  "children"
> {
  children?: React.ReactNode;
  icon?: React.ReactNode;
  label?: React.ReactNode;
  placeholder?: React.ReactNode;
  selected?: boolean;
}

type CommandSelectShellProps = React.ComponentProps<typeof PopoverContent> & {
  commandProps?: React.ComponentProps<typeof Command>;
  inputProps?: React.ComponentProps<typeof CommandInput> & {
    "data-testid"?: string;
  };
  inputPlaceholder?: string;
};

type CommandSelectGroupProps = React.ComponentProps<typeof CommandGroup>;
type CommandSelectChipStripProps = React.ComponentProps<"div">;
type CommandSelectChipProps = React.ComponentProps<"button"> & {
  onRemove?: () => void;
};

function CommandSelect(props: CommandSelectProps) {
  return <Popover {...props} />;
}

function CommandSelectTrigger({
  children,
  className,
  icon,
  label,
  placeholder,
  selected,
  type = "button",
  ...props
}: CommandSelectTriggerProps) {
  const content = children ?? (
    <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
      {icon ? <span className="shrink-0 text-subtle">{icon}</span> : null}
      <span className={cn("truncate text-small-body", selected ? "text-fg" : "text-subtle")}>
        {label ?? placeholder}
      </span>
    </span>
  );

  return (
    <PopoverTrigger
      data-slot="command-select-trigger"
      type={type}
      className={cn(
        "flex h-input w-full items-center justify-between gap-2 rounded-md border border-line bg-elevated px-3 py-2 text-small-body text-fg transition-colors outline-none hover:bg-btn-default-hover disabled:cursor-not-allowed disabled:opacity-50 focus-visible:outline-none focus-visible:border-line-strong focus-visible:shadow-focus-ring",
        className
      )}
      {...props}
    >
      {content}
      <ChevronsUpDown aria-hidden="true" className="size-4 shrink-0 text-subtle" />
    </PopoverTrigger>
  );
}

function CommandSelectShell({
  children,
  className,
  commandProps,
  inputProps,
  inputPlaceholder = "Search...",
  align = "start",
  ...props
}: CommandSelectShellProps) {
  return (
    <PopoverContent
      data-slot="command-select-shell"
      align={align}
      className={cn("w-(--anchor-width) min-w-64 p-0", className)}
      {...props}
    >
      <Command {...commandProps}>
        <CommandInput placeholder={inputPlaceholder} {...inputProps} />
        {children}
      </Command>
    </PopoverContent>
  );
}

function CommandSelectGroup(props: CommandSelectGroupProps) {
  return <CommandGroup data-slot="command-select-group" {...props} />;
}

function CommandSelectChipStrip({ className, ...props }: CommandSelectChipStripProps) {
  return (
    <div
      data-slot="command-select-chip-strip"
      className={cn("flex min-w-0 flex-wrap items-center gap-1.5", className)}
      {...props}
    />
  );
}

function CommandSelectChip({
  children,
  className,
  onClick,
  onRemove,
  type = "button",
  ...props
}: CommandSelectChipProps) {
  return (
    <button
      data-slot="command-select-chip"
      type={type}
      className={cn(
        "eyebrow inline-flex max-w-full items-center gap-1 rounded-sm border border-line bg-canvas-soft px-1.5 py-0.5 text-muted",
        className
      )}
      onClick={event => {
        onClick?.(event);
        if (!event.defaultPrevented) onRemove?.();
      }}
      {...props}
    >
      <span className="truncate">{children}</span>
      {onRemove ? <XIcon aria-hidden="true" className="size-3 shrink-0" /> : null}
    </button>
  );
}

export {
  CommandSelect,
  CommandSelectTrigger,
  CommandSelectShell,
  CommandSelectGroup,
  CommandSelectChipStrip,
  CommandSelectChip,
};
export type {
  CommandSelectProps,
  CommandSelectTriggerProps,
  CommandSelectShellProps,
  CommandSelectGroupProps,
  CommandSelectChipStripProps,
  CommandSelectChipProps,
};

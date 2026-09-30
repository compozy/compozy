import { cva, type VariantProps } from "class-variance-authority";
import * as React from "react";

import { cn } from "../lib/utils";
import { Button } from "./button";
import { Input } from "./input";
import { Textarea } from "./textarea";

export type InputGroupVariant = "default" | "composer";

export interface InputGroupProps extends React.ComponentProps<"div"> {
  /**
   * `composer` is the chat composer card: a `canvas` surface with `shadow-card`
   * that lifts to `shadow-elevated` + a `line-strong` hairline on focus, a
   * borderless textarea, and a block-end tool row (`tool` pills + round `send`).
   */
  variant?: InputGroupVariant;
}

const INPUT_GROUP_COMPOSER_CLASS =
  "h-auto flex-col items-stretch rounded-lg border-0 bg-canvas px-3 pt-3 pb-2.5 shadow-card transition-shadow focus-within:shadow-elevated focus-within:ring-1 focus-within:ring-line-strong";

function InputGroup({ className, variant = "default", ...props }: InputGroupProps) {
  return (
    <div
      data-slot="input-group"
      data-variant={variant}
      role="group"
      className={cn(
        "group/input-group relative flex h-9 w-full min-w-0 items-center rounded-md border border-line bg-canvas text-fg hover:border-line-strong transition-colors outline-none in-data-[slot=combobox-content]:focus-within:border-inherit in-data-[slot=combobox-content]:focus-within:ring-0 has-disabled:opacity-50 has-[[data-slot=input-group-control]:focus-visible]:border-line-strong has-[[data-slot][aria-invalid=true]]:border-danger has-[>[data-align=block-end]]:h-auto has-[>[data-align=block-end]]:flex-col has-[>[data-align=block-start]]:h-auto has-[>[data-align=block-start]]:flex-col has-[>textarea]:h-auto has-[>[data-align=block-end]]:[&>input]:pt-3 has-[>[data-align=block-start]]:[&>input]:pb-3 has-[>[data-align=inline-end]]:[&>input]:pr-1.5 has-[>[data-align=inline-start]]:[&>input]:pl-1.5",
        variant === "composer" && INPUT_GROUP_COMPOSER_CLASS,
        className
      )}
      {...props}
    />
  );
}

const inputGroupAddonVariants = cva(
  "flex h-auto cursor-text items-center justify-center gap-2 py-1.5 text-small-body text-muted select-none group-data-[disabled=true]/input-group:opacity-50 [&>kbd]:rounded-xs [&>svg:not([class*='size-'])]:size-4",
  {
    variants: {
      align: {
        "inline-start": "order-first pl-2 has-[>button]:-ml-1.5 has-[>kbd]:-ml-0.5",
        "inline-end": "order-last pr-2 has-[>button]:-mr-1.5 has-[>kbd]:-mr-0.5",
        "block-start":
          "order-first w-full justify-start px-2.5 pt-2 group-has-[>input]/input-group:pt-2 [.border-b]:pb-2",
        "block-end":
          "order-last w-full justify-start px-2.5 pb-2 group-has-[>input]/input-group:pb-2 [.border-t]:pt-2 group-data-[variant=composer]/input-group:gap-1 group-data-[variant=composer]/input-group:px-0 group-data-[variant=composer]/input-group:pt-2 group-data-[variant=composer]/input-group:pb-0",
      },
    },
    defaultVariants: {
      align: "inline-start",
    },
  }
);

function InputGroupAddon({
  className,
  align = "inline-start",
  ...props
}: React.ComponentProps<"div"> & VariantProps<typeof inputGroupAddonVariants>) {
  return (
    <div
      role="group"
      data-slot="input-group-addon"
      data-align={align}
      className={cn(inputGroupAddonVariants({ align }), className)}
      onMouseDown={e => {
        if ((e.target as HTMLElement).closest("button")) {
          return;
        }
        e.preventDefault();
        e.currentTarget.parentElement
          ?.querySelector<HTMLElement>("[data-slot=input-group-control]")
          ?.focus();
      }}
      {...props}
    />
  );
}

// Geometry is `!`: Button's own `h-button-*` tokens are unknown to tailwind-merge,
// so a plain height here would not replace them.
const inputGroupButtonVariants = cva("flex items-center gap-2 text-sm", {
  variants: {
    size: {
      xs: "h-6! gap-1 rounded-xxs px-1.5 [&>svg:not([class*='size-'])]:size-3",
      sm: "",
      "icon-xs": "size-6! rounded-xxs p-0 has-[>svg]:p-0",
      "icon-sm": "size-8! p-0 has-[>svg]:p-0",
      /** Composer tool pill (attach, agent, mode): quiet until hovered. */
      tool: "h-7! gap-1.5 px-2.5 text-small-body text-muted hover:text-fg [&>svg:not([class*='size-'])]:size-4",
      /** Composer send: a 30 px round inverted button at the row end. */
      send: "ml-auto size-7.5! p-0 has-[>svg]:p-0 [&>svg:not([class*='size-'])]:size-4",
    },
  },
  defaultVariants: {
    size: "xs",
  },
});

function InputGroupButton({
  className,
  type = "button",
  size = "xs",
  variant = size === "send" ? "primary" : "ghost",
  ...props
}: Omit<React.ComponentProps<typeof Button>, "size" | "type"> &
  VariantProps<typeof inputGroupButtonVariants> & {
    type?: "button" | "submit" | "reset";
  }) {
  return (
    <Button
      type={type}
      data-size={size}
      variant={variant}
      className={cn(inputGroupButtonVariants({ size }), className)}
      {...props}
    />
  );
}

function InputGroupText({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      className={cn(
        "flex items-center gap-2 text-small-body text-muted [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4",
        className
      )}
      {...props}
    />
  );
}

function InputGroupInput({ className, ...props }: React.ComponentProps<"input">) {
  return (
    <Input
      data-slot="input-group-control"
      className={cn(
        "flex-1 rounded-none border-0 bg-transparent ring-0 focus-visible:ring-0 disabled:bg-transparent aria-invalid:ring-0 dark:bg-transparent dark:disabled:bg-transparent",
        className
      )}
      {...props}
    />
  );
}

function InputGroupTextarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <Textarea
      data-slot="input-group-control"
      className={cn(
        "flex-1 resize-none rounded-none border-0 bg-transparent py-2 ring-0 focus-visible:ring-0 disabled:bg-transparent aria-invalid:ring-0 dark:bg-transparent dark:disabled:bg-transparent",
        "group-data-[variant=composer]/input-group:min-h-0 group-data-[variant=composer]/input-group:px-0 group-data-[variant=composer]/input-group:py-0 group-data-[variant=composer]/input-group:text-body group-data-[variant=composer]/input-group:focus-visible:shadow-none group-data-[variant=composer]/input-group:placeholder:text-muted",
        className
      )}
      {...props}
    />
  );
}

export {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
  InputGroupText,
  InputGroupTextarea,
};

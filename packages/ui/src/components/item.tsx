import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";
import { cva, type VariantProps } from "class-variance-authority";
import * as React from "react";

import { cn } from "../lib/utils";
import { Separator } from "./separator";

function ItemGroup({ className, children, ...props }: React.ComponentProps<"ul">) {
  return (
    <ul
      data-slot="item-group"
      className={cn(
        "group/item-group flex w-full flex-col gap-4 has-data-[size=sm]:gap-2.5 has-data-[size=xs]:gap-2",
        className
      )}
      {...props}
    >
      {React.Children.map(children, child => (
        <li className="list-none">{child}</li>
      ))}
    </ul>
  );
}

function ItemSeparator({ className, ...props }: React.ComponentProps<typeof Separator>) {
  return (
    <Separator
      data-slot="item-separator"
      orientation="horizontal"
      className={cn("my-2", className)}
      {...props}
    />
  );
}

const itemVariants = cva(
  "group/item flex w-full flex-wrap items-center rounded-lg border text-small-body text-fg transition-colors duration-base outline-none focus-visible:outline-none focus-visible:shadow-focus-ring [a]:transition-colors [a]:hover:bg-hover",
  {
    variants: {
      variant: {
        default: "border-transparent",
        outline: "border-line",
        muted: "border-transparent bg-canvas-tint",
      },
      selectable: {
        true: "relative text-left hover:bg-hover",
        false: "",
      },
      selected: {
        true: "bg-elevated text-fg-strong",
        false: "",
      },
      size: {
        default: "gap-2.5 px-3 py-2.5",
        sm: "gap-2.5 px-3 py-2.5",
        xs: "gap-2 px-2.5 py-2 in-data-[slot=dropdown-menu-content]:p-0",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
      selectable: false,
      selected: false,
    },
  }
);

type ItemIndicator = "rail" | "dot" | "none";
type ItemIndicatorTone = "white" | "accent";
type ItemAs = "div" | "button";

interface ItemOwnProps extends VariantProps<typeof itemVariants> {
  as?: ItemAs;
  disabled?: boolean;
  indicator?: ItemIndicator;
  indicatorTone?: ItemIndicatorTone;
}

type ItemDivProps = ItemOwnProps & Omit<useRender.ComponentProps<"div">, keyof ItemOwnProps>;
type ItemButtonProps = Omit<ItemOwnProps, "as"> &
  Omit<useRender.ComponentProps<"button">, keyof ItemOwnProps> & {
    as: "button";
  };
type ItemProps<As extends ItemAs = "div"> = As extends "button" ? ItemButtonProps : ItemDivProps;
interface ItemDataProps {
  "data-selected"?: string;
}

function isButtonItemProps(props: ItemButtonProps | ItemDivProps): props is ItemButtonProps {
  return props.as === "button";
}

function getItemClassName({
  className,
  variant,
  size,
  selectable,
  selected,
}: {
  className?: string;
  variant?: VariantProps<typeof itemVariants>["variant"];
  size?: VariantProps<typeof itemVariants>["size"];
  selectable: boolean;
  selected: boolean;
}) {
  return cn(
    itemVariants({
      variant,
      size,
      selectable,
      selected,
      className,
    })
  );
}

function getItemChildren(
  children: React.ReactNode,
  indicator: ItemIndicator,
  tone: ItemIndicatorTone
) {
  return (
    <>
      {indicator !== "none" ? <ItemSelectionIndicator kind={indicator} tone={tone} /> : null}
      {children}
    </>
  );
}

function Item(props: ItemButtonProps): React.ReactElement;
function Item(props: ItemDivProps): React.ReactElement;
function Item(props: ItemButtonProps | ItemDivProps) {
  if (isButtonItemProps(props)) {
    return <ButtonItem {...props} />;
  }

  return <DivItem {...props} />;
}

function ButtonItem(props: ItemButtonProps) {
  const indicator = props.indicator ?? "none";
  const indicatorTone: ItemIndicatorTone = props.indicatorTone ?? "white";
  const selectedState = Boolean(props.selected);
  const selectableState = Boolean(props.selectable || selectedState || indicator !== "none");
  const itemChildren = getItemChildren(props.children, indicator, indicatorTone);

  const {
    as: _as,
    className,
    indicator: _indicator,
    indicatorTone: _indicatorTone,
    variant = "default",
    size = "default",
    selected: _selected = false,
    selectable: _selectable = false,
    render,
    children: _children,
    disabled,
    ...buttonProps
  } = props;

  const mergedButtonProps: useRender.ComponentProps<"button"> & ItemDataProps = {
    className: getItemClassName({
      className,
      variant,
      size,
      selectable: selectableState,
      selected: selectedState,
    }),
    children: itemChildren,
    "aria-pressed": selectableState ? selectedState : undefined,
    "data-selected": selectedState ? "true" : undefined,
    disabled,
    type: "button",
  };

  return useRender({
    defaultTagName: "button",
    props: mergeProps<"button">(mergedButtonProps, buttonProps),
    render,
    state: {
      slot: "item",
      variant,
      size,
      selected: selectedState,
      selectable: selectableState,
    },
  });
}

function DivItem(props: ItemDivProps) {
  const indicator = props.indicator ?? "none";
  const indicatorTone: ItemIndicatorTone = props.indicatorTone ?? "white";
  const selectedState = Boolean(props.selected);
  const selectableState = Boolean(props.selectable || selectedState || indicator !== "none");
  const itemChildren = getItemChildren(props.children, indicator, indicatorTone);

  const {
    as: _as,
    className,
    indicator: _indicator,
    indicatorTone: _indicatorTone,
    variant = "default",
    size = "default",
    selected: _selected = false,
    selectable: _selectable = false,
    render,
    children: _children,
    disabled: _disabled,
    ...divProps
  } = props;

  const mergedDivProps: useRender.ComponentProps<"div"> & ItemDataProps = {
    className: getItemClassName({
      className,
      variant,
      size,
      selectable: selectableState,
      selected: selectedState,
    }),
    children: itemChildren,
    "data-selected": selectedState ? "true" : undefined,
  };

  return useRender({
    defaultTagName: "div",
    props: mergeProps<"div">(mergedDivProps, divProps),
    render,
    state: {
      slot: "item",
      variant,
      size,
      selected: selectedState,
      selectable: selectableState,
    },
  });
}

interface ItemSelectionIndicatorProps extends React.ComponentProps<"span"> {
  kind?: ItemIndicator;
  tone?: ItemIndicatorTone;
}

function ItemSelectionIndicator({
  className,
  kind = "rail",
  tone = "white",
  ...props
}: ItemSelectionIndicatorProps) {
  if (kind === "none") return null;

  const toneClass = tone === "accent" ? "bg-accent" : "bg-fg-strong";

  return (
    <span
      aria-hidden="true"
      data-slot="item-selection-indicator"
      data-indicator={kind}
      data-tone={tone}
      className={cn(
        kind === "rail"
          ? "absolute top-2 bottom-2 left-0 w-0.5 rounded-r"
          : "size-1.5 shrink-0 rounded-full",
        toneClass,
        className
      )}
      {...props}
    />
  );
}

const itemMediaVariants = cva(
  "flex shrink-0 items-center justify-center gap-2 group-has-data-[slot=item-description]/item:translate-y-0.5 group-has-data-[slot=item-description]/item:self-start [&_svg]:pointer-events-none",
  {
    variants: {
      variant: {
        default: "bg-transparent",
        icon: "[&_svg:not([class*='size-'])]:size-4",
        image:
          "size-10 overflow-hidden rounded-md group-data-[size=sm]/item:size-8 group-data-[size=xs]/item:size-6 [&_img]:size-full [&_img]:object-cover",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
);

function ItemMedia({
  className,
  variant = "default",
  ...props
}: React.ComponentProps<"div"> & VariantProps<typeof itemMediaVariants>) {
  return (
    <div
      data-slot="item-media"
      data-variant={variant}
      className={cn(itemMediaVariants({ variant, className }))}
      {...props}
    />
  );
}

function ItemContent({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="item-content"
      className={cn(
        "flex flex-1 flex-col gap-1 group-data-[size=xs]/item:gap-0 [&+[data-slot=item-content]]:flex-none",
        className
      )}
      {...props}
    />
  );
}

function ItemTitle({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="item-title"
      className={cn(
        "line-clamp-1 flex w-fit items-center gap-2 text-card-title leading-snug font-medium text-fg-strong underline-offset-4",
        className
      )}
      {...props}
    />
  );
}

function ItemDescription({ className, ...props }: React.ComponentProps<"p">) {
  return (
    <p
      data-slot="item-description"
      className={cn(
        "line-clamp-2 text-left text-small-body leading-normal font-normal text-muted group-data-[size=xs]/item:text-form-label [&>a]:underline [&>a]:underline-offset-4 [&>a:hover]:text-accent",
        className
      )}
      {...props}
    />
  );
}

function ItemActions({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="item-actions" className={cn("flex items-center gap-2", className)} {...props} />
  );
}

function ItemHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="item-header"
      className={cn("flex basis-full items-center justify-between gap-2", className)}
      {...props}
    />
  );
}

function ItemFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="item-footer"
      className={cn("flex basis-full items-center justify-between gap-2", className)}
      {...props}
    />
  );
}

export {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemFooter,
  ItemGroup,
  ItemHeader,
  ItemMedia,
  ItemSelectionIndicator,
  ItemSeparator,
  ItemTitle,
};
export type { ItemAs, ItemIndicator, ItemIndicatorTone, ItemProps, ItemSelectionIndicatorProps };

import * as React from "react";

import { cn } from "../../lib/utils";
import { Pill } from "./pill";

type DataAttributes = {
  [key: `data-${string}`]: string | number | boolean | undefined;
};

interface ListGroupProps extends React.ComponentProps<"div"> {
  label?: React.ReactNode;
  count?: React.ReactNode;
  actions?: React.ReactNode;
  headerProps?: Omit<ListGroupHeaderProps, "label" | "count" | "actions"> & DataAttributes;
  itemsProps?: ListGroupItemsProps;
}

interface ListGroupHeaderProps extends React.ComponentProps<"div"> {
  label: React.ReactNode;
  count?: React.ReactNode;
  actions?: React.ReactNode;
}

type ListGroupItemsProps = React.ComponentProps<"div">;

function hasContent(content: React.ReactNode): boolean {
  return content !== undefined && content !== null && content !== false;
}

function ListGroupRoot({
  label,
  count,
  actions,
  headerProps,
  itemsProps,
  className,
  children,
  ...props
}: ListGroupProps) {
  const hasHeader = hasContent(label) || hasContent(count) || hasContent(actions);

  return (
    <div data-slot="list-group" className={cn("flex min-w-0 flex-col", className)} {...props}>
      {hasHeader ? (
        <ListGroupHeader label={label} count={count} actions={actions} {...headerProps} />
      ) : null}
      <ListGroupItems {...itemsProps}>{children}</ListGroupItems>
    </div>
  );
}

function ListGroupHeader({ label, count, actions, className, ...props }: ListGroupHeaderProps) {
  return (
    <div
      data-slot="list-group-header"
      className={cn(
        "flex items-center justify-between gap-2 border-b border-line bg-canvas-soft px-4 py-2",
        className
      )}
      {...props}
    >
      <span data-slot="list-group-label" className="eyebrow text-muted">
        {label}
      </span>
      <div data-slot="list-group-header-actions" className="flex shrink-0 items-center gap-2">
        {hasContent(count) ? <Pill mono>{count}</Pill> : null}
        {actions}
      </div>
    </div>
  );
}

function ListGroupItems({ className, ...props }: ListGroupItemsProps) {
  return (
    <div
      data-slot="list-group-items"
      className={cn("flex min-w-0 flex-col", className)}
      {...props}
    />
  );
}

const ListGroup = Object.assign(ListGroupRoot, {
  Root: ListGroupRoot,
  Header: ListGroupHeader,
  Items: ListGroupItems,
});

export { ListGroup, ListGroupRoot, ListGroupHeader, ListGroupItems };
export type { ListGroupProps, ListGroupHeaderProps, ListGroupItemsProps };

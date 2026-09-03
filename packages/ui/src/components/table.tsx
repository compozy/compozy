import type * as React from "react";

import { cn } from "../lib/utils";

export type TableOverflowX = "auto" | "hidden";

export interface TableProps extends React.ComponentProps<"table"> {
  /** Sideways scroll is the default. Fixed layouts that already truncate pass `hidden`. */
  overflowX?: TableOverflowX;
}

function Table({ className, overflowX = "auto", ...props }: TableProps) {
  return (
    <div
      className={cn(
        "relative w-full",
        overflowX === "hidden" ? "overflow-x-hidden" : "overflow-x-auto"
      )}
      data-overflow-x={overflowX}
      data-slot="table-container"
    >
      <table
        data-slot="table"
        className={cn("w-full caption-bottom text-small-body text-fg", className)}
        {...props}
      />
    </div>
  );
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("[&_tr]:border-b [&_tr]:border-line", className)}
      {...props}
    />
  );
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  );
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        "border-t border-line bg-canvas-tint text-small-body text-fg font-medium [&>tr]:last:border-b-0",
        className
      )}
      {...props}
    />
  );
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "border-b border-line transition-colors hover:bg-hover has-aria-expanded:bg-hover data-[state=selected]:bg-elevated",
        className
      )}
      {...props}
    />
  );
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "eyebrow h-9 px-3 text-left align-middle whitespace-nowrap text-muted has-[[role=checkbox]]:pr-0",
        className
      )}
      {...props}
    />
  );
}

function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn(
        "px-3 py-2.5 align-middle whitespace-nowrap text-fg has-[[role=checkbox]]:pr-0",
        className
      )}
      {...props}
    />
  );
}

function TableCaption({ className, ...props }: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-small-body text-muted", className)}
      {...props}
    />
  );
}

export { Table, TableBody, TableCaption, TableCell, TableFooter, TableHead, TableHeader, TableRow };

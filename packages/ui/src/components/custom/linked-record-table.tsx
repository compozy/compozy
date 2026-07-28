import * as React from "react";

import { cn } from "../../lib/utils";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../table";
import { Section, type SectionProps } from "./section";

interface LinkedRecordTableProps extends Omit<SectionProps, "children" | "label"> {
  label?: React.ReactNode;
  columns?: readonly string[];
  empty?: React.ReactNode;
  children?: React.ReactNode;
}

type LinkedRecordTableTitleProps = React.ComponentProps<"div">;
type LinkedRecordTableBodyProps = React.ComponentProps<typeof TableBody>;
type LinkedRecordTableRowProps = React.ComponentProps<typeof TableRow>;
type LinkedRecordTableCellProps = React.ComponentProps<typeof TableCell>;
type LinkedRecordTableOpenCellProps = React.ComponentProps<typeof TableCell>;

function LinkedRecordTableRoot({
  label,
  columns = ["Title", "Owner", "Updated"],
  empty,
  className,
  bodyClassName,
  children,
  ...props
}: LinkedRecordTableProps) {
  return (
    <Section
      data-slot="linked-record-table"
      label={label}
      className={cn("w-full", className)}
      bodyClassName={cn("overflow-hidden", bodyClassName)}
      {...props}
    >
      {children ? (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-8" />
              {columns.map(column => (
                <TableHead key={column}>{column}</TableHead>
              ))}
              <TableHead className="w-8" />
            </TableRow>
          </TableHeader>
          {children}
        </Table>
      ) : (
        empty
      )}
    </Section>
  );
}

function LinkedRecordTableTitle({ className, ...props }: LinkedRecordTableTitleProps) {
  return (
    <div
      data-slot="linked-record-table-title"
      className={cn("flex min-w-0 flex-col gap-1", className)}
      {...props}
    />
  );
}

function LinkedRecordTableBody({ className, ...props }: LinkedRecordTableBodyProps) {
  return <TableBody data-slot="linked-record-table-body" className={className} {...props} />;
}

function LinkedRecordTableRow({ className, ...props }: LinkedRecordTableRowProps) {
  return <TableRow data-slot="linked-record-table-row" className={className} {...props} />;
}

function LinkedRecordTableCell({ className, ...props }: LinkedRecordTableCellProps) {
  return <TableCell className={className} {...props} />;
}

function LinkedRecordTableOpenCell({ className, ...props }: LinkedRecordTableOpenCellProps) {
  return (
    <TableCell
      data-slot="linked-record-table-open-cell"
      className={cn("w-8 pr-4", className)}
      {...props}
    />
  );
}

const LinkedRecordTable = Object.assign(LinkedRecordTableRoot, {
  Body: LinkedRecordTableBody,
  Cell: LinkedRecordTableCell,
  OpenCell: LinkedRecordTableOpenCell,
  Row: LinkedRecordTableRow,
  Title: LinkedRecordTableTitle,
});

export {
  LinkedRecordTable,
  LinkedRecordTableBody,
  LinkedRecordTableCell,
  LinkedRecordTableOpenCell,
  LinkedRecordTableRoot,
  LinkedRecordTableRow,
  LinkedRecordTableTitle,
};
export type {
  LinkedRecordTableBodyProps,
  LinkedRecordTableCellProps,
  LinkedRecordTableOpenCellProps,
  LinkedRecordTableProps,
  LinkedRecordTableRowProps,
  LinkedRecordTableTitleProps,
};

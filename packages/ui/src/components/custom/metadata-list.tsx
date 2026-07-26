import * as React from "react";

import { cn } from "../../lib/utils";

interface MetadataListProps extends React.ComponentProps<"dl"> {}
interface MetadataListTermProps extends React.ComponentProps<"dt"> {}
interface MetadataListValueProps extends React.ComponentProps<"dd"> {}
interface DataAttributes {
  [key: `data-${string}`]: string | number | boolean | undefined;
}
interface MetadataListRowProps extends React.ComponentProps<"div"> {
  label?: React.ReactNode;
  termProps?: MetadataListTermProps & DataAttributes;
  valueProps?: MetadataListValueProps & DataAttributes;
}

function MetadataListRoot({ className, ...props }: MetadataListProps) {
  return (
    <dl
      data-slot="metadata-list"
      className={cn("grid grid-cols-1 gap-x-3 gap-y-1.5", className)}
      {...props}
    />
  );
}

function MetadataListRow({
  label,
  termProps,
  valueProps,
  className,
  children,
  ...props
}: MetadataListRowProps) {
  if (label !== undefined && label !== null && label !== false) {
    const { className: termClassName, ...restTermProps } = termProps ?? {};
    const { className: valueClassName, ...restValueProps } = valueProps ?? {};
    return (
      <div
        data-slot="metadata-list-row"
        className={cn("grid grid-cols-[var(--width-kv-label)_1fr] items-center gap-3", className)}
        {...props}
      >
        <MetadataListTerm className={termClassName} {...restTermProps}>
          {label}
        </MetadataListTerm>
        <MetadataListValue className={valueClassName} {...restValueProps}>
          {children}
        </MetadataListValue>
      </div>
    );
  }

  return (
    <div
      data-slot="metadata-list-row"
      className={cn("flex min-w-0 items-center gap-1.5", className)}
      {...props}
    >
      {children}
    </div>
  );
}

function MetadataListTerm({ className, ...props }: MetadataListTermProps) {
  return (
    <dt
      data-slot="metadata-list-term"
      className={cn("eyebrow shrink-0 text-subtle", className)}
      {...props}
    />
  );
}

function MetadataListValue({ className, ...props }: MetadataListValueProps) {
  return (
    <dd
      data-slot="metadata-list-value"
      className={cn("min-w-0 text-small-body text-muted", className)}
      {...props}
    />
  );
}

const MetadataList = Object.assign(MetadataListRoot, {
  Row: MetadataListRow,
  Term: MetadataListTerm,
  Value: MetadataListValue,
});

export { MetadataList, MetadataListRoot, MetadataListRow, MetadataListTerm, MetadataListValue };
export type {
  MetadataListProps,
  MetadataListRowProps,
  MetadataListTermProps,
  MetadataListValueProps,
};

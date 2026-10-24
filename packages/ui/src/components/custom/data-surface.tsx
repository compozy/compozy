"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { Empty, type EmptyProps } from "../empty";
import { SkeletonRegion } from "../skeleton";
import { BlockLoading, type BlockLoadingProps } from "./block-loading";
import type { DataSurfaceState } from "./data-surface-state";

interface DataSurfaceProps extends React.ComponentProps<"div"> {
  state: DataSurfaceState;
}

type DataSurfaceSlotProps = React.ComponentProps<"div"> & {
  "data-surface-state"?: DataSurfaceState;
};

type DataSurfaceLoadingProps = BlockLoadingProps;
type DataSurfaceEmptyProps = EmptyProps;
type DataSurfaceErrorProps = EmptyProps;
type DataSurfaceContentProps = React.ComponentProps<"div">;

function isStateElement(child: React.ReactNode): child is React.ReactElement<DataSurfaceSlotProps> {
  return React.isValidElement(child) && getStateElementState(child) !== undefined;
}

function getStateElementState(child: React.ReactElement): DataSurfaceState | undefined {
  const props = child.props as Partial<DataSurfaceSlotProps>;
  if (props["data-surface-state"] !== undefined) return props["data-surface-state"];
  if (child.type === DataSurfaceLoading) return "loading";
  if (child.type === DataSurfaceError) return "error";
  if (child.type === DataSurfaceEmpty) return "empty";
  if (child.type === DataSurfaceContent) return "ready";
  return undefined;
}

function DataSurfaceRoot({ state, children, className, ...props }: DataSurfaceProps) {
  const selectedChild = React.Children.toArray(children).find(
    child => isStateElement(child) && getStateElementState(child) === state
  );

  return (
    <div
      data-slot="data-surface"
      data-state={state}
      className={cn("min-w-0", className)}
      {...props}
    >
      {selectedChild}
    </div>
  );
}

/**
 * The loading slot. Pass a content-shaped skeleton as `children` (`ListingRowSkeleton`,
 * `CatalogCardSkeleton`, a `Table` with `TableRowSkeleton`, …): it renders inside a
 * `SkeletonRegion`, which carries the status role, the busy flag and the visually hidden `label`,
 * so the skeleton shapes stay presentational. With no children it falls back to the centered
 * `BlockLoading` spinner; that fallback is transitional, content areas should show a skeleton.
 */
function DataSurfaceLoading({
  children,
  label = "Loading",
  size,
  surface,
  showLabel,
  className,
  ...props
}: DataSurfaceLoadingProps) {
  if (React.Children.toArray(children).length === 0) {
    return (
      <BlockLoading
        data-surface-state="loading"
        label={label}
        size={size}
        surface={surface}
        showLabel={showLabel}
        className={className}
        {...props}
      />
    );
  }

  return (
    <SkeletonRegion data-surface-state="loading" label={label} className={className} {...props}>
      {children}
    </SkeletonRegion>
  );
}

function DataSurfaceEmpty(props: DataSurfaceEmptyProps) {
  return <Empty data-surface-state="empty" {...props} />;
}

function DataSurfaceError(props: DataSurfaceErrorProps) {
  return <Empty data-surface-state="error" {...props} />;
}

function DataSurfaceContent({ className, ...props }: DataSurfaceContentProps) {
  return (
    <div
      data-slot="data-surface-content"
      data-surface-state="ready"
      className={cn("min-w-0", className)}
      {...props}
    />
  );
}

const DataSurface = Object.assign(DataSurfaceRoot, {
  Loading: DataSurfaceLoading,
  Empty: DataSurfaceEmpty,
  Error: DataSurfaceError,
  Content: DataSurfaceContent,
});

export { DataSurface, DataSurfaceLoading, DataSurfaceEmpty, DataSurfaceError, DataSurfaceContent };
export type {
  DataSurfaceContentProps,
  DataSurfaceEmptyProps,
  DataSurfaceErrorProps,
  DataSurfaceLoadingProps,
  DataSurfaceProps,
  DataSurfaceState,
};

import * as React from "react";

import { cn } from "../../lib/utils";
import { useStatusCardTone } from "./hooks/use-status-card-tone";
import { Pill, type PillDotProps } from "./pill";

type DataAttributes = {
  [key: `data-${string}`]: string | number | boolean | undefined;
};

export interface StatusCardHeaderProps extends React.ComponentProps<"div"> {
  label?: React.ReactNode;
  dotProps?: Omit<PillDotProps, "tone"> & DataAttributes;
  labelProps?: React.ComponentProps<"span"> & DataAttributes;
}

export type StatusCardBodyProps = React.ComponentProps<"div">;
export type StatusCardFooterProps = React.ComponentProps<"div">;
export type StatusCardActionProps = React.ComponentProps<"div">;

export function StatusCardHeader({
  label,
  dotProps,
  labelProps,
  className,
  children,
  ...props
}: StatusCardHeaderProps) {
  const tone = useStatusCardTone();
  const { className: dotClassName, ...restDotProps } = dotProps ?? {};
  const { className: labelClassName, ...restLabelProps } = labelProps ?? {};

  return (
    <div
      className={cn("flex min-w-0 items-center gap-3", className)}
      data-slot="status-card-header"
      {...props}
    >
      <Pill.Dot
        aria-hidden="true"
        className={dotClassName}
        data-slot="status-card-dot"
        size="md"
        tone={tone}
        {...restDotProps}
      />
      {label ? (
        <span
          className={cn(
            "min-w-0 truncate text-item-title font-medium text-fg-strong",
            labelClassName
          )}
          data-slot="status-card-label"
          {...restLabelProps}
        >
          {label}
        </span>
      ) : null}
      {children}
    </div>
  );
}

export function StatusCardBody({ className, ...props }: StatusCardBodyProps) {
  return (
    <div
      className={cn("text-small-body leading-5 text-muted", className)}
      data-slot="status-card-body"
      {...props}
    />
  );
}

export function StatusCardFooter({ className, ...props }: StatusCardFooterProps) {
  return (
    <div
      className={cn("flex flex-wrap items-center gap-2", className)}
      data-slot="status-card-footer"
      {...props}
    />
  );
}

export function StatusCardAction({ className, ...props }: StatusCardActionProps) {
  return (
    <div
      className={cn("flex flex-wrap items-center gap-2", className)}
      data-slot="status-card-action"
      {...props}
    />
  );
}

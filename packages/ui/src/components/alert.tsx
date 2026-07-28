import type { VariantProps } from "class-variance-authority";
import type * as React from "react";

import { cn } from "../lib/utils";
import { alertVariants } from "./alert-variants";

type AlertProps = React.ComponentProps<"div"> & VariantProps<typeof alertVariants>;

function Alert({ className, variant, role = "alert", ...props }: AlertProps) {
  return (
    <div
      data-slot="alert"
      data-variant={variant ?? "default"}
      role={role}
      className={cn(alertVariants({ variant }), className)}
      {...props}
    />
  );
}

function AlertTitle({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-title"
      className={cn(
        "text-item-title font-medium text-fg-strong",
        "group-has-[>svg]/alert:col-start-2",
        "[&_a]:underline [&_a]:underline-offset-3 [&_a]:hover:text-fg",
        className
      )}
      {...props}
    />
  );
}

function AlertDescription({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-description"
      className={cn(
        "text-form-input leading-normal text-muted text-balance md:text-pretty",
        "group-has-[>svg]/alert:col-start-2",
        "[&_a]:underline [&_a]:underline-offset-3 [&_a]:hover:text-fg",
        "[&_p:not(:last-child)]:mb-2",
        className
      )}
      {...props}
    />
  );
}

function AlertAction({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div data-slot="alert-action" className={cn("absolute top-2 right-2", className)} {...props} />
  );
}

function AlertMeta({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-meta"
      className={cn(
        "flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 text-form-hint text-muted",
        "group-has-[>svg]/alert:col-start-2",
        className
      )}
      {...props}
    />
  );
}

function AlertActions({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="alert-actions"
      className={cn(
        "mt-1 flex flex-wrap items-center justify-end gap-1.5",
        "group-has-[>svg]/alert:col-start-2",
        className
      )}
      {...props}
    />
  );
}

export { Alert, AlertAction, AlertActions, AlertDescription, AlertMeta, AlertTitle };
export type { AlertProps };

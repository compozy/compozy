import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes, ReactElement, ReactNode } from "react";

import { cn } from "../lib/utils";

export const alertVariants = cva(
  "flex items-start gap-3 rounded-[var(--radius-md)] border px-4 py-3 text-sm",
  {
    variants: {
      variant: {
        info: "border-[color:var(--tone-info-border)] bg-[color:var(--tone-info-bg)] text-[color:var(--tone-info-text)]",
        success:
          "border-[color:var(--tone-success-border)] bg-[color:var(--tone-success-bg)] text-[color:var(--tone-success-text)]",
        warning:
          "border-[color:var(--tone-warning-border)] bg-[color:var(--tone-warning-bg)] text-[color:var(--tone-warning-text)]",
        error:
          "border-[color:var(--tone-danger-border)] bg-[color:var(--tone-danger-bg)] text-[color:var(--tone-danger-text)]",
        neutral: "border-border bg-[color:var(--tone-neutral-bg)] text-muted-foreground",
      },
    },
    defaultVariants: {
      variant: "info",
    },
  }
);

export interface AlertProps
  extends Omit<HTMLAttributes<HTMLDivElement>, "title">, VariantProps<typeof alertVariants> {
  title?: ReactNode;
  icon?: ReactNode;
}

export function Alert({
  children,
  className,
  icon,
  role,
  title,
  variant,
  ...props
}: AlertProps): ReactElement {
  const resolvedRole = role ?? (variant === "error" ? "alert" : undefined);
  return (
    <div className={cn(alertVariants({ variant }), className)} role={resolvedRole} {...props}>
      {icon ? (
        <span
          aria-hidden="true"
          className="mt-0.5 flex size-4 shrink-0 items-center justify-center"
        >
          {icon}
        </span>
      ) : null}
      <div className="min-w-0 flex-1 space-y-1">
        {title ? <p className="font-semibold leading-5 text-current">{title}</p> : null}
        {children ? (
          <div className="text-sm leading-5 text-current [&_a]:underline [&_a]:underline-offset-2">
            {children}
          </div>
        ) : null}
      </div>
    </div>
  );
}

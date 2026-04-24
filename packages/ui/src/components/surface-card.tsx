import type { HTMLAttributes, ReactElement } from "react";

import { cn } from "../lib/utils";

export function SurfaceCard({ className, ...props }: HTMLAttributes<HTMLElement>): ReactElement {
  return (
    <section
      className={cn(
        "overflow-hidden rounded-[var(--radius-xl)] border border-border-subtle bg-card text-card-foreground shadow-[var(--shadow-sm)]",
        "transition-[border-color,background-color,box-shadow,transform] duration-200 ease-out",
        "data-[interactive=true]:hover:-translate-y-px data-[interactive=true]:hover:border-border-strong data-[interactive=true]:hover:shadow-[var(--shadow-md)]",
        className
      )}
      {...props}
    />
  );
}

export function SurfaceCardHeader({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>): ReactElement {
  return (
    <div
      className={cn(
        "flex items-start justify-between gap-4 border-b border-border-subtle px-5 py-4",
        className
      )}
      {...props}
    />
  );
}

export function SurfaceCardEyebrow({
  className,
  ...props
}: HTMLAttributes<HTMLParagraphElement>): ReactElement {
  return <p className={cn("mb-1 eyebrow text-muted-foreground", className)} {...props} />;
}

export function SurfaceCardTitle({
  className,
  ...props
}: HTMLAttributes<HTMLHeadingElement>): ReactElement {
  return <h2 className={cn("text-sm font-semibold text-foreground", className)} {...props} />;
}

export function SurfaceCardDescription({
  className,
  ...props
}: HTMLAttributes<HTMLParagraphElement>): ReactElement {
  return <p className={cn("mt-1 text-sm leading-6 text-muted-foreground", className)} {...props} />;
}

export function SurfaceCardBody({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>): ReactElement {
  return <div className={cn("px-5 py-5", className)} {...props} />;
}

export function SurfaceCardFooter({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>): ReactElement {
  return (
    <div
      className={cn(
        "flex items-center justify-between gap-3 border-t border-border-subtle px-5 py-4",
        className
      )}
      {...props}
    />
  );
}

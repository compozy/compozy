import type { ButtonHTMLAttributes, HTMLAttributes, ReactElement, ReactNode } from "react";

import { cn } from "../lib/utils";

export function AppShell({ className, ...props }: HTMLAttributes<HTMLDivElement>): ReactElement {
  return (
    <div
      className={cn(
        "grid min-h-[100dvh] bg-background text-foreground lg:grid-cols-[240px_minmax(0,1fr)]",
        className
      )}
      {...props}
    />
  );
}

export function AppShellSidebar({
  className,
  ...props
}: HTMLAttributes<HTMLElement>): ReactElement {
  return (
    <aside
      className={cn(
        "flex min-h-full flex-col gap-6 border-b border-sidebar-border bg-sidebar px-4 py-5",
        "lg:border-b-0 lg:border-r",
        className
      )}
      {...props}
    />
  );
}

export interface AppShellBrandProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  badge?: ReactNode;
  detail?: ReactNode;
  mark?: ReactNode;
  title: ReactNode;
}

export function AppShellBrand({
  badge,
  className,
  detail,
  mark = "C",
  title,
  ...props
}: AppShellBrandProps): ReactElement {
  return (
    <div className={cn("flex items-start gap-3", className)} {...props}>
      <div className="grid size-10 shrink-0 place-items-center rounded-[calc(var(--radius)-2px)] bg-primary text-primary-foreground font-display text-lg">
        {mark}
      </div>
      <div className="min-w-0 space-y-1">
        <div className="flex items-center gap-2">
          <h2 className="truncate text-sm font-semibold tracking-[-0.01em] text-foreground">
            {title}
          </h2>
          {badge}
        </div>
        {detail ? <p className="eyebrow text-muted-foreground">{detail}</p> : null}
      </div>
    </div>
  );
}

export interface AppShellNavSectionProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  title?: ReactNode;
}

export function AppShellNavSection({
  children,
  className,
  title,
  ...props
}: AppShellNavSectionProps): ReactElement {
  return (
    <div className={cn("space-y-2", className)} {...props}>
      {title ? <p className="px-3 eyebrow text-muted-foreground">{title}</p> : null}
      <div className="flex flex-col gap-1">{children}</div>
    </div>
  );
}

export interface AppShellNavItemProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  active?: boolean;
  badge?: ReactNode;
  icon?: ReactNode;
  label: ReactNode;
}

export function AppShellNavItem({
  active = false,
  badge,
  className,
  icon,
  label,
  type = "button",
  ...props
}: AppShellNavItemProps): ReactElement {
  return (
    <button
      aria-current={active ? "page" : undefined}
      className={cn(
        "flex w-full items-center gap-3 rounded-[calc(var(--radius)-2px)] px-3 py-2 text-left transition-colors",
        active
          ? "bg-sidebar-accent text-sidebar-accent-foreground"
          : "text-muted-foreground hover:bg-accent hover:text-foreground",
        className
      )}
      type={type}
      {...props}
    >
      <span
        aria-hidden="true"
        className={cn(
          "flex size-4 shrink-0 items-center justify-center",
          active ? "text-primary" : "text-muted-foreground"
        )}
      >
        {icon}
      </span>
      <span className="min-w-0 flex-1 truncate text-sm">{label}</span>
      {badge ? <span className="eyebrow text-muted-foreground">{badge}</span> : null}
    </button>
  );
}

export function AppShellMain({ className, ...props }: HTMLAttributes<HTMLElement>): ReactElement {
  return <main className={cn("flex min-w-0 flex-col", className)} {...props} />;
}

export function AppShellHeader({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>): ReactElement {
  return (
    <div
      className={cn("border-b border-border bg-background px-5 py-5 lg:px-6", className)}
      {...props}
    />
  );
}

export function AppShellContent({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>): ReactElement {
  return <div className={cn("flex-1 overflow-auto p-5 lg:p-6", className)} {...props} />;
}

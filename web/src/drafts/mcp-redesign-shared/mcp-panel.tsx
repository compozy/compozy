/**
 * The shell every MCP row panel shares (sign-in, review, not working, add to CompozyOS): an
 * identity well, a title with one quiet subtitle, a hairline, the body, and a ruled footer with
 * the actions. Rendered inside a `PopoverContent` with no padding of its own.
 */
import { X } from "@compozy/ui/icons";
import type { ComponentType, ReactNode } from "react";

import { Button, cn, KindIcon } from "@compozy/ui";

export type MCPPanelTone = "neutral" | "warning" | "destructive" | "success";

const WELL_TONE: Record<MCPPanelTone, string | undefined> = {
  neutral: undefined,
  warning: "bg-warning-tint text-warning",
  destructive: "bg-destructive-tint text-destructive",
  success: "bg-success-tint text-success",
};

export interface MCPPanelProps {
  icon: ComponentType<{ className?: string }>;
  /** A brand mark that replaces the glyph well (e.g. a provider logo). */
  mark?: ReactNode;
  tone?: MCPPanelTone;
  title: ReactNode;
  subtitle?: ReactNode;
  onClose?: () => void;
  children: ReactNode;
  footer?: ReactNode;
  /** Extra ruled section under the body (e.g. the paste-the-address fallback). */
  aside?: ReactNode;
  testId?: string;
}

export function MCPPanel({
  icon,
  mark,
  tone = "neutral",
  title,
  subtitle,
  onClose,
  children,
  footer,
  aside,
  testId,
}: MCPPanelProps) {
  return (
    <div className="flex flex-col" data-testid={testId}>
      <header className="flex items-start gap-3 border-b border-line-soft px-3.5 py-3">
        {mark ?? (
          <KindIcon
            aria-hidden="true"
            icon={icon}
            tone="well"
            size="sm"
            className={cn("shrink-0", WELL_TONE[tone])}
          />
        )}
        <div className="flex min-w-0 flex-1 flex-col gap-0.5 pt-px">
          <p className="truncate text-small-body font-semibold text-fg-strong">{title}</p>
          {subtitle ? <p className="truncate text-meta text-faint">{subtitle}</p> : null}
        </div>
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label="Close"
          className="-mt-0.5 -mr-1 text-faint"
          onClick={onClose}
        >
          <X />
        </Button>
      </header>
      <div className="flex flex-col gap-3 px-3.5 py-3.5">{children}</div>
      {aside ? <div className="border-t border-line-soft px-3.5 py-2.5">{aside}</div> : null}
      {footer ? (
        <footer className="flex items-center gap-1.5 border-t border-line-soft px-3.5 py-2.5">
          {footer}
        </footer>
      ) : null}
    </div>
  );
}

/** A labelled fact inside a panel: small faint label above a value. */
export function MCPPanelFact({
  label,
  icon: Icon,
  children,
}: {
  label: string;
  icon?: ComponentType<{ className?: string }>;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <span className="flex items-center gap-1.5 text-meta text-faint">
        {Icon ? <Icon aria-hidden="true" className="size-3.5" /> : null}
        {label}
      </span>
      {children}
    </div>
  );
}

export function MCPCommandBlock({ children }: { children: ReactNode }) {
  return (
    <code className="block rounded-md bg-sunken px-2.5 py-2 font-mono text-mono-id leading-relaxed break-all text-muted-foreground shadow-hairline-inset">
      {children}
    </code>
  );
}

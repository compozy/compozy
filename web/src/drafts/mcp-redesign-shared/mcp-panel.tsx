/**
 * The shell every MCP row panel shares (sign-in, review, not working, add to CompozyOS), composed
 * from the shared `PopoverPanel`. Render it inside `PopoverPanelContent`. The drafts always show
 * Close, so a panel without a handler still renders one.
 */
import type { ComponentType, ReactNode } from "react";

import { PopoverPanel, type PopoverPanelTone } from "@compozy/ui";

export interface MCPPanelProps {
  icon: ComponentType<{ className?: string }>;
  /** A brand mark that replaces the glyph well (e.g. a provider logo). */
  mark?: ReactNode;
  tone?: PopoverPanelTone;
  title: ReactNode;
  subtitle?: ReactNode;
  onClose?: () => void;
  children: ReactNode;
  footer?: ReactNode;
  /** Extra ruled section under the body (e.g. the paste-the-address fallback). */
  aside?: ReactNode;
  testId?: string;
}

const noop = () => {};

export function MCPPanel({ icon, mark, onClose, testId, ...panel }: MCPPanelProps) {
  return (
    <PopoverPanel
      {...panel}
      {...(mark ? { mark } : { icon })}
      data-testid={testId}
      onClose={onClose ?? noop}
    />
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

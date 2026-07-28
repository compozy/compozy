"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

type IconComponent = React.ComponentType<{ className?: string; size?: number }>;

export interface OperationalLink {
  label: React.ReactNode;
  href: string;
  icon?: IconComponent;
  /** Optional override for `target` (e.g. `_blank`). */
  target?: string;
  /** Optional override for `rel`. Defaults to `noreferrer` when target is `_blank`. */
  rel?: string;
}

export interface OperationalLinksRowProps extends React.ComponentProps<"nav"> {
  items: ReadonlyArray<OperationalLink>;
  ariaLabel?: string;
}

function OperationalLinksRow({
  items,
  ariaLabel = "Operational links",
  className,
  ...props
}: OperationalLinksRowProps) {
  return (
    <nav
      data-slot="operational-links-row"
      aria-label={ariaLabel}
      className={cn(
        "flex flex-wrap items-center gap-1 rounded border border-line bg-canvas-soft px-2 py-1.5",
        className
      )}
      {...props}
    >
      {items.map(item => {
        const Icon = item.icon;
        const externalRel = item.rel ?? (item.target === "_blank" ? "noreferrer" : undefined);
        return (
          <a
            key={item.href}
            data-slot="operational-links-row-link"
            href={item.href}
            target={item.target}
            rel={externalRel}
            className="inline-flex items-center gap-1 rounded-xs px-2 py-1 text-form-label text-muted transition-colors duration-base ease-out hover:bg-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
          >
            {Icon ? <Icon aria-hidden="true" className="size-3" /> : null}
            <span>{item.label}</span>
          </a>
        );
      })}
    </nav>
  );
}

export { OperationalLinksRow };

"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

type IconComponent = React.ComponentType<{ className?: string; size?: number }>;

export interface SectionProps extends React.ComponentProps<"section"> {
  label?: React.ReactNode;
  note?: React.ReactNode;
  right?: React.ReactNode;
  divided?: boolean;
  /**
   * When `true`, the section head renders a bottom hairline via `border-b border-line`.
   * Default is `false` (flat-depth pass) — depth comes from the warm-surface ramp.
   */
  bordered?: boolean;
  bodyClassName?: string;
  headClassName?: string;
  rightClassName?: string;
  count?: number | string;
  icon?: IconComponent;
  tabs?: React.ReactNode;
}

function hasSectionContent(content: React.ReactNode): boolean {
  return content !== undefined && content !== null && content !== false;
}

function SectionLabel({
  label,
  count,
  icon: Icon,
}: Pick<SectionProps, "label" | "count" | "icon">) {
  const hasCount = count !== undefined && count !== null && count !== "";
  return (
    <div className="flex min-w-0 items-center gap-2">
      {Icon ? (
        <span
          aria-hidden="true"
          data-slot="section-icon"
          className="inline-flex size-5 shrink-0 items-center justify-center text-fg-2"
        >
          <Icon className="size-3" />
        </span>
      ) : null}
      <h2 data-slot="section-label" className="truncate text-item-title font-medium text-fg-strong">
        {label}
      </h2>
      {hasCount ? (
        <span
          data-slot="section-count"
          className="inline-flex items-center text-eyebrow font-normal tabular-nums text-subtle"
        >
          {count}
        </span>
      ) : null}
    </div>
  );
}

function SectionRight({
  right,
  tabs,
  rightClassName,
}: Pick<SectionProps, "right" | "tabs" | "rightClassName">) {
  const hasRight = hasSectionContent(right);
  const hasTabs = hasSectionContent(tabs);
  if (!hasRight && !hasTabs) return null;
  return (
    <div
      data-slot="section-right"
      className={cn(
        "flex w-full items-center gap-2 self-start @md/section:w-auto @md/section:shrink-0",
        rightClassName
      )}
    >
      {hasTabs ? <div data-slot="section-tabs">{tabs}</div> : null}
      {hasRight ? right : null}
    </div>
  );
}

function SectionHead({
  label,
  note,
  right,
  tabs,
  count,
  icon,
  bordered,
  headClassName,
  rightClassName,
}: Pick<
  SectionProps,
  | "label"
  | "note"
  | "right"
  | "tabs"
  | "count"
  | "icon"
  | "bordered"
  | "headClassName"
  | "rightClassName"
>) {
  const hasLabel = hasSectionContent(label);
  const hasNote = hasSectionContent(note);
  if (!hasLabel && !hasNote && !hasSectionContent(right) && !hasSectionContent(tabs)) return null;
  return (
    <header
      data-slot="section-head"
      data-bordered={bordered ? "true" : undefined}
      className={cn(
        "flex flex-col gap-3 pb-2 @md/section:flex-row @md/section:items-start @md/section:justify-between",
        bordered && "border-b border-line",
        headClassName
      )}
    >
      <div className="flex min-w-0 flex-col gap-2">
        {hasLabel ? <SectionLabel count={count} icon={icon} label={label} /> : null}
        {hasNote ? (
          <div data-slot="section-note" className="max-w-152 text-small-body text-muted">
            {note}
          </div>
        ) : null}
      </div>
      <SectionRight right={right} rightClassName={rightClassName} tabs={tabs} />
    </header>
  );
}

function Section({
  label,
  note,
  right,
  divided = false,
  bordered = false,
  bodyClassName,
  headClassName,
  rightClassName,
  className,
  children,
  count,
  icon,
  tabs,
  ...props
}: SectionProps) {
  return (
    <section
      data-slot="section"
      // A size container measures no intrinsic width, so inside a row-flex
      // parent it would collapse to 0: the section claims the full line itself.
      className={cn(
        "@container/section flex w-full min-w-0 flex-col gap-3",
        divided && "border-t border-line pt-5 first:border-t-0 first:pt-0",
        className
      )}
      {...props}
    >
      <SectionHead
        bordered={bordered}
        count={count}
        headClassName={headClassName}
        icon={icon}
        label={label}
        note={note}
        right={right}
        rightClassName={rightClassName}
        tabs={tabs}
      />
      {hasSectionContent(children) ? (
        <div data-slot="section-body" className={cn("flex min-w-0 flex-col", bodyClassName)}>
          {children}
        </div>
      ) : null}
    </section>
  );
}

export { Section };

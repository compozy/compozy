import type { ComponentProps, ReactNode } from "react";
import { ChevronRight } from "lucide-react";
import { CollapsibleTrigger, Empty, Eyebrow, cn, type EmptyProps } from "@compozy/ui";

/** One rail section: internal rhythm only; the rail body owns the gap between sections. */
export function SessionInspectorSection({ className, ...props }: ComponentProps<"section">) {
  return <section className={cn("flex min-w-0 flex-col gap-2.25", className)} {...props} />;
}

/** Sentence-case eyebrow with an optional mono meta on the trailing edge ("newest first"). */
export function SessionInspectorSectionHead({
  children,
  meta,
  className,
  ...props
}: ComponentProps<"div"> & {
  meta?: ReactNode;
}) {
  return (
    <div className={cn("flex items-center gap-1.5", className)} {...props}>
      <Eyebrow>{children}</Eyebrow>
      {meta ? (
        <span className="ml-auto truncate font-mono text-mono-id tabular-nums text-faint">
          {meta}
        </span>
      ) : null}
    </div>
  );
}

/** Chevron-first trigger of a section that ships collapsed; the mono meta summarizes what the fold hides. */
export function SessionInspectorDisclosureHead({
  children,
  meta,
  className,
  ...props
}: Omit<ComponentProps<typeof CollapsibleTrigger>, "render"> & {
  meta?: ReactNode;
}) {
  return (
    <CollapsibleTrigger
      render={
        <button
          type="button"
          className={cn(
            "group flex w-full items-center gap-1.5 rounded-sm text-left text-fg outline-none focus-visible:shadow-focus-ring",
            className
          )}
        />
      }
      {...props}
    >
      <ChevronRight
        aria-hidden="true"
        className="size-3.25 shrink-0 text-subtle transition-transform duration-base ease-out group-aria-expanded:rotate-90 motion-reduce:transition-none"
      />
      <span className="text-form-label font-medium">{children}</span>
      {meta ? (
        <span className="ml-auto truncate font-mono text-mono-id tabular-nums text-muted">
          {meta}
        </span>
      ) : null}
    </CollapsibleTrigger>
  );
}

/** The compact `Empty` inside the rail's dashed hairline frame; five sections share this frame. */
export function SessionInspectorEmpty({ className, ...props }: Omit<EmptyProps, "size" | "fill">) {
  return (
    <Empty
      size="compact"
      fill={false}
      className={cn("rounded-lg border border-dashed border-line-soft px-3 py-4.5", className)}
      {...props}
    />
  );
}

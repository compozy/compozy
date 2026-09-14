import type { ComponentProps, ReactNode } from "react";
import { Empty, Eyebrow, cn, type EmptyProps } from "@compozy/ui";

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

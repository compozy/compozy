import {
  ComposerPrimitive,
  type Unstable_TriggerItem,
  unstable_useTriggerPopoverScopeContext as useTriggerPopoverScopeContext,
} from "@assistant-ui/react";
import type { LucideIcon } from "lucide-react";
import { createElement, useEffect, useRef, type ComponentProps, type ReactNode } from "react";

import { cn } from "@/lib/utils";

/**
 * Rows, headers and messages shared by the composer's trigger popovers — the
 * `/` command menu and the `@` mention menu — so both speak one grammar.
 */

export function TriggerMenuRow({
  item,
  flatIndex,
  icon,
  className,
  children,
  trailing,
  ...props
}: Omit<ComponentProps<typeof ComposerPrimitive.Unstable_TriggerPopoverItem>, "item" | "index"> & {
  item: Unstable_TriggerItem;
  flatIndex: number;
  icon: LucideIcon;
  /** Title and secondary text, laid out on one baseline. */
  children: ReactNode;
  trailing?: ReactNode;
}) {
  const { highlightedIndex } = useTriggerPopoverScopeContext();
  const isHighlighted = highlightedIndex === flatIndex;
  const rowRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (isHighlighted) rowRef.current?.scrollIntoView({ block: "nearest" });
  }, [isHighlighted]);

  return (
    <ComposerPrimitive.Unstable_TriggerPopoverItem
      ref={rowRef}
      item={item}
      index={flatIndex}
      className={cn(
        "group/trigger-row flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left",
        "text-small-body text-fg outline-none select-none",
        "transition-colors duration-fast ease-out",
        "data-[highlighted]:bg-surface-2 data-[highlighted]:text-fg",
        className
      )}
      {...props}
    >
      <span
        className={cn(
          "flex size-4 shrink-0 items-center justify-center text-muted",
          "group-data-[highlighted]/trigger-row:text-fg"
        )}
      >
        {createElement(icon, { className: "size-3.5", "aria-hidden": "true" })}
      </span>
      <span className="flex min-w-0 flex-1 items-baseline gap-1.5">{children}</span>
      {trailing ? (
        <span className="ml-auto flex shrink-0 items-center gap-1.5">{trailing}</span>
      ) : null}
    </ComposerPrimitive.Unstable_TriggerPopoverItem>
  );
}

/** Secondary text after a row title: caption size, subtle ink, truncates first. */
export function TriggerMenuRowSecondary({ className, ...props }: ComponentProps<"span">) {
  return (
    <span className={cn("truncate text-transcript-caption text-subtle", className)} {...props} />
  );
}

export function TriggerMenuGroupHeader({
  label,
  ...props
}: { label: string } & Omit<ComponentProps<"div">, "children">) {
  return (
    <div role="presentation" className="eyebrow px-2 pt-2 pb-1 text-muted" {...props}>
      {label}
    </div>
  );
}

export function TriggerMenuDivider() {
  return <div role="presentation" className="-mx-1 my-1 h-px bg-line" />;
}

/** The empty / error / loading row; one element and one tone for every state. */
export function TriggerMenuMessage({ className, ...props }: ComponentProps<"p">) {
  return (
    <p className={cn("px-2 py-4 text-center text-small-body text-muted", className)} {...props} />
  );
}

/** A quiet note under a group's rows, such as a truncated index. */
export function TriggerMenuFooter({ className, ...props }: ComponentProps<"p">) {
  return (
    <p
      className={cn(
        "mt-0.5 border-t border-line-soft px-2 pt-1.5 pb-1 text-transcript-caption text-muted",
        className
      )}
      {...props}
    />
  );
}

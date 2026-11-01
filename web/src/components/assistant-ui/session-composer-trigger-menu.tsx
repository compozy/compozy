import {
  ComposerPrimitive,
  type Unstable_TriggerItem,
  unstable_useTriggerPopoverScopeContext as useTriggerPopoverScopeContext,
} from "@assistant-ui/react";
import type { LucideIcon } from "@compozy/ui/icons";
import { createElement, useEffect, useRef, type ComponentProps, type ReactNode } from "react";

import { Skeleton, SkeletonText } from "@compozy/ui";

import { cn } from "@/lib/utils";

import {
  TRIGGER_MENU_ROW_GEOMETRY,
  TRIGGER_MENU_ROW_ICON_WELL,
  TRIGGER_MENU_ROW_LABEL,
} from "./session-composer-trigger-menu-classes";

/**
 * Rows, headers and messages shared by the composer's trigger popovers — the
 * `/` command menu and the `@` mention menu — so both speak one grammar.
 */

/** Overrides the primitive's highlight for a row the keyboard cannot reach. */
const UNHIGHLIGHTABLE_ROW_PROPS = {
  "aria-selected": false,
  "data-highlighted": undefined,
} as const;

export function TriggerMenuRow({
  item,
  flatIndex,
  icon,
  className,
  children,
  trailing,
  highlightable = true,
  ...props
}: Omit<ComponentProps<typeof ComposerPrimitive.Unstable_TriggerPopoverItem>, "item" | "index"> & {
  item: Unstable_TriggerItem;
  flatIndex: number;
  icon: LucideIcon;
  /**
   * False keeps the row on screen and clickable but out of keyboard reach: it
   * is never highlighted, so Enter/Tab cannot commit it. The `@` menu uses it
   * for rows that answer an earlier query.
   */
  highlightable?: boolean;
  /** Title and secondary text, laid out on one baseline. */
  children: ReactNode;
  trailing?: ReactNode;
}) {
  const { highlightedIndex } = useTriggerPopoverScopeContext();
  const isHighlighted = highlightable && highlightedIndex === flatIndex;
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
        TRIGGER_MENU_ROW_GEOMETRY,
        "group/trigger-row text-left",
        "text-fg outline-none select-none",
        "transition-colors duration-fast ease-out",
        "data-[highlighted]:bg-surface-2 data-[highlighted]:text-fg",
        className
      )}
      {...props}
      {...(highlightable ? {} : UNHIGHLIGHTABLE_ROW_PROPS)}
    >
      <span
        className={cn(
          TRIGGER_MENU_ROW_ICON_WELL,
          "text-muted-foreground",
          "group-data-[highlighted]/trigger-row:text-fg"
        )}
      >
        {createElement(icon, { className: "size-3.5", "aria-hidden": "true" })}
      </span>
      <span className={TRIGGER_MENU_ROW_LABEL}>{children}</span>
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
    <div role="presentation" className="eyebrow px-2 pt-2 pb-1 text-muted-foreground" {...props}>
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
    <p
      className={cn("px-2 py-4 text-center text-small-body text-muted-foreground", className)}
      {...props}
    />
  );
}

const LOADING_ROW_POSITIONS = [0, 1, 2] as const;
const LOADING_TITLE_WIDTHS = ["w-28", "w-20", "w-24"] as const;

/**
 * Placeholder rows for a trigger menu, or one of its groups, whose first answer is on its way:
 * the live row's frame with a title and a secondary line of text. The enclosing popover is
 * already the polite status surface, so the rows stay silent and `children` is the one thing
 * announced, in the words the empty and error rows speak ("Loading files…").
 */
export function TriggerMenuLoading({
  children,
  rows = 3,
  ...props
}: { rows?: number } & ComponentProps<"div">) {
  return (
    <div {...props}>
      <span className="sr-only">{children}</span>
      {LOADING_ROW_POSITIONS.slice(0, rows).map(position => (
        <div key={position} aria-hidden="true" className={TRIGGER_MENU_ROW_GEOMETRY}>
          <span className={TRIGGER_MENU_ROW_ICON_WELL}>
            <Skeleton className="size-3.5 rounded-xs" />
          </span>
          <span className={cn(TRIGGER_MENU_ROW_LABEL, "min-h-5 items-center")}>
            <SkeletonText className={cn("font-medium", LOADING_TITLE_WIDTHS[position])} />
            <SkeletonText className="w-32 text-transcript-caption" />
          </span>
        </div>
      ))}
    </div>
  );
}

/** A quiet note under a group's rows, such as a truncated index. */
export function TriggerMenuFooter({ className, ...props }: ComponentProps<"p">) {
  return (
    <p
      className={cn(
        "mt-0.5 border-t border-line-soft px-2 pt-1.5 pb-1 text-transcript-caption text-muted-foreground",
        className
      )}
      {...props}
    />
  );
}

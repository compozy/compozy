"use client";

import * as React from "react";
import { DayPicker, getDefaultClassNames, type DayButton } from "react-day-picker";

import { ChevronDown, ChevronLeft, ChevronRight } from "../icons";
import { cn } from "../lib/utils";
import { Button } from "./button";
import { buttonVariants } from "./button-variants";

export type CalendarProps = React.ComponentProps<typeof DayPicker>;

/**
 * Month grid on react-day-picker, ported from the shadcn base-nova calendar
 * (via the ReUI registry) onto Compozy tokens: the selected day is the
 * inverted primary plate, today carries a hairline rim, outside and disabled
 * days fall back to quiet ink. Every DayPicker mode and prop passes through.
 */
function Calendar({
  className,
  classNames,
  showOutsideDays = true,
  captionLayout = "label",
  components,
  ...props
}: CalendarProps) {
  const defaultClassNames = getDefaultClassNames();
  const navButton = cn(
    buttonVariants({ variant: "quiet", size: "icon-sm" }),
    "size-8 rounded-sm p-0 select-none aria-disabled:pointer-events-none aria-disabled:opacity-40"
  );

  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      captionLayout={captionLayout}
      className={cn(
        "group/calendar p-2 text-small-body text-fg",
        String.raw`rtl:**:[.rdp-button\_next>svg]:rotate-180`,
        String.raw`rtl:**:[.rdp-button\_previous>svg]:rotate-180`,
        className
      )}
      classNames={{
        root: cn("w-fit", defaultClassNames.root),
        months: cn("relative flex flex-col gap-4 md:flex-row", defaultClassNames.months),
        month: cn("flex w-full flex-col gap-3", defaultClassNames.month),
        nav: cn(
          "absolute inset-x-0 top-0 flex w-full items-center justify-between gap-1",
          defaultClassNames.nav
        ),
        button_previous: cn(navButton, defaultClassNames.button_previous),
        button_next: cn(navButton, defaultClassNames.button_next),
        month_caption: cn(
          "flex h-8 w-full items-center justify-center px-8",
          defaultClassNames.month_caption
        ),
        dropdowns: cn(
          "flex h-8 w-full items-center justify-center gap-1.5 font-medium",
          defaultClassNames.dropdowns
        ),
        dropdown_root: cn("relative rounded-sm", defaultClassNames.dropdown_root),
        dropdown: cn("absolute inset-0 bg-popover opacity-0", defaultClassNames.dropdown),
        caption_label: cn(
          "font-medium text-fg select-none",
          captionLayout === "label"
            ? "text-small-body"
            : "flex items-center gap-1 rounded-sm text-small-body [&>svg]:size-3.5 [&>svg]:text-subtle",
          defaultClassNames.caption_label
        ),
        month_grid: cn("w-full border-collapse", defaultClassNames.month_grid),
        weekdays: cn("flex", defaultClassNames.weekdays),
        weekday: cn(
          "w-8 flex-1 text-eyebrow font-normal text-subtle select-none",
          defaultClassNames.weekday
        ),
        week: cn("mt-1 flex w-full", defaultClassNames.week),
        week_number_header: cn("w-8 select-none", defaultClassNames.week_number_header),
        week_number: cn("text-eyebrow text-subtle select-none", defaultClassNames.week_number),
        day: cn(
          "group/day relative aspect-square h-full w-full p-0 text-center select-none",
          defaultClassNames.day
        ),
        range_start: cn("rounded-l-sm bg-surface-2", defaultClassNames.range_start),
        range_middle: cn("rounded-none bg-surface-2", defaultClassNames.range_middle),
        range_end: cn("rounded-r-sm bg-surface-2", defaultClassNames.range_end),
        today: cn("text-fg-strong", defaultClassNames.today),
        outside: cn("text-subtle", defaultClassNames.outside),
        disabled: cn("text-disabled", defaultClassNames.disabled),
        hidden: cn("invisible", defaultClassNames.hidden),
        ...classNames,
      }}
      components={{
        Root: ({ className: rootClassName, rootRef, ...rootProps }) => (
          <div data-slot="calendar" ref={rootRef} className={cn(rootClassName)} {...rootProps} />
        ),
        Chevron: ({ className: chevronClassName, orientation, size: _size, ...chevronProps }) => {
          const Icon =
            orientation === "left"
              ? ChevronLeft
              : orientation === "right"
                ? ChevronRight
                : ChevronDown;
          return (
            <Icon aria-hidden="true" className={cn("size-4", chevronClassName)} {...chevronProps} />
          );
        },
        DayButton: CalendarDayButton,
        ...components,
      }}
      {...props}
    />
  );
}

/** One day of the grid: a ghost cell whose selected state is the primary plate. */
function CalendarDayButton({
  className,
  day,
  modifiers,
  ...props
}: React.ComponentProps<typeof DayButton>) {
  const defaultClassNames = getDefaultClassNames();
  const ref = React.useRef<HTMLButtonElement>(null);
  React.useEffect(() => {
    if (modifiers.focused) ref.current?.focus();
  }, [modifiers.focused]);

  return (
    <Button
      ref={ref}
      variant="ghost"
      size="icon-sm"
      data-day={day.isoDate}
      data-today={modifiers.today || undefined}
      data-selected-single={
        (modifiers.selected &&
          !modifiers.range_start &&
          !modifiers.range_end &&
          !modifiers.range_middle) ||
        undefined
      }
      data-range-start={modifiers.range_start || undefined}
      data-range-end={modifiers.range_end || undefined}
      data-range-middle={modifiers.range_middle || undefined}
      className={cn(
        "relative flex aspect-square size-auto w-full min-w-8 rounded-sm font-normal text-inherit tabular-nums active:not-aria-[haspopup]:translate-y-0",
        "data-today:shadow-[inset_0_0_0_1px_var(--color-line-strong)]",
        "data-[selected-single=true]:bg-primary data-[selected-single=true]:text-primary-foreground data-[selected-single=true]:shadow-none data-[selected-single=true]:hover:bg-primary-hover",
        "data-range-start:bg-primary data-range-start:text-primary-foreground data-range-end:bg-primary data-range-end:text-primary-foreground data-range-middle:rounded-none data-range-middle:bg-transparent",
        "disabled:text-disabled disabled:hover:bg-transparent",
        defaultClassNames.day,
        className
      )}
      {...props}
    />
  );
}

export { Calendar, CalendarDayButton };

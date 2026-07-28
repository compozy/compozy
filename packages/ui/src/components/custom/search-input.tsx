"use client";

import { SearchIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";

export interface SearchInputProps extends Omit<
  React.ComponentProps<"input">,
  "onChange" | "value" | "size"
> {
  value?: string;
  onChange?: (next: string) => void;
  placeholder?: string;
  kbd?: React.ReactNode;
  containerClassName?: string;
}

/**
 * Compact search field — 28 px row, 220 px min-width, panel-tone surface.
 * Focus strengthens the border and draws the 2 px focus ring; no accent ring.
 */
function SearchInput({
  value,
  onChange,
  placeholder = "Search...",
  kbd,
  className,
  containerClassName,
  disabled,
  ...props
}: SearchInputProps) {
  const isControlled = value !== undefined;

  return (
    <div
      data-slot="search-input"
      data-disabled={disabled ? "true" : undefined}
      className={cn(
        "flex h-search min-w-search-input-min items-center gap-2 rounded-md border border-line bg-canvas-soft px-2 text-small-body text-fg transition-colors focus-within:border-line-strong focus-within:shadow-focus-ring",
        "data-[disabled=true]:cursor-not-allowed data-[disabled=true]:border-line-soft data-[disabled=true]:bg-canvas data-[disabled=true]:text-disabled data-[disabled=true]:opacity-100",
        containerClassName
      )}
    >
      <SearchIcon aria-hidden="true" className="size-3 shrink-0 text-subtle" />
      <input
        type="search"
        data-slot="search-input-control"
        placeholder={placeholder}
        {...(isControlled ? { value } : {})}
        onChange={event => onChange?.(event.target.value)}
        disabled={disabled}
        className={cn(
          "min-w-0 flex-1 bg-transparent text-small-body text-fg outline-none placeholder:text-subtle disabled:cursor-not-allowed",
          className
        )}
        {...props}
      />
      {kbd ? (
        <span
          data-slot="search-input-kbd"
          aria-hidden="true"
          className="eyebrow hidden items-center rounded-xs border border-line bg-canvas-soft px-1 py-px text-subtle sm:inline-flex"
        >
          {kbd}
        </span>
      ) : null}
    </div>
  );
}

export { SearchInput };

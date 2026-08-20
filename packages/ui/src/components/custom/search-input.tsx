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
 * Compact toolbar search — `--height-search` (28px) matches the RouteNav /
 * PillGroup track. Eyebrow type + leading-none keep icon and text centered
 * without clipping. Focus strengthens the border and draws the 2 px ring.
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
        "flex h-search min-h-0 min-w-search-input-min shrink-0 items-center gap-1.5 rounded-md border border-line bg-canvas-soft px-2 text-eyebrow leading-none text-fg transition-colors focus-within:border-line-strong focus-within:shadow-focus-ring",
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
          "h-full min-h-0 min-w-0 flex-1 appearance-none bg-transparent py-0 text-eyebrow leading-none text-fg outline-none placeholder:text-subtle disabled:cursor-not-allowed [&::-webkit-search-cancel-button]:appearance-none [&::-webkit-search-decoration]:appearance-none",
          className
        )}
        {...props}
      />
      {kbd ? (
        <span
          data-slot="search-input-kbd"
          aria-hidden="true"
          className="eyebrow hidden items-center rounded-xs border border-line bg-canvas-soft px-1 py-px leading-none text-subtle sm:inline-flex"
        >
          {kbd}
        </span>
      ) : null}
    </div>
  );
}

export { SearchInput };

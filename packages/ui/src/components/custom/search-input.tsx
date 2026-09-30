"use client";

import { useMergedRefs } from "@base-ui/utils/useMergedRefs";
import { SearchIcon, XIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";
import { Button } from "../button";
import { Kbd } from "../kbd";

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
 *
 * The native search cancel glyph is suppressed (it paints browser blue/white,
 * off-token); a quiet clear button takes its place while the field has text,
 * standing in for the `kbd` hint. Like the native one it is not a tab stop —
 * Escape clears a search field from the keyboard.
 */
function SearchInput({
  value,
  onChange,
  placeholder = "Search...",
  kbd,
  className,
  containerClassName,
  disabled,
  readOnly,
  defaultValue,
  ref,
  ...props
}: SearchInputProps) {
  const isControlled = value !== undefined;
  const inputRef = React.useRef<HTMLInputElement | null>(null);
  const mergedRef = useMergedRefs(inputRef, ref);
  const [uncontrolledFilled, setUncontrolledFilled] = React.useState(
    () => String(defaultValue ?? "") !== ""
  );
  const filled = isControlled ? value !== "" : uncontrolledFilled;
  const clearable = filled && !disabled && !readOnly;

  const clear = () => {
    const input = inputRef.current;
    if (!isControlled && input) input.value = "";
    setUncontrolledFilled(false);
    onChange?.("");
    input?.focus();
  };

  return (
    <div
      data-slot="search-input"
      data-disabled={disabled ? "true" : undefined}
      className={cn(
        "flex h-search min-h-0 min-w-search-input shrink-0 items-center gap-1.5 rounded-md border border-line bg-canvas px-2 text-eyebrow leading-none text-fg transition-colors hover:border-line-strong focus-within:border-line-strong focus-within:shadow-focus-ring",
        "data-[disabled=true]:cursor-not-allowed data-[disabled=true]:border-line-soft data-[disabled=true]:bg-canvas data-[disabled=true]:text-disabled data-[disabled=true]:opacity-100",
        containerClassName
      )}
    >
      <SearchIcon aria-hidden="true" className="size-3 shrink-0 text-subtle" />
      <input
        type="search"
        data-slot="search-input-control"
        placeholder={placeholder}
        ref={mergedRef}
        {...(isControlled ? { value } : { defaultValue })}
        onChange={event => {
          if (!isControlled) setUncontrolledFilled(event.target.value !== "");
          onChange?.(event.target.value);
        }}
        disabled={disabled}
        readOnly={readOnly}
        className={cn(
          "h-full min-h-0 w-0 min-w-0 flex-1 bg-transparent py-0 text-eyebrow leading-none text-fg outline-none placeholder:text-subtle disabled:cursor-not-allowed [&::-webkit-search-cancel-button]:appearance-none [&::-webkit-search-decoration]:appearance-none",
          className
        )}
        {...props}
      />
      {clearable ? (
        <Button
          type="button"
          variant="quiet"
          size="icon-xs"
          data-slot="search-input-clear"
          aria-label="Clear search field"
          tabIndex={-1}
          onMouseDown={event => event.preventDefault()}
          onClick={clear}
          className="-mr-1.5 text-subtle"
        >
          <XIcon aria-hidden="true" className="size-3" strokeWidth={1.75} />
        </Button>
      ) : kbd ? (
        <Kbd data-slot="search-input-kbd" aria-hidden="true" className="hidden sm:inline-flex">
          {kbd}
        </Kbd>
      ) : null}
    </div>
  );
}

export { SearchInput };

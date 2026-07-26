"use client";

import { cn } from "@agh/ui";
import { Search } from "lucide-react";
import { useSearchContext } from "fumadocs-ui/contexts/search";
import type { ComponentProps } from "react";
import { useSiteSearch } from "@/components/site/hooks/use-site-search";

type HeaderSearchInputProps = Omit<ComponentProps<"form">, "onSubmit"> & {
  hideIfDisabled?: boolean;
};

const searchInputClasses = [
  "min-w-0 flex-1 bg-transparent text-sm text-fg outline-none",
  "placeholder:text-muted focus-visible:outline-none",
];

const keyboardHintClasses = [
  "ms-auto hidden items-center gap-0.5 text-eyebrow text-subtle",
  "xl:inline-flex [&_kbd]:rounded-md [&_kbd]:border [&_kbd]:border-line",
  "[&_kbd]:bg-background [&_kbd]:px-1.5 [&_kbd]:font-mono",
];

export function HeaderSearchInput({ className, hideIfDisabled, ...props }: HeaderSearchInputProps) {
  const { enabled, hotKey, setOpenSearch } = useSearchContext();
  const { openWithQuery, query } = useSiteSearch();

  if (hideIfDisabled && !enabled) return null;

  function openSearch(nextQuery: string) {
    openWithQuery(nextQuery);
    setOpenSearch(true);
  }

  return (
    <form
      role="search"
      aria-label="Site search"
      className={cn(
        "inline-flex h-9 items-center gap-2 rounded-full border border-line bg-canvas-soft px-2.5 text-muted transition-colors hover:bg-hover focus-within:shadow-focus-ring focus-within:text-fg",
        className
      )}
      onSubmit={event => {
        event.preventDefault();
        openSearch(query);
      }}
      {...props}
    >
      <Search aria-hidden className="size-4 shrink-0" />
      <input
        type="search"
        aria-label="Search docs"
        placeholder="Search docs"
        value={query}
        className={cn(searchInputClasses)}
        onFocus={() => {
          openSearch(query);
        }}
        onChange={event => {
          const nextQuery = event.currentTarget.value;
          openSearch(nextQuery);
        }}
      />
      <span aria-hidden className={cn(keyboardHintClasses)}>
        {hotKey.map((key, index) => (
          <kbd key={index}>{key.display}</kbd>
        ))}
      </span>
    </form>
  );
}

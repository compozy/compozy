import { Bot } from "lucide-react";

import { CommandItem, CommandShortcut } from "@compozy/ui";

import type { PaletteAgentFallback } from "../lib/cmd-palette-sections";

export interface OsPaletteFallbackRowProps {
  fallback: PaletteAgentFallback;
  pending: boolean;
  onSelect(query: string): void;
}

/** Agent delegation is a normal result row, with a distinct informational treatment. */
export function OsPaletteFallbackRow({ fallback, pending, onSelect }: OsPaletteFallbackRowProps) {
  return (
    <CommandItem
      forceMount
      aria-busy={pending || undefined}
      className="mt-2 h-10 gap-3 border-t border-line-soft px-3 pt-2 text-info"
      data-palette-row={fallback.value}
      data-testid="os-palette-agent-fallback"
      value={fallback.value}
      onSelect={() => onSelect(fallback.query)}
    >
      <span className="flex size-[18px] shrink-0 items-center justify-center rounded-full bg-info-tint text-info">
        <Bot aria-hidden="true" className="size-3" />
      </span>
      <span className="min-w-0 truncate text-fg">
        Ask agent: <span className="text-info">&apos;{fallback.query}&apos;</span>
      </span>
      <CommandShortcut>{pending ? "starting…" : "↵"}</CommandShortcut>
    </CommandItem>
  );
}

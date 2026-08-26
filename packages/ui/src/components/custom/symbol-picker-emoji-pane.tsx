import { EmojiPicker, type Locale as EmojiLocale } from "frimousse";

import { IDENTITY_SURFACE_VALUE, identityColorsFor } from "../../lib/identity-palette";
import type { SymbolPickerLabels } from "../../lib/symbol-palette";
import { cn } from "../../lib/utils";
import { Spinner } from "../spinner";

export interface SymbolPickerEmojiPaneProps {
  /** Base URL serving Emojibase JSON as `{emojibaseUrl}/{locale}/data.json`. */
  emojibaseUrl: string;
  locale?: EmojiLocale;
  /** Currently chosen emoji grapheme, or "" when an icon owns the symbol. */
  selected: string;
  onSelect: (emoji: string) => void;
  color: string;
  surface?: string;
  labels: SymbolPickerLabels;
}

/** Emoji tab body: frimousse search, skin tone, and virtualized grid. */
export function SymbolPickerEmojiPane({
  emojibaseUrl,
  locale = "en",
  selected,
  onSelect,
  color,
  surface = IDENTITY_SURFACE_VALUE,
  labels,
}: SymbolPickerEmojiPaneProps) {
  const plate = identityColorsFor(color, surface).bg;

  return (
    <EmojiPicker.Root
      emojibaseUrl={emojibaseUrl}
      locale={locale}
      onEmojiSelect={picked => onSelect(picked.emoji)}
      data-slot="symbol-picker-emoji-pane"
      className="flex flex-col gap-3"
    >
      <div className="flex items-center gap-2">
        <EmojiPicker.Search
          aria-label={labels.searchEmojis}
          placeholder={labels.searchEmojis}
          data-slot="symbol-picker-emoji-search"
          className={cn(
            "h-search min-w-0 flex-1 rounded-md border border-line bg-input px-2.5",
            "text-small-body text-fg placeholder:text-subtle",
            "outline-none focus-visible:shadow-focus-ring"
          )}
        />
        <EmojiPicker.SkinToneSelector
          title={labels.skinTone}
          aria-label={labels.skinTone}
          data-slot="symbol-picker-skin-tone"
          className={cn(
            "grid size-(--height-search) shrink-0 cursor-pointer place-items-center rounded-md",
            "text-small-body outline-none hover:bg-row-hover focus-visible:shadow-focus-ring"
          )}
        />
      </div>
      <EmojiPicker.Viewport className="h-(--height-symbol-picker-grid) rounded-md outline-none">
        <EmojiPicker.Loading
          aria-label={labels.loadingEmojis}
          className="grid h-full place-items-center"
        >
          <Spinner className="size-4 text-subtle" />
        </EmojiPicker.Loading>
        <EmojiPicker.Empty className="block px-1 py-3 text-small-body text-subtle">
          {({ search }) => labels.noResults("emoji", search, labels.icons)}
        </EmojiPicker.Empty>
        <EmojiPicker.List
          className="pb-1 select-none"
          components={{
            CategoryHeader: ({ category, ...props }) => (
              <div
                {...props}
                className="bg-canvas-soft px-1 pt-2 pb-1 text-micro font-medium text-subtle"
              >
                {category.label}
              </div>
            ),
            Row: ({ children, ...props }) => (
              <div {...props} className="scroll-my-1 px-0.5">
                {children}
              </div>
            ),
            Emoji: ({ emoji, ...props }) => (
              <button
                {...props}
                type="button"
                data-selected-emoji={emoji.emoji === selected ? "true" : undefined}
                style={emoji.emoji === selected ? { backgroundColor: plate } : undefined}
                className={cn(
                  "grid size-symbol-picker-cell cursor-pointer place-items-center rounded-xs",
                  "text-small-body transition-colors data-[active]:bg-row-selected hover:bg-row-hover"
                )}
              >
                {emoji.emoji}
              </button>
            ),
          }}
        />
      </EmojiPicker.Viewport>
    </EmojiPicker.Root>
  );
}

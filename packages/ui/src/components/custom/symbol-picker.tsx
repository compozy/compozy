import * as React from "react";
import type { Locale as EmojiLocale } from "frimousse";

import type {
  SymbolIconOption,
  SymbolKind,
  SymbolPickerLabels,
  SymbolSwatch,
  SymbolValue,
} from "../../lib/symbol-palette";
import { matchesSymbolQuery, SYMBOL_PICKER_DEFAULT_LABELS } from "../../lib/symbol-palette";
import { cn } from "../../lib/utils";
import { PillGroup } from "./pill-group";
import { SearchInput } from "./search-input";
import { SymbolPickerColorSection } from "./symbol-picker-color-section";
import { SymbolPickerEmojiPane } from "./symbol-picker-emoji-pane";
import { SymbolPickerIconGrid } from "./symbol-picker-icon-grid";

export interface SymbolPickerProps extends Omit<React.ComponentProps<"div">, "onChange"> {
  /** The chosen color, as `#rrggbb`. */
  color: string;
  onColorChange: (next: string) => void;
  onColorValidityChange?: (valid: boolean) => void;
  symbol: SymbolValue;
  onSymbolChange: (next: SymbolValue) => void;
  /** Full icon catalog, usually from a lazily loaded manifest. */
  icons: readonly SymbolIconOption[];
  iconsLoading?: boolean;
  /** URL of the SVG sprite whose `<symbol>` ids are the icon names. */
  spriteUrl: string;
  /** Base URL serving Emojibase JSON as `{emojibaseUrl}/{locale}/data.json`. */
  emojibaseUrl: string;
  emojiLocale?: EmojiLocale;
  swatches: readonly SymbolSwatch[];
  /** Surface the grid is painted on, so ink contrast is measured against it. */
  surface?: string;
  /** Generic English by default; override to localize or re-word. */
  labels?: SymbolPickerLabels;
}

/** Picks a symbol and color while previewing their combined identity. */
export function SymbolPicker({
  className,
  color,
  onColorChange,
  onColorValidityChange,
  symbol,
  onSymbolChange,
  icons,
  iconsLoading = false,
  spriteUrl,
  emojibaseUrl,
  emojiLocale,
  swatches,
  surface,
  labels = SYMBOL_PICKER_DEFAULT_LABELS,
  ...props
}: SymbolPickerProps) {
  // An explicit tab choice persists until the component remounts.
  const [browsing, setBrowsing] = React.useState<SymbolKind | null>(null);
  const [query, setQuery] = React.useState("");
  const kind = browsing ?? symbol.kind;

  const matchingIcons = icons.filter(icon => matchesSymbolQuery(icon, query));
  const otherTab = kind === "icon" ? labels.emojis : labels.icons;

  const handleKindChange = (next: SymbolKind) => {
    setBrowsing(next);
    setQuery("");
  };

  return (
    <div data-slot="symbol-picker" className={cn("flex flex-col gap-3", className)} {...props}>
      <PillGroup
        items={[
          { value: "icon", label: labels.icons },
          { value: "emoji", label: labels.emojis },
        ]}
        value={kind}
        onChange={handleKindChange}
        size="sm"
        aria-label={labels.kind}
        className="self-start"
      />
      <SymbolPickerColorSection
        color={color}
        onChange={onColorChange}
        onValidityChange={onColorValidityChange}
        swatches={swatches}
        swatchesLabel={labels.swatches}
        customLabel={labels.customColor}
        customToggleLabel={labels.customColorToggle}
        invalidMessage={labels.invalidColor}
      />
      {kind === "icon" ? (
        <>
          <SearchInput
            value={query}
            onChange={setQuery}
            placeholder={labels.searchIcons}
            aria-label={labels.searchIcons}
            className="w-full"
          />
          <SymbolPickerIconGrid
            icons={matchingIcons}
            spriteUrl={spriteUrl}
            selected={symbol.kind === "icon" ? symbol.value : ""}
            onSelect={value => onSymbolChange({ kind: "icon", value })}
            color={color}
            surface={surface}
            label={labels.icons}
            emptyMessage={labels.noResults("icon", query.trim(), otherTab)}
            loading={iconsLoading}
            loadingLabel={labels.loadingIcons}
          />
        </>
      ) : (
        <SymbolPickerEmojiPane
          emojibaseUrl={emojibaseUrl}
          locale={emojiLocale}
          selected={symbol.kind === "emoji" ? symbol.value : ""}
          onSelect={value => onSymbolChange({ kind: "emoji", value })}
          color={color}
          surface={surface}
          labels={labels}
        />
      )}
    </div>
  );
}

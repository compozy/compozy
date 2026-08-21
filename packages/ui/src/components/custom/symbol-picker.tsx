import * as React from "react";

import type {
  SymbolEmojiOption,
  SymbolIconOption,
  SymbolKind,
  SymbolPickerLabels,
  SymbolSwatch,
  SymbolValue,
} from "../../lib/symbol-palette";
import { matchesSymbolQuery, SYMBOL_PICKER_DEFAULT_LABELS } from "../../lib/symbol-palette";
import { cn } from "../../lib/utils";
import type { KindIconRegistry } from "./kind-icon-registry";
import { PillGroup } from "./pill-group";
import { SearchInput } from "./search-input";
import { SymbolPickerColorRow } from "./symbol-picker-color-row";
import { SymbolPickerGrid } from "./symbol-picker-grid";

export interface SymbolPickerProps extends Omit<React.ComponentProps<"div">, "onChange"> {
  /** The chosen color, as `#rrggbb`. */
  color: string;
  onColorChange: (next: string) => void;
  onColorValidityChange?: (valid: boolean) => void;
  symbol: SymbolValue;
  onSymbolChange: (next: SymbolValue) => void;
  icons: readonly SymbolIconOption[];
  iconRegistry: KindIconRegistry;
  emojis: readonly SymbolEmojiOption[];
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
  iconRegistry,
  emojis,
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
  const matchingEmojis = emojis.filter(emoji => matchesSymbolQuery(emoji, query));
  const otherTab = kind === "icon" ? labels.emojis : labels.icons;

  const handleKindChange = (next: SymbolKind) => {
    setBrowsing(next);
    setQuery("");
  };

  return (
    <div data-slot="symbol-picker" className={cn("flex flex-col gap-3", className)} {...props}>
      <div className="flex items-center gap-2.5">
        <PillGroup
          items={[
            { value: "icon", label: labels.icons },
            { value: "emoji", label: labels.emojis },
          ]}
          value={kind}
          onChange={handleKindChange}
          size="sm"
          aria-label={labels.kind}
        />
        <span className="flex-1" aria-hidden="true" />
        <SearchInput
          value={query}
          onChange={setQuery}
          placeholder={kind === "icon" ? labels.searchIcons : labels.searchEmojis}
          aria-label={kind === "icon" ? labels.searchIcons : labels.searchEmojis}
        />
      </div>
      <SymbolPickerGrid
        kind={kind}
        icons={matchingIcons}
        iconRegistry={iconRegistry}
        emojis={matchingEmojis}
        selected={symbol.kind === kind ? symbol.value : ""}
        onSelect={value => onSymbolChange({ kind, value })}
        color={color}
        {...(surface === undefined ? {} : { surface })}
        label={kind === "icon" ? labels.icons : labels.emojis}
        emptyMessage={`No ${kind === "icon" ? "icons" : "emojis"} match "${query.trim()}". Try the ${otherTab} tab.`}
      />
      <SymbolPickerColorRow
        color={color}
        onChange={onColorChange}
        onValidityChange={onColorValidityChange}
        swatches={swatches}
        swatchesLabel={labels.swatches}
        customLabel={labels.customColor}
        invalidMessage={labels.invalidColor}
      />
    </div>
  );
}

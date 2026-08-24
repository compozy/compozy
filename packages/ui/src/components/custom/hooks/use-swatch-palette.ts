import { useId, useState, type KeyboardEvent } from "react";

import type { SymbolSwatch } from "../../../lib/symbol-palette";

interface SwatchPalette {
  active: number;
  swatchId: (index: number) => string;
  listboxId: string;
  selectedIndex: number;
  pick: (value: string) => void;
  handleKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void;
}

function nextSwatchIndex(key: string, index: number, total: number): number | null {
  if (total === 0) return null;

  switch (key) {
    case "ArrowRight":
    case "ArrowDown":
      return Math.min(index + 1, total - 1);
    case "ArrowLeft":
    case "ArrowUp":
      return Math.max(index - 1, 0);
    case "Home":
      return 0;
    case "End":
      return total - 1;
    default:
      return null;
  }
}

/** Keyboard model shared by the suggested palette's pointer and arrow-key input. */
export function useSwatchPalette(
  swatches: readonly SymbolSwatch[],
  color: string,
  onPick: (value: string) => void
): SwatchPalette {
  const listboxId = useId();
  const [cursor, setCursor] = useState<number | null>(null);
  const selectedIndex = swatches.findIndex(
    swatch => swatch.value.toLowerCase() === color.toLowerCase()
  );
  const active = Math.min(
    Math.max(cursor ?? Math.max(selectedIndex, 0), 0),
    Math.max(swatches.length - 1, 0)
  );
  const pick = (value: string) => {
    setCursor(null);
    onPick(value);
  };
  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      const swatch = swatches[active];
      if (swatch) {
        event.preventDefault();
        pick(swatch.value);
      }
      return;
    }
    const target = nextSwatchIndex(event.key, active, swatches.length);
    if (target === null) return;
    event.preventDefault();
    setCursor(target);
  };
  return {
    active,
    swatchId: (index: number) => `${listboxId}-swatch-${index}`,
    listboxId,
    selectedIndex,
    pick,
    handleKeyDown,
  };
}

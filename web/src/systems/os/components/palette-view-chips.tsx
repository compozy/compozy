import type { CmdPaletteViewPayload } from "../lib/cmd-palette-types";
import { PaletteChipToolbar } from "./palette-chip-toolbar";

type Chip = NonNullable<CmdPaletteViewPayload["chips"]>[number];

const ALL_CHIP_ID = "all";

export function PaletteViewChips({
  active,
  allCount,
  chips,
  onChange,
}: {
  active: string | null;
  allCount: number;
  chips: readonly Chip[];
  onChange: (id: string | null) => void;
}) {
  return (
    <PaletteChipToolbar
      activeId={active ?? ALL_CHIP_ID}
      chips={[{ id: ALL_CHIP_ID, label: "All", count: allCount }, ...chips]}
      label="View filters"
      testIdPrefix="palette-view-chip"
      onSelect={id => onChange(id === ALL_CHIP_ID ? null : id)}
    />
  );
}

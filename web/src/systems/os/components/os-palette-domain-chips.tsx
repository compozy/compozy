import { PaletteChipToolbar } from "./palette-chip-toolbar";

export interface PaletteDomainChip {
  readonly id: string;
  readonly label: string;
  readonly count: number;
}

export function OsPaletteDomainChips({
  active,
  chips,
  onChange,
}: {
  active: string;
  chips: readonly PaletteDomainChip[];
  onChange: (id: string) => void;
}) {
  return (
    <PaletteChipToolbar
      activeId={active}
      chips={chips}
      label="Domain filters"
      testIdPrefix="os-palette-domain-filter"
      onSelect={onChange}
    />
  );
}

import { Pill } from "@compozy/ui";

export interface PaletteChipToolbarItem {
  id: string;
  label: string;
  count?: number | null;
}

export function PaletteChipToolbar({
  activeId,
  chips,
  label,
  onSelect,
  testIdPrefix,
}: {
  activeId: string | null;
  chips: readonly PaletteChipToolbarItem[];
  label: string;
  onSelect: (id: string) => void;
  testIdPrefix: string;
}) {
  return (
    <div
      aria-label={label}
      className="flex gap-1 overflow-x-auto border-b border-line px-4 py-2"
      role="toolbar"
    >
      {chips.map(chip => {
        const countLabel = chip.count == null ? null : String(chip.count);
        return (
          <Pill
            key={chip.id}
            active={activeId === chip.id}
            aria-label={countLabel === null ? chip.label : `${chip.label}, ${countLabel}`}
            data-testid={`${testIdPrefix}-${chip.id}`}
            render={<button type="button" />}
            size="xs"
            onClick={() => onSelect(chip.id)}
          >
            {chip.label}
            {countLabel === null ? null : ` ${countLabel}`}
          </Pill>
        );
      })}
    </div>
  );
}

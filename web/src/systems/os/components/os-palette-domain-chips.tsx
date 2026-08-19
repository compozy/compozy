import { Pill } from "@compozy/ui";

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
    <div className="flex gap-1 overflow-x-auto border-b border-line px-3 py-2" role="toolbar">
      {chips.map(chip => (
        <Pill
          key={chip.id}
          active={active === chip.id}
          aria-label={`${chip.label}, ${chip.count}`}
          data-testid={`os-palette-domain-filter-${chip.id}`}
          render={<button type="button" />}
          size="xs"
          onClick={() => onChange(chip.id)}
        >
          {chip.label} {chip.count}
        </Pill>
      ))}
    </div>
  );
}

import { Pill } from "@compozy/ui";

import type { CmdPaletteViewPayload } from "../lib/cmd-palette-types";

type Chip = NonNullable<CmdPaletteViewPayload["chips"]>[number];

export function PaletteViewChips({
  active,
  chips,
  onChange,
}: {
  active: string;
  chips: readonly Chip[];
  onChange: (id: string) => void;
}) {
  return (
    <div className="flex gap-1 overflow-x-auto border-b border-line px-3 py-2" role="toolbar">
      {chips.map(chip => (
        <Pill
          key={chip.id}
          active={active === chip.id}
          aria-label={chip.count == null ? chip.label : `${chip.label}, ${chip.count}`}
          data-testid={`palette-view-chip-${chip.id}`}
          render={<button type="button" />}
          size="xs"
          onClick={() => onChange(chip.id)}
        >
          {chip.label}
          {chip.count == null ? null : ` ${chip.count}`}
        </Pill>
      ))}
    </div>
  );
}

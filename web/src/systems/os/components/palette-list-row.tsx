import { KindIcon, Pill } from "@compozy/ui";

import { statusTone } from "@/lib/status-tone";

import type { CmdPaletteViewRow } from "../lib/cmd-palette-types";

export function PaletteListRow({ row }: { row: CmdPaletteViewRow }) {
  return (
    <div className="flex min-w-0 flex-1 items-center gap-3">
      <KindIcon kind={row.icon ?? "item"} size="sm" />
      <div className="min-w-0 flex-1">
        <div className="truncate text-card-title text-fg">{row.title}</div>
        {row.subtitle ? (
          <div className="truncate text-small-body text-muted">{row.subtitle}</div>
        ) : null}
      </div>
      {row.accessories?.map(accessory => (
        <span key={accessory} className="shrink-0 text-small-body text-muted">
          {accessory}
        </span>
      ))}
      {row.badge ? (
        <Pill size="xs" tone={statusTone(row.badge.tone)}>
          {row.badge.label}
        </Pill>
      ) : null}
    </div>
  );
}

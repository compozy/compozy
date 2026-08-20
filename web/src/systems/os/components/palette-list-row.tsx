import { KindIcon, Pill } from "@compozy/ui";

import { statusTone } from "@/lib/status-tone";

import { cmdPaletteIconRegistry, isEmojiIcon } from "../lib/cmd-palette-icons";
import type { CmdPaletteViewRow } from "../lib/cmd-palette-types";

export function PaletteListRow({ row }: { row: CmdPaletteViewRow }) {
  return (
    <div className="flex min-w-0 flex-1 items-center gap-3">
      <RowIcon icon={row.icon} />
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

function RowIcon({ icon }: { icon?: string }) {
  if (icon && isEmojiIcon(icon)) {
    return (
      <span aria-hidden="true" className="grid size-4 shrink-0 place-items-center text-small-body">
        {icon}
      </span>
    );
  }
  const token = icon?.trim();
  const Glyph = token ? cmdPaletteIconRegistry[token] : undefined;
  if (Glyph) {
    return <Glyph aria-hidden="true" className="size-4 shrink-0 text-subtle" />;
  }
  return <KindIcon kind={icon ?? "item"} size="sm" />;
}

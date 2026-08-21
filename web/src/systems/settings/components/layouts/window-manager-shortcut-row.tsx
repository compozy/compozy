import { CircleOff, Plus, RotateCcw, TriangleAlert } from "lucide-react";

import { Button, MonoId, Pill, TableCell, TableRow, cn } from "@compozy/ui";

import { ShortcutBindingKeys, CORE_SHORTCUT_SOURCE } from "@/systems/os";

import type { ShortcutTableRow } from "../../lib/window-manager-shortcut-rows";

export interface WindowManagerShortcutRowProps {
  row: ShortcutTableRow;
  recording: boolean;
  busy: boolean;
  /** The alias field for this command; the table owns its state. */
  aliasCell: React.ReactNode;
  /** Rendered under the row when this command is the one in conflict. */
  notice?: React.ReactNode;
  onRecord: (commandId: string, mode?: "alternate") => void;
  onReset: (commandId: string) => void;
}

/** State word + glyph, so the row never leans on tone alone to say what it is. */
function RowFlag({ kind, children }: { kind: "unbound" | "dormant"; children: string }) {
  const Glyph = kind === "unbound" ? CircleOff : TriangleAlert;
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 font-mono text-badge",
        kind === "unbound" ? "text-danger" : "text-warning"
      )}
    >
      <Glyph aria-hidden="true" className="size-3" />
      {children}
    </span>
  );
}

/**
 * One bindable command: what it is, what it answers to, and who contributed it.
 *
 * The chord chip is the recorder trigger and keeps that identity while
 * recording — it never swaps for a different control — so the operator's eye
 * stays where they clicked.
 */
export function WindowManagerShortcutRow({
  row,
  recording,
  busy,
  aliasCell,
  notice,
  onRecord,
  onReset,
}: WindowManagerShortcutRowProps) {
  const dormant = row.dormantReason !== null;
  return (
    <>
      <TableRow
        className={cn(
          row.unbound && "bg-danger-tint",
          dormant && !row.unbound && "bg-warning-tint",
          row.shadowedReason !== null && !row.unbound && !dormant && "bg-warning-tint"
        )}
        data-state={recording ? "recording" : row.overridden ? "custom" : "default"}
        data-testid={`window-manager-shortcut-${row.commandId}`}
      >
        <TableCell className="py-2 align-top">
          <span className="block truncate text-small-body text-fg">{row.title}</span>
          <MonoId className="mt-0.5 block text-faint" value={row.commandId} preserveCase />
        </TableCell>

        <TableCell className="py-2 align-top">
          <div className="flex flex-col items-start gap-1">
            <button
              aria-label={`${row.title} shortcut`}
              className={cn(
                "inline-flex min-h-7 shrink-0 items-center rounded-sm border px-2",
                "border-line bg-btn-default-fill transition-colors duration-base ease-out",
                "hover:border-line-strong hover:bg-btn-default-hover",
                "focus-visible:outline-none focus-visible:shadow-focus-ring",
                "disabled:cursor-not-allowed disabled:opacity-60",
                row.overridden && "border-accent-dim bg-accent-tint",
                recording && "border-accent bg-accent-tint-strong"
              )}
              data-testid={`shortcut-recorder-${row.commandId}`}
              disabled={busy}
              type="button"
              onClick={() => onRecord(row.commandId)}
            >
              {recording ? (
                <span className="text-form-label text-accent-strong">Press keys…</span>
              ) : row.unbound ? (
                <RowFlag kind="unbound">unbound</RowFlag>
              ) : (
                <ShortcutBindingKeys bindings={row.bindings} overridden={row.overridden} />
              )}
            </button>
            {dormant ? (
              <>
                <RowFlag kind="dormant">dormant</RowFlag>
                <p className="text-form-hint text-muted">{row.dormantReason}</p>
              </>
            ) : null}
            {row.shadowedReason !== null ? (
              <p className="text-form-hint text-warning">{row.shadowedReason}</p>
            ) : null}
          </div>
        </TableCell>

        <TableCell className="py-2 align-top">{aliasCell}</TableCell>

        <TableCell className="py-2 align-top">
          {row.source === CORE_SHORTCUT_SOURCE ? (
            <span className="text-form-label text-muted">{row.sourceLabel}</span>
          ) : (
            <Pill className="font-mono" size="xs" tone="info">
              {row.sourceLabel}
            </Pill>
          )}
        </TableCell>

        <TableCell className="py-2 text-right align-top">
          <div className="inline-flex items-center gap-0.5">
            <Button
              aria-label={`Add an alternate shortcut for ${row.title}`}
              disabled={busy}
              size="icon-xs"
              type="button"
              variant="ghost"
              onClick={() => onRecord(row.commandId, "alternate")}
            >
              <Plus aria-hidden="true" className="size-3" />
            </Button>
            <Button
              aria-label={`Reset ${row.title} to its default shortcut`}
              className={cn(!row.overridden && "invisible")}
              data-testid={`shortcut-reset-${row.commandId}`}
              disabled={!row.overridden || busy}
              size="icon-xs"
              type="button"
              variant="ghost"
              onClick={() => onReset(row.commandId)}
            >
              <RotateCcw aria-hidden="true" className="size-3" />
            </Button>
          </div>
        </TableCell>
      </TableRow>
      {notice ? (
        <TableRow className="hover:bg-transparent">
          <TableCell className="pt-0 pb-3" colSpan={5}>
            {notice}
          </TableCell>
        </TableRow>
      ) : null}
    </>
  );
}

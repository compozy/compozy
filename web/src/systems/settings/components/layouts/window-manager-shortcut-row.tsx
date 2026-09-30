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

/** Unbound outranks dormant, which outranks shadowed; each tints the row once. */
function shortcutRowTint(row: ShortcutTableRow): string | undefined {
  if (row.unbound) return "bg-danger-tint";
  if (row.dormantReason !== null || row.shadowedReason !== null) return "bg-warning-tint";
  return undefined;
}

function shortcutRowState(row: ShortcutTableRow, recording: boolean) {
  if (recording) return "recording";
  return row.overridden ? "custom" : "default";
}

function ShortcutChord({ row, recording }: { row: ShortcutTableRow; recording: boolean }) {
  if (recording) return <span className="text-form-label text-fg">Press keys…</span>;
  if (row.unbound) return <RowFlag kind="unbound">unbound</RowFlag>;
  return <ShortcutBindingKeys bindings={row.bindings} overridden={row.overridden} />;
}

function ShortcutBindingCell({
  row,
  recording,
  busy,
  onRecord,
}: Pick<WindowManagerShortcutRowProps, "row" | "recording" | "busy" | "onRecord">) {
  return (
    <div className="flex flex-col items-start gap-1">
      <button
        aria-label={`${row.title} shortcut`}
        className={cn(
          "inline-flex min-h-7 shrink-0 items-center rounded-pill px-2.5",
          "bg-surface-2 transition-colors duration-base ease-out hover:bg-selected",
          "focus-visible:outline-none focus-visible:shadow-focus-ring",
          "disabled:cursor-not-allowed disabled:opacity-60",
          // Listening for keys: the focus ring says the recorder has the keyboard.
          recording && "bg-selected shadow-focus-ring"
        )}
        data-testid={`shortcut-recorder-${row.commandId}`}
        disabled={busy}
        type="button"
        onClick={() => onRecord(row.commandId)}
      >
        <ShortcutChord recording={recording} row={row} />
      </button>
      {row.dormantReason !== null ? (
        <>
          <RowFlag kind="dormant">dormant</RowFlag>
          <p className="text-form-hint text-muted">{row.dormantReason}</p>
        </>
      ) : null}
      {row.shadowedReason !== null ? (
        <p className="text-form-hint text-warning">{row.shadowedReason}</p>
      ) : null}
    </div>
  );
}

function ShortcutSource({ row }: { row: ShortcutTableRow }) {
  if (row.source === CORE_SHORTCUT_SOURCE) {
    return <span className="text-form-label text-muted">{row.sourceLabel}</span>;
  }
  return (
    <Pill className="font-mono" size="xs" tone="info">
      {row.sourceLabel}
    </Pill>
  );
}

function ShortcutActions({
  row,
  busy,
  onRecord,
  onReset,
}: Pick<WindowManagerShortcutRowProps, "row" | "busy" | "onRecord" | "onReset">) {
  return (
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
  return (
    <>
      <TableRow
        className={shortcutRowTint(row)}
        data-state={shortcutRowState(row, recording)}
        data-testid={`window-manager-shortcut-${row.commandId}`}
      >
        <TableCell className="py-2 align-top">
          <span className="block truncate text-small-body text-fg">{row.title}</span>
          <MonoId className="mt-0.5 block text-faint" value={row.commandId} preserveCase />
        </TableCell>

        <TableCell className="py-2 align-top">
          <ShortcutBindingCell busy={busy} onRecord={onRecord} recording={recording} row={row} />
        </TableCell>

        <TableCell className="py-2 align-top">{aliasCell}</TableCell>

        <TableCell className="py-2 align-top">
          <ShortcutSource row={row} />
        </TableCell>

        <TableCell className="py-2 text-right align-top">
          <ShortcutActions busy={busy} onRecord={onRecord} onReset={onReset} row={row} />
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

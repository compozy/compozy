import { useState } from "react";

import { Button, Empty, Table, TableBody, TableHead, TableHeader, TableRow } from "@compozy/ui";

import { orderShortcutSources, shortcutLabel } from "@/systems/os";
import type { WindowManagerSettingsSection } from "@/systems/os";

import type { AliasEditorModel } from "../../hooks/use-window-manager-alias-editor";
import type { ShortcutRecorderModel } from "../../hooks/use-window-manager-shortcut-recorder";
import {
  buildShortcutTableRows,
  shortcutSourceCounts,
} from "../../lib/window-manager-shortcut-rows";
import { SHORTCUT_SOURCE_ALL, ShortcutSourceFilter } from "./shortcut-source-filter";
import { WindowManagerAliasCell } from "./window-manager-alias-cell";
import { WindowManagerBindingConflict } from "./window-manager-binding-conflict";
import { WindowManagerShortcutRow } from "./window-manager-shortcut-row";

export interface WindowManagerShortcutTableProps {
  section: WindowManagerSettingsSection;
  recorder: ShortcutRecorderModel;
  aliases: AliasEditorModel;
  /** Command the operator arrived to bind, from a palette deep link. */
  focusCommandId?: string;
}

/**
 * Every bindable command in one table (S12).
 *
 * The list is the registry itself, so an extension's commands appear here the
 * moment it loads and no command is bindable-but-invisible. The daemon owns
 * every judgement the table shows — which chord is effective, which is
 * overridden, which claim was refused and by whom.
 */
export function WindowManagerShortcutTable({
  section,
  recorder,
  aliases,
  focusCommandId,
}: WindowManagerShortcutTableProps) {
  const [source, setSource] = useState<string>(SHORTCUT_SOURCE_ALL);
  const rows = buildShortcutTableRows(section, recorder.conflicts);
  const counts = shortcutSourceCounts(rows);
  const sources = orderShortcutSources(rows.map(row => row.source));
  const activeSource =
    counts.has(source) || source === SHORTCUT_SOURCE_ALL ? source : SHORTCUT_SOURCE_ALL;
  const visible = rows
    .filter(row => activeSource === SHORTCUT_SOURCE_ALL || row.source === activeSource)
    .sort((left, right) => {
      // The command the operator deep-linked to leads, so "Set shortcut…" lands
      // on the row instead of on a registry they then have to search again.
      if (left.commandId === focusCommandId) return -1;
      if (right.commandId === focusCommandId) return 1;
      return 0;
    });
  const changed = rows.filter(row => row.overridden).length;
  const titleFor = (commandId: string) =>
    rows.find(row => row.commandId === commandId)?.title ?? commandId;

  return (
    <div data-testid="window-manager-shortcut-table">
      <span aria-live="polite" className="sr-only">
        {recorder.announcement}
      </span>

      <div className="flex flex-wrap items-center gap-2 border-b border-line-soft px-4 py-2.5">
        <ShortcutSourceFilter
          counts={counts}
          onSelect={setSource}
          selected={activeSource}
          sources={sources}
        />
        <span className="flex-1" />
        <Button
          data-testid="shortcut-reset-all"
          disabled={changed === 0 || recorder.saving}
          size="sm"
          type="button"
          variant="outline"
          onClick={recorder.resetAll}
        >
          Reset all
        </Button>
      </div>

      {recorder.error !== null ? (
        <p className="px-4 py-2 text-form-hint text-danger" role="alert">
          {recorder.error}
        </p>
      ) : null}

      {section.diagnostics.length > 0 ? (
        <div className="border-b border-line-soft px-4 py-2" role="status">
          {section.diagnostics.map(diagnostic => (
            <p className="text-form-hint text-warning" key={diagnostic.commandId}>
              {diagnostic.commandId}: {diagnostic.message}
            </p>
          ))}
        </div>
      ) : null}

      {visible.length === 0 ? (
        <Empty
          className="py-10"
          data-testid="window-manager-shortcut-empty"
          description={
            rows.length === 0
              ? "The daemon has not reported any bindable commands for this workspace."
              : "No commands come from this source."
          }
          title={rows.length === 0 ? "No commands to bind" : "No matches"}
        />
      ) : (
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="w-2/5">Command</TableHead>
              <TableHead>Effective binding</TableHead>
              <TableHead>Alias</TableHead>
              <TableHead>Source</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">Reset to default</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {visible.map(row => (
              <WindowManagerShortcutRow
                aliasCell={
                  <WindowManagerAliasCell
                    commandId={row.commandId}
                    commandTitle={row.title}
                    onCancel={aliases.cancel}
                    onChange={aliases.change}
                    onCommit={aliases.commit}
                    state={aliases.cell(row.commandId)}
                  />
                }
                busy={recorder.saving}
                key={row.commandId}
                notice={renderNotice(row.commandId, recorder, aliases, titleFor)}
                onRecord={(commandId, mode) => recorder.start(commandId, mode)}
                onReset={recorder.reset}
                recording={recorder.recording === row.commandId}
                row={row}
              />
            ))}
          </TableBody>
        </Table>
      )}
    </div>
  );
}

function renderNotice(
  commandId: string,
  recorder: ShortcutRecorderModel,
  aliases: AliasEditorModel,
  titleFor: (commandId: string) => string
) {
  const chordConflict = recorder.conflict;
  if (chordConflict !== null && chordConflict.commandId === commandId) {
    return (
      <WindowManagerBindingConflict
        claim={shortcutLabel(chordConflict.chord)}
        consequence={`Overwriting leaves ${titleFor(chordConflict.owner)} unbound.`}
        onCancel={recorder.dismissConflict}
        onOverwrite={recorder.overwrite}
        ownerTitle={titleFor(chordConflict.owner)}
        overwriteLabel="Overwrite"
        testId={`shortcut-conflict-${commandId}`}
      />
    );
  }
  const aliasConflict = aliases.conflict;
  if (aliasConflict !== null && aliasConflict.commandId === commandId) {
    return (
      <WindowManagerBindingConflict
        claim={aliasConflict.alias}
        consequence={`Moving it leaves ${aliasConflict.ownerTitle} without an alias.`}
        onCancel={aliases.dismissConflict}
        onOverwrite={aliases.overwrite}
        ownerTitle={aliasConflict.ownerTitle}
        overwriteLabel="Move alias"
        testId={`alias-conflict-${commandId}`}
      />
    );
  }
  return null;
}

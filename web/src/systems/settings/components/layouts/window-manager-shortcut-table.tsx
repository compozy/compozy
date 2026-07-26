import { ChevronRight } from "lucide-react";
import { useState } from "react";

import { cn } from "@agh/ui";

import {
  resolveWindowManagerActions,
  type ResolvedWindowManagerAction,
  type WindowManagerActionId,
} from "@/systems/os";

import type { ShortcutRecorderModel } from "../../hooks/use-window-manager-shortcut-recorder";
import { WindowManagerShortcutRow } from "./window-manager-shortcut-row";

const GROUPS: ReadonlyArray<{ title: string; ids: readonly WindowManagerActionId[] }> = [
  {
    title: "Window",
    ids: ["window.close", "window.minimize", "window.zoom", "window.toggle_floating"],
  },
  {
    title: "Tiling",
    ids: [
      "window.tile.left",
      "window.tile.right",
      "window.tile.top",
      "window.tile.bottom",
      "window.tile.top-left",
      "window.tile.top-right",
      "window.tile.bottom-left",
      "window.tile.bottom-right",
    ],
  },
  {
    title: "Focus",
    ids: ["window.focus.left", "window.focus.right", "window.focus.up", "window.focus.down"],
  },
  {
    title: "Desktops",
    ids: ["desktop.switch.previous", "desktop.switch.next", "desktop.overview"],
  },
  {
    title: "Layout",
    ids: [
      "layout.arrange.two-up",
      "layout.arrange.grid",
      "layout.balance",
      "layout.undo",
      "layout.redo",
    ],
  },
];

/**
 * Every action the daemon accepts a chord for, grouped by what it does. Only the
 * ones changed here are stored, so the table shows the shipped default and a way
 * back to it rather than a JSON object holding all twenty-four.
 */
export function WindowManagerShortcutTable({
  overrides,
  recorder,
}: {
  overrides: Readonly<Record<string, string>>;
  recorder: ShortcutRecorderModel;
}) {
  const [open, setOpen] = useState<ReadonlySet<string>>(new Set(["Window", "Tiling"]));
  const actions = new Map(
    resolveWindowManagerActions(overrides).map(action => [action.id, action])
  );

  return (
    <div data-testid="window-manager-shortcut-table">
      <span aria-live="polite" className="sr-only">
        {recorder.announcement}
      </span>
      {GROUPS.map(group => {
        const expanded = open.has(group.title);
        const changed = group.ids.filter(id => overrides[id] != null).length;
        const groupBlocked = group.ids.filter(id => recorder.blocked.has(id));
        const groupShadowed = group.ids.filter(id => recorder.shadowed.has(id));
        return (
          <section className="border-t border-line-soft first:border-t-0" key={group.title}>
            <button
              aria-expanded={expanded}
              className="eyebrow flex w-full items-center gap-2 px-4 py-2.5 text-left text-fg-strong transition-colors duration-base ease-out hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-ring"
              type="button"
              onClick={() =>
                setOpen(current => {
                  const next = new Set(current);
                  if (next.has(group.title)) next.delete(group.title);
                  else next.add(group.title);
                  return next;
                })
              }
            >
              <ChevronRight
                aria-hidden="true"
                className={cn(
                  "size-3 text-faint transition-transform duration-base ease-out",
                  expanded && "rotate-90"
                )}
              />
              {group.title}
              <span className="ml-auto font-mono text-badge tracking-normal normal-case text-faint">
                {group.ids.length}
                {changed > 0 ? ` · ${changed} changed` : ""}
              </span>
            </button>
            {expanded ? (
              <div className="pb-1">
                {group.ids.map(id => {
                  const action = actions.get(id);
                  if (!action) return null;
                  return (
                    <WindowManagerShortcutRow
                      action={action as ResolvedWindowManagerAction}
                      blocked={recorder.blocked.has(id)}
                      key={id}
                      overridden={overrides[id] != null}
                      recording={recorder.recording === id}
                      shadowed={recorder.shadowed.has(id)}
                      onRecord={() => recorder.start(id)}
                      onReset={() => recorder.reset(id)}
                    />
                  );
                })}
                {groupBlocked.length > 0 ? (
                  <p className="px-4 pt-1 pb-2 text-form-hint text-danger" role="status">
                    {groupBlocked.length} shortcut{groupBlocked.length === 1 ? "" : "s"} here share
                    a chord with another one you changed. The daemon refuses a duplicate, so this
                    cannot be saved until one of them moves.
                  </p>
                ) : null}
                {groupShadowed.length > 0 && groupBlocked.length === 0 ? (
                  <p className="px-4 pt-1 pb-2 text-form-hint text-warning" role="status">
                    {groupShadowed.length} shortcut{groupShadowed.length === 1 ? "" : "s"} here land
                    on a chord another action ships with. Both are stored; the first one in this
                    list wins.
                  </p>
                ) : null}
              </div>
            ) : null}
          </section>
        );
      })}
    </div>
  );
}

import { ChevronRight } from "lucide-react";
import { useState } from "react";

import { cn } from "@compozy/ui";

import type { ShortcutRecorderModel } from "../../hooks/use-window-manager-shortcut-recorder";
import { WindowManagerShortcutFamilyRow } from "./window-manager-shortcut-family-row";
import { WindowManagerShortcutRow } from "./window-manager-shortcut-row";
import {
  effectiveShortcutMap,
  registryShortcutActions,
  resolveWindowManagerActions,
  shortcutLabel,
  usePaletteRegistry,
  type ResolvedWindowManagerAction,
  type ShortcutConflict,
  type ShortcutMap,
} from "@/systems/os";

import { SHORTCUT_SOURCE_ALL, ShortcutSourceFilter } from "./shortcut-source-filter";

/**
 * Curated reading order for the core areas. Sections the registry carries but
 * this list does not name — an extension's own group — render after these
 * rather than vanishing.
 */
const SECTION_ORDER: readonly string[] = [
  "Shell",
  "Window",
  "Tiling",
  "Tabs",
  "Sessions",
  "Desktops",
  "Workspaces",
  "Layout",
];
const TAB_JUMP_IDS = Array.from({ length: 8 }, (_, index) => `window.tab.jump.${index + 1}`);
const DESKTOP_SWITCH_IDS = Array.from({ length: 9 }, (_, index) => `desktop.switch.${index + 1}`);

function sourceLabel(source: string): string {
  return source === "core" ? "Core" : source.replace(/^ext\./, "");
}

function conflictCopy(conflict: ShortcutConflict | undefined, actionId: string) {
  if (!conflict) return {};
  if (conflict.kind === "shadowed") {
    return {
      shadowedMessage: `Shadowed while ${conflict.winner?.surface ?? "a local surface"} is focused — ${conflict.winner?.label ?? "its local action"} wins there. Works everywhere else.`,
    };
  }
  const other = conflict.actionIds.find(id => id !== actionId);
  const digit = other?.match(/\.(\d)$/)?.[1];
  return {
    blockedMessage: `Blocked — ${shortcutLabel(conflict.chord)} is${digit ? ` member ${digit} of` : " bound to"} ${other ?? "another action"}. Free it or pick another chord.`,
  };
}

export function WindowManagerShortcutTable({
  defaults,
  overrides,
  recorder,
}: {
  defaults: ShortcutMap;
  overrides: ShortcutMap;
  recorder: ShortcutRecorderModel;
}) {
  const [open, setOpen] = useState<ReadonlySet<string>>(
    new Set(["Shell", "Window", "Tabs", "Sessions"])
  );
  const [expandedFamilies, setExpandedFamilies] = useState<ReadonlySet<string>>(new Set());
  const [source, setSource] = useState<string>(SHORTCUT_SOURCE_ALL);
  const registry = usePaletteRegistry();
  const effective = effectiveShortcutMap(defaults, overrides);
  // The whole registry, not a closed list: every command is bindable, so every
  // command has a row here (ADR-001, US-001.AC-3).
  const sourceById = new Map(registry.commands.map(command => [command.id, command.source]));
  const actions = resolveWindowManagerActions(effective, registryShortcutActions(registry)).filter(
    action => source === SHORTCUT_SOURCE_ALL || sourceById.get(action.id) === source
  );
  const actionById = new Map(actions.map(action => [action.id, action]));
  const sourceCounts = new Map<string, number>();
  for (const command of registry.commands) {
    sourceCounts.set(command.source, (sourceCounts.get(command.source) ?? 0) + 1);
  }
  const sourceOptions = [...sourceCounts.entries()]
    .map(([id, count]) => ({ id, label: sourceLabel(id), count }))
    .sort((left, right) => left.label.localeCompare(right.label));
  const sections = [
    ...SECTION_ORDER,
    ...[...new Set(actions.map(action => action.section))].filter(
      section => !SECTION_ORDER.includes(section)
    ),
  ];
  const groups: Array<{ title: string; actions: typeof actions }> = [];
  for (const title of sections) {
    const sectionActions = actions.filter(action => action.section === title);
    if (sectionActions.length > 0) groups.push({ title, actions: sectionActions });
  }

  const renderAction = (action: ResolvedWindowManagerAction) => {
    const conflict = recorder.conflicts.find(entry => entry.actionIds.includes(action.id));
    const familyOverridden = action.id.startsWith("window.tab.jump.")
      ? overrides["window.tab.jump"] !== undefined
      : action.id.startsWith("desktop.switch.")
        ? overrides["desktop.switch"] !== undefined
        : false;
    return (
      <WindowManagerShortcutRow
        action={action}
        key={action.id}
        overridden={familyOverridden || overrides[action.id] !== undefined}
        recorder={recorder}
        {...conflictCopy(conflict, action.id)}
      />
    );
  };

  const renderFamily = (
    familyId: "desktop.switch" | "window.tab.jump",
    label: string,
    memberIds: readonly string[]
  ) => {
    const manuallyExpanded = expandedFamilies.has(familyId);
    const partial = memberIds.some(id => overrides[id] !== undefined);
    const expanded = manuallyExpanded || partial;
    const renderedActions: ReturnType<typeof renderAction>[] = [];
    if (expanded) {
      for (const id of memberIds) {
        const action = actionById.get(id);
        if (action) renderedActions.push(renderAction(action));
      }
    }
    return (
      <div key={familyId}>
        <WindowManagerShortcutFamilyRow
          conflicts={recorder.conflicts}
          defaults={defaults}
          effective={effective}
          expanded={expanded}
          familyId={familyId}
          label={label}
          memberIds={memberIds}
          onExpand={() =>
            setExpandedFamilies(current => {
              const next = new Set(current);
              if (!next.delete(familyId)) next.add(familyId);
              return next;
            })
          }
        />
        {renderedActions}
      </div>
    );
  };

  return (
    <div data-testid="window-manager-shortcut-table">
      <span aria-live="polite" className="sr-only">
        {recorder.announcement}
      </span>
      <ShortcutSourceFilter onSelect={setSource} options={sourceOptions} selected={source} />
      {groups.map(group => {
        const expanded = open.has(group.title);
        const changed = group.actions.filter(action => overrides[action.id] !== undefined).length;
        return (
          <section className="border-t border-line-soft first:border-t-0" key={group.title}>
            <button
              aria-expanded={expanded}
              className="eyebrow flex w-full items-center gap-2 px-4 py-2.5 text-left text-fg-strong transition-colors duration-base ease-out hover:bg-row-hover focus-visible:outline-none focus-visible:shadow-focus-ring"
              type="button"
              onClick={() =>
                setOpen(current => {
                  const next = new Set(current);
                  if (!next.delete(group.title)) next.add(group.title);
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
              {group.title === "Tabs" ? (
                <span className="rounded-sm bg-accent-tint px-1.5 py-0.5 text-badge text-accent-strong">
                  new
                </span>
              ) : null}
              <span className="ml-auto font-mono text-badge tracking-normal normal-case text-faint">
                {group.actions.length}
                {changed > 0 ? ` · ${changed} changed` : ""}
              </span>
            </button>
            {expanded ? (
              <div className="pb-1">
                {group.actions.map(action => {
                  if (action.id === "window.tab.jump.1") {
                    return renderFamily("window.tab.jump", "Jump to tab", TAB_JUMP_IDS);
                  }
                  if (action.id.startsWith("window.tab.jump.")) return null;
                  if (action.id === "desktop.switch.1") {
                    return renderFamily("desktop.switch", "Switch desktop", DESKTOP_SWITCH_IDS);
                  }
                  if (/^desktop\.switch\.\d$/.test(action.id)) return null;
                  return renderAction(action);
                })}
              </div>
            ) : null}
          </section>
        );
      })}
    </div>
  );
}

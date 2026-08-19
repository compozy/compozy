import { LockKeyhole } from "lucide-react";

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Eyebrow,
  Kbd,
  Pill,
  dialogShellClass,
} from "@compozy/ui";

import { usePaletteRegistry } from "../hooks/use-palette-registry";
import { useDesktop } from "../hooks/use-desktop";
import { useOsShell } from "../hooks/use-os-shell";
import { registryShortcutActions } from "../lib/cmd-palette-shortcut-actions";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import {
  deriveShortcutCheatsheet,
  primaryShortcutModifier,
  shortcutLabel,
} from "../lib/window-manager-shortcuts";
import { ShortcutBindingKeys } from "./shortcut-binding-keys";

export interface OsShortcutsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Curated reading order. A section the registry carries but this list does not
 * name still renders — appended after these — so an extension's group can never
 * be silently dropped from the sheet.
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

const SURFACE_LOCAL_ROWS = [
  { id: "permission.decide", label: "Approve / deny permission", keys: ["1", "4"] },
  { id: "composer.steer", label: "Steer the agent", keys: ["meta+Enter"] },
  { id: "composer.focus", label: "Focus composer", keys: ["/"] },
] as const;

/** Live daemon keymap plus the current surface's fixed, read-only controls. */
export function OsShortcutsDialog({ open, onOpenChange }: OsShortcutsDialogProps) {
  const { coordinator } = useOsShell();
  const registry = usePaletteRegistry();
  const windowManagerConfig = useDesktop(state => state.windowManagerConfig);
  const canOpenApps = useDesktop(windowManagerCommandsAvailable);
  const overrides = windowManagerConfig?.shortcuts ?? {};
  const rows = deriveShortcutCheatsheet(
    windowManagerConfig?.effectiveShortcuts ?? {},
    overrides,
    registryShortcutActions(registry)
  );
  const primaryModifier = primaryShortcutModifier(
    typeof navigator === "undefined" ? "" : navigator.platform
  );
  const sections = [
    ...SECTION_ORDER,
    ...[...new Set(rows.map(row => row.section))].filter(
      section => !SECTION_ORDER.includes(section)
    ),
  ];
  const groups: Array<{ title: string; rows: typeof rows }> = [];
  for (const title of sections) {
    const sectionRows = rows.filter(row => row.section === title);
    if (sectionRows.length > 0) groups.push({ title, rows: sectionRows });
  }

  const openLayoutSettings = () => {
    if (!canOpenApps) return;
    onOpenChange(false);
    void coordinator.userOpen({
      app: "settings",
      route: { pathname: "/settings/layouts", search: {} },
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        unframed
        className={`flex flex-col ${dialogShellClass("lg")}`}
        data-testid="os-shortcuts-dialog"
      >
        <DialogHeader variant="ruled">
          <div className="flex items-center gap-2">
            <DialogTitle>Keyboard shortcuts</DialogTitle>
            <Pill size="xs" tone="neutral">
              Live keymap
            </Pill>
          </div>
          <DialogDescription className="sr-only">
            Defaults, overrides, and surface-local keyboard controls in effect now.
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          <div className="grid items-start gap-x-8 gap-y-5 sm:grid-cols-2">
            {groups.map(group => (
              <section className="min-w-0" key={group.title}>
                <Eyebrow className="mb-1.5 text-subtle">{group.title}</Eyebrow>
                <dl className="overflow-hidden rounded-md border border-line bg-canvas-soft">
                  {group.rows.map(row => (
                    <div
                      className="flex min-h-8 items-center justify-between gap-5 border-t border-line-soft px-3 py-1.5 first:border-t-0"
                      data-overridden={row.overridden ? "true" : undefined}
                      data-testid={`os-shortcut-row-${row.id}`}
                      key={row.id}
                    >
                      <dt className="min-w-0 truncate text-small-body text-fg">
                        {row.label}
                        {row.overridden ? (
                          <span className="ml-1.5 text-badge text-accent-strong">override</span>
                        ) : null}
                      </dt>
                      <dd className="shrink-0">
                        <ShortcutBindingKeys bindings={row.bindings} overridden={row.overridden} />
                      </dd>
                    </div>
                  ))}
                </dl>
              </section>
            ))}
            <section className="min-w-0 sm:col-span-2">
              <Eyebrow className="mb-1.5 inline-flex items-center gap-1.5 text-subtle">
                <LockKeyhole aria-hidden="true" className="size-3" />
                In this surface — session window
              </Eyebrow>
              <dl className="grid overflow-hidden rounded-md border border-line bg-canvas-soft sm:grid-cols-3">
                {SURFACE_LOCAL_ROWS.map(row => (
                  <div
                    className="flex min-h-9 items-center justify-between gap-3 border-t border-line-soft px-3 py-1.5 first:border-t-0 sm:border-t-0 sm:border-l sm:first:border-l-0"
                    data-testid={`os-shortcut-local-${row.id}`}
                    key={row.id}
                  >
                    <dt className="text-small-body text-fg">{row.label}</dt>
                    <dd className="inline-flex gap-1">
                      {row.keys.map(key => (
                        <Kbd className="border-dashed border-line-strong bg-transparent" key={key}>
                          {key.includes("+") ? shortcutLabel(key, primaryModifier) : key}
                        </Kbd>
                      ))}
                    </dd>
                  </div>
                ))}
              </dl>
            </section>
          </div>
        </div>
        <DialogFooter className="justify-between" variant="ruled">
          <span className="text-form-hint text-muted">Surface-local rows are read-only.</span>
          <Button
            data-testid="os-shortcuts-edit"
            disabled={!canOpenApps}
            type="button"
            variant="outline"
            onClick={openLayoutSettings}
          >
            Edit shortcuts in Layouts
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

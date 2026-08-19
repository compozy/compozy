import { LockKeyhole } from "lucide-react";
import { Fragment } from "react";

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
import { groupShortcutRowsBySource } from "../lib/shortcut-source-groups";
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
  // Grouped by contributing source, then by area. The source band is drawn only
  // when something beyond core contributes, so a core-only install keeps the
  // plain area grid instead of gaining a header that names nothing.
  const sourceGroups = groupShortcutRowsBySource(rows);
  const showSourceBands = sourceGroups.length > 1;

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
            {sourceGroups.map(sourceGroup => (
              <Fragment key={sourceGroup.source}>
                {showSourceBands ? (
                  <Eyebrow
                    className="text-faint sm:col-span-2"
                    data-testid={`os-shortcut-source-${sourceGroup.source}`}
                  >
                    {sourceGroup.label}
                  </Eyebrow>
                ) : null}
                {sourceGroup.sections.map(group => (
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
                            {row.alias ? <span className="text-muted"> ({row.alias})</span> : null}
                            {row.overridden ? (
                              <span className="ml-1.5 text-badge text-accent-strong">override</span>
                            ) : null}
                          </dt>
                          <dd className="shrink-0">
                            <ShortcutBindingKeys
                              bindings={row.bindings}
                              overridden={row.overridden}
                            />
                          </dd>
                        </div>
                      ))}
                    </dl>
                  </section>
                ))}
              </Fragment>
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

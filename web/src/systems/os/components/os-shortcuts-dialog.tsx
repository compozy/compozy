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
} from "@agh/ui";

import { useDesktop } from "../hooks/use-desktop";
import { useOsShell } from "../hooks/use-os-shell";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { resolveWindowManagerActions } from "../lib/window-manager-shortcuts";

export interface OsShortcutsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

interface ShortcutRow {
  id: string;
  label: string;
  keys: string | null;
}

/** Shell chords are bound in `useOsShortcuts`, not in the WM registry. */
const SHELL_ROWS: readonly ShortcutRow[] = [
  { id: "palette.open", label: "Command palette", keys: "⌘K" },
  { id: "session.new", label: "New session", keys: "⌘N" },
  { id: "overlay.close", label: "Close overlay", keys: "Esc" },
];

const SECTIONS = [
  { title: "Window", match: (id: string) => id.startsWith("window.") },
  { title: "Layout", match: (id: string) => id.startsWith("layout.") },
  { title: "Desktops", match: (id: string) => id.startsWith("desktop.") },
] as const;

/**
 * The keyboard reference. Every registry action is listed, including the ones
 * with no bound chord (em dash), so the surface is complete rather than
 * flattering. Overrides are edited in Settings → Layouts, which the footer
 * links to instead of duplicating an editor here.
 */
export function OsShortcutsDialog({ open, onOpenChange }: OsShortcutsDialogProps) {
  const { coordinator } = useOsShell();
  const windowManagerConfig = useDesktop(state => state.windowManagerConfig);
  const canOpenApps = useDesktop(windowManagerCommandsAvailable);
  const actions = resolveWindowManagerActions(windowManagerConfig?.shortcuts ?? {});
  const groups = [
    { title: "Shell", rows: SHELL_ROWS },
    ...SECTIONS.map(section => ({
      title: section.title,
      rows: actions.flatMap(action =>
        section.match(action.id)
          ? [{ id: action.id, label: action.label, keys: action.shortcutLabel }]
          : []
      ),
    })),
  ].filter(group => group.rows.length > 0);

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
        data-testid="os-shortcuts-dialog"
        className="flex max-h-[min(var(--height-modal-md),80vh)] w-full flex-col sm:max-w-(--width-modal-sm)"
      >
        <DialogHeader variant="ruled">
          <DialogTitle>Keyboard shortcuts</DialogTitle>
          <DialogDescription>
            Chords bound right now. Actions without one are listed with a dash.
          </DialogDescription>
        </DialogHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 py-4">
          {groups.map(group => (
            <section key={group.title} className="flex flex-col gap-1">
              <Eyebrow className="text-subtle">{group.title}</Eyebrow>
              <dl className="flex flex-col">
                {group.rows.map(row => (
                  <div
                    key={row.id}
                    data-testid={`os-shortcut-row-${row.id}`}
                    className="flex min-h-7 items-center justify-between gap-6 border-b border-line-soft py-1 last:border-b-0"
                  >
                    <dt className="min-w-0 truncate text-small-body text-fg">{row.label}</dt>
                    <dd className="shrink-0">
                      {row.keys ? (
                        <Kbd>{row.keys}</Kbd>
                      ) : (
                        <span className="font-mono text-micro text-faint">—</span>
                      )}
                    </dd>
                  </div>
                ))}
              </dl>
            </section>
          ))}
        </div>
        <DialogFooter variant="ruled">
          <Button
            type="button"
            variant="outline"
            disabled={!canOpenApps}
            data-testid="os-shortcuts-edit"
            onClick={openLayoutSettings}
          >
            Edit shortcuts in Layouts
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

import { TriangleAlert } from "lucide-react";

import { ShortcutBindingKeys } from "@/systems/os";
import { SettingsGroup, type SettingsHooksExtensionsInstalled } from "@/systems/settings";
import { MonoId, Pill, cn } from "@compozy/ui";

interface ExtensionPalettePanelProps {
  extension: SettingsHooksExtensionsInstalled;
}

function extensionDisplayName(name: string): string {
  return name.length > 0 ? `${name[0].toUpperCase()}${name.slice(1)}` : name;
}

function contributionReason(extension: SettingsHooksExtensionsInstalled): string | null {
  const palette = extension.palette;
  if (!palette) return null;
  const reason = [...palette.commands, ...palette.views].find(item => item.reason)?.reason;
  return reason ?? null;
}

export function ExtensionPalettePanel({ extension }: ExtensionPalettePanelProps) {
  const palette = extension.palette;
  if (!palette || (palette.commands.length === 0 && palette.views.length === 0)) return null;

  const reason = contributionReason(extension);
  const unavailable = reason !== null;
  return (
    <section
      aria-label={`${extension.name} palette contributions`}
      className={cn(
        "overflow-hidden rounded-lg border border-line bg-canvas-soft",
        unavailable && "text-muted"
      )}
      data-testid={`extension-palette-${extension.name}`}
    >
      <header className="flex items-start justify-between gap-3 border-b border-line-soft px-4 py-3">
        <div className="min-w-0">
          <h3 className="truncate text-ws-name font-semibold tracking-tight text-fg-strong">
            {extensionDisplayName(extension.name)}
          </h3>
          <MonoId
            className="mt-0.5 block text-faint"
            preserveCase
            value={`ext.${extension.name}`}
          />
        </div>
      </header>

      {reason ? (
        <div
          className="flex items-center gap-1.5 border-b border-line-soft bg-warning-tint px-4 py-2 text-form-hint text-warning"
          role="status"
        >
          <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0" />
          <span>{reason}</span>
        </div>
      ) : null}

      <div className={cn(unavailable && "opacity-55")}>
        {palette.commands.map(command => (
          <div
            className="flex min-h-10 items-center justify-between gap-4 border-b border-line-soft px-4 py-2 last:border-b-0"
            data-testid={`extension-palette-command-${command.id}`}
            key={command.id}
          >
            <span className="min-w-0 truncate text-small-body text-fg">{command.title}</span>
            {command.default_dormant ? (
              <div className="flex min-w-0 flex-col items-end gap-0.5 text-right">
                <span className="inline-flex items-center gap-1 font-mono text-badge text-warning">
                  <TriangleAlert aria-hidden="true" className="size-3" />
                  dormant
                </span>
                <span className="text-form-hint text-muted">
                  default unavailable — conflicts with {command.conflict_with}
                </span>
              </div>
            ) : command.bindings.length > 0 ? (
              <ShortcutBindingKeys bindings={command.bindings} compact />
            ) : (
              <span className="font-mono text-micro text-faint">unbound</span>
            )}
          </div>
        ))}
      </div>

      {palette.views.length > 0 ? (
        <>
          <div className="border-y border-line-soft bg-canvas px-4 py-2 text-form-label font-semibold text-muted">
            Views
          </div>
          <div className={cn(unavailable && "opacity-55")}>
            {palette.views.map(view => (
              <div
                className="flex min-h-10 items-center justify-between gap-4 border-b border-line-soft px-4 py-2 last:border-b-0"
                data-testid={`extension-palette-view-${view.id}`}
                key={view.id}
              >
                <span className="min-w-0 truncate text-small-body text-fg">{view.title}</span>
                <Pill size="xs">view</Pill>
              </div>
            ))}
          </div>
        </>
      ) : null}
    </section>
  );
}

export function ExtensionPalettePanels({
  extensions,
}: {
  extensions: readonly SettingsHooksExtensionsInstalled[];
}) {
  const contributors = extensions.filter(extension => extension.palette != null);
  if (contributors.length === 0) return null;
  return (
    <SettingsGroup data-testid="settings-page-extensions-palette-section" title="Palette">
      <div className="flex flex-col gap-3 p-3">
        {contributors.map(extension => (
          <ExtensionPalettePanel extension={extension} key={extension.name} />
        ))}
      </div>
    </SettingsGroup>
  );
}

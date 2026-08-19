import { AlertTriangle, Check, Clipboard, RotateCcw } from "lucide-react";
import { useState } from "react";

import { Button, Pill } from "@compozy/ui";

import { notifyUser } from "@/lib/user-feedback";
import {
  ShortcutBindingKeys,
  registryShortcutActions,
  usePaletteRegistry,
  type ShortcutMap,
} from "@/systems/os";
import {
  applyTerminalShortcutPreset,
  previewTerminalShortcutPreset,
  revertTerminalShortcutPreset,
  TERMINAL_SHORTCUT_PRESET,
  TERMINAL_SHORTCUT_PRESET_TOML,
  type ShortcutPresetRevertToken,
} from "../../lib/window-manager-shortcut-preset";

const HAZARD_COPY = {
  altgr: "Control+Alt can alias AltGr on some layouts.",
  "browser-tabs": "Control+digits can be reserved for browser tabs.",
  "displaced-default": "Replaces the current effective binding.",
} as const;

async function copyTerminalPresetToml() {
  if (!navigator.clipboard) {
    notifyUser({ message: "Clipboard is unavailable in this browser.", tone: "error" });
    return;
  }
  try {
    await navigator.clipboard.writeText(TERMINAL_SHORTCUT_PRESET_TOML);
    notifyUser({ message: "Terminal preset copied.", tone: "success" });
  } catch {
    notifyUser({ message: "Could not copy the Terminal preset.", tone: "error" });
  }
}

export function ShortcutPresetCard({
  defaults,
  overrides,
  onChange,
}: {
  defaults: ShortcutMap;
  overrides: ShortcutMap;
  onChange: (next: ShortcutMap) => void;
}) {
  const [previewOpen, setPreviewOpen] = useState(false);
  const [revertToken, setRevertToken] = useState<ShortcutPresetRevertToken | null>(null);
  const preview = previewTerminalShortcutPreset(
    overrides,
    defaults,
    registryShortcutActions(usePaletteRegistry())
  );
  const applied = Object.entries(TERMINAL_SHORTCUT_PRESET).every(
    ([actionId, binding]) => JSON.stringify(overrides[actionId] ?? []) === JSON.stringify(binding)
  );
  const blockingConflicts = preview.flatMap(change =>
    change.conflicts.filter(conflict => conflict.kind === "blocked")
  );

  const apply = () => {
    if (blockingConflicts.length > 0) return;
    const result = applyTerminalShortcutPreset(overrides);
    if (!result.changed) return;
    setRevertToken(result.revert);
    onChange(result.overrides);
    setPreviewOpen(false);
  };

  const revert = () => {
    if (revertToken === null) return;
    onChange(revertTerminalShortcutPreset(overrides, revertToken));
    setRevertToken(null);
  };

  return (
    <section className="border-b border-line" data-testid="terminal-shortcut-preset">
      <div className="flex flex-wrap items-center gap-3 px-4 py-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h3 className="text-small-body font-semibold text-fg-strong">Terminal preset</h3>
            {applied ? (
              <Pill size="xs" tone="success">
                <Check aria-hidden="true" className="size-3" /> Applied
              </Pill>
            ) : null}
          </div>
        </div>
        <Button
          size="sm"
          type="button"
          variant="ghost"
          onClick={() => void copyTerminalPresetToml()}
        >
          <Clipboard aria-hidden="true" className="size-3.5" /> Copy as TOML
        </Button>
        {revertToken ? (
          <Button size="sm" type="button" variant="outline" onClick={revert}>
            <RotateCcw aria-hidden="true" className="size-3.5" /> Revert
          </Button>
        ) : (
          <Button
            size="sm"
            type="button"
            variant={previewOpen ? "outline" : "default"}
            onClick={() => setPreviewOpen(open => !open)}
          >
            {previewOpen ? "Cancel" : "Preview"}
          </Button>
        )}
      </div>
      {previewOpen ? (
        <div className="border-t border-line-soft bg-canvas px-4 py-3">
          <div className="max-h-64 overflow-y-auto rounded-md border border-line">
            {preview.map(change => (
              <div
                className="grid min-h-10 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 border-t border-line-soft px-3 py-2 first:border-t-0"
                key={change.actionId}
              >
                <span className="truncate font-mono text-mono-id text-fg">{change.actionId}</span>
                <span className="flex items-center gap-1 opacity-65 line-through">
                  <ShortcutBindingKeys bindings={change.before} compact />
                </span>
                <ShortcutBindingKeys bindings={change.after} overridden compact />
                {[
                  ...change.hazards.map(hazard => HAZARD_COPY[hazard]),
                  ...change.conflicts.map(conflict =>
                    conflict.kind === "blocked"
                      ? `${conflict.chord} also binds ${conflict.actionIds.join(" and ")}.`
                      : `${conflict.chord} is owned by ${conflict.winner?.surface ?? "this surface"}.`
                  ),
                ].map(message => (
                  <span
                    className="col-span-3 inline-flex items-center gap-1 text-form-hint text-warning"
                    key={message}
                  >
                    <AlertTriangle aria-hidden="true" className="size-3" /> {message}
                  </span>
                ))}
              </div>
            ))}
          </div>
          <div className="mt-3 flex justify-end">
            <Button
              disabled={preview.length === 0 || blockingConflicts.length > 0}
              type="button"
              onClick={apply}
            >
              Apply preset
            </Button>
          </div>
        </div>
      ) : null}
    </section>
  );
}

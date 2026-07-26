import { RotateCcw } from "lucide-react";

import { Button, Kbd, KbdGroup, cn } from "@agh/ui";

import type { ResolvedWindowManagerAction, WindowPlacementId } from "@/systems/os";

import { WindowTilingDiagram } from "./window-manager-tiling-diagrams";

interface WindowManagerShortcutRowProps {
  action: ResolvedWindowManagerAction;
  overridden: boolean;
  recording: boolean;
  blocked: boolean;
  shadowed: boolean;
  onRecord: () => void;
  onReset: () => void;
}

const KEY_SYMBOL: Record<string, string> = {
  meta: "⌘",
  control: "⌃",
  alt: "⌥",
  shift: "⇧",
  ArrowLeft: "←",
  ArrowRight: "→",
  ArrowUp: "↑",
  ArrowDown: "↓",
  BracketLeft: "[",
  BracketRight: "]",
  Space: "␣",
  Enter: "⏎",
  Escape: "⎋",
  Backspace: "⌫",
  Delete: "⌦",
  Tab: "⇥",
};

function placementOf(actionId: string): WindowPlacementId | null {
  const suffix = actionId.startsWith("window.tile.") ? actionId.slice("window.tile.".length) : null;
  return suffix as WindowPlacementId | null;
}

function chordKeys(chord: string): string[] {
  return chord.split("+").map(token => {
    if (KEY_SYMBOL[token]) return KEY_SYMBOL[token];
    if (/^Key[A-Z]$/.test(token)) return token.slice(3);
    if (/^Digit\d$/.test(token)) return token.slice(5);
    return token;
  });
}

/** One action, the shape it produces, and the keys that trigger it. */
export function WindowManagerShortcutRow({
  action,
  overridden,
  recording,
  blocked,
  shadowed,
  onRecord,
  onReset,
}: WindowManagerShortcutRowProps) {
  const placement = placementOf(action.id);
  const chord = action.chord?.canonical ?? null;

  return (
    <div
      className="group/row flex min-h-8.5 items-center gap-2.5 px-4 py-1 transition-colors duration-base ease-out hover:bg-row-hover"
      data-testid={`window-manager-shortcut-${action.id}`}
    >
      <span className="flex w-6 shrink-0 justify-center">
        {placement ? <WindowTilingDiagram placement={placement} /> : null}
      </span>
      <span className="min-w-0 truncate text-small-body text-fg">{action.label}</span>
      <span className="flex-1" />
      <span className="hidden shrink-0 font-mono text-mono-id text-faint sm:inline">
        {action.id}
      </span>
      <button
        aria-label={`${action.label} shortcut`}
        className={cn(
          "inline-flex h-6 shrink-0 items-center gap-0.5 rounded-sm border px-2",
          "border-line bg-btn-default-fill transition-colors duration-base ease-out",
          "hover:border-line-strong hover:bg-btn-default-hover",
          "focus-visible:outline-none focus-visible:shadow-focus-ring",
          overridden && "border-accent-dim bg-accent-tint",
          shadowed && !blocked && "border-warning/40 bg-warning-tint",
          blocked && "border-danger bg-danger-tint",
          recording && "border-accent bg-accent-tint-strong"
        )}
        data-state={
          recording ? "recording" : blocked ? "conflict" : overridden ? "custom" : "default"
        }
        type="button"
        onClick={onRecord}
      >
        {recording ? (
          <span className="text-form-label text-accent-strong">Press keys…</span>
        ) : chord === null ? (
          <span className="text-form-label text-subtle">Set shortcut</span>
        ) : (
          <KbdGroup>
            {chordKeys(chord).map(key => (
              <Kbd key={key}>{key}</Kbd>
            ))}
          </KbdGroup>
        )}
      </button>
      <Button
        aria-label={`Reset ${action.label} to its default`}
        className={cn(!overridden && "invisible")}
        disabled={!overridden}
        size="icon-xs"
        type="button"
        variant="ghost"
        onClick={onReset}
      >
        <RotateCcw aria-hidden="true" className="size-3" />
      </Button>
    </div>
  );
}

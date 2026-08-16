import { Plus, RotateCcw } from "lucide-react";

import { Button, cn } from "@compozy/ui";

import type { ShortcutRecorderModel } from "../../hooks/use-window-manager-shortcut-recorder";
import { WindowTilingDiagram } from "./window-manager-tiling-diagrams";
import {
  ShortcutBindingKeys,
  type ResolvedWindowManagerAction,
  type WindowPlacementId,
} from "@/systems/os";

interface WindowManagerShortcutRowProps {
  action: ResolvedWindowManagerAction;
  overridden: boolean;
  blockedMessage?: string;
  shadowedMessage?: string;
  recorder: ShortcutRecorderModel;
}

function placementOf(actionId: string): WindowPlacementId | null {
  const suffix = actionId.startsWith("window.tile.") ? actionId.slice("window.tile.".length) : null;
  return suffix as WindowPlacementId | null;
}

/** One action, every effective alternate, and its recorder controls. */
export function WindowManagerShortcutRow({
  action,
  overridden,
  blockedMessage,
  shadowedMessage,
  recorder,
}: WindowManagerShortcutRowProps) {
  const placement = placementOf(action.id);
  const recording = recorder.recording === action.id;
  const blocked = blockedMessage !== undefined;
  const shadowed = shadowedMessage !== undefined;

  return (
    <div
      className={cn(
        "group/row border-t border-line-soft first:border-t-0",
        blocked && "bg-danger-tint",
        shadowed && !blocked && "bg-warning-tint"
      )}
      data-testid={`window-manager-shortcut-${action.id}`}
    >
      <div className="flex min-h-setting-row items-center gap-2.5 px-4 py-2">
        <span className="flex w-6 shrink-0 justify-center">
          {placement ? <WindowTilingDiagram placement={placement} /> : null}
        </span>
        <span className="min-w-0 truncate text-small-body text-fg">{action.label}</span>
        <span className="flex-1" />
        <span className="hidden shrink-0 font-mono text-mono-id text-faint lg:inline">
          {action.id}
        </span>
        <button
          aria-label={`${action.label} shortcut`}
          className={cn(
            "inline-flex min-h-7 shrink-0 items-center rounded-sm border px-2",
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
          onClick={() => recorder.start(action.id)}
        >
          {recording ? (
            <span className="text-form-label text-accent-strong">Press keys…</span>
          ) : (
            <ShortcutBindingKeys
              bindings={action.chords.map(chord => chord.canonical)}
              overridden={overridden}
            />
          )}
        </button>
        <Button
          aria-label={`Add an alternate for ${action.label}`}
          size="icon-xs"
          type="button"
          variant="ghost"
          onClick={() => recorder.start(action.id, "alternate")}
        >
          <Plus aria-hidden="true" className="size-3" />
        </Button>
        <Button
          aria-label={`Reset ${action.label} to its default`}
          className={cn(!overridden && "invisible")}
          disabled={!overridden}
          size="icon-xs"
          type="button"
          variant="ghost"
          onClick={() => recorder.reset(action.id)}
        >
          <RotateCcw aria-hidden="true" className="size-3" />
        </Button>
      </div>
      {blockedMessage ? (
        <p className="px-12 pb-2 text-form-hint text-danger" role="status">
          {blockedMessage}
        </p>
      ) : shadowedMessage ? (
        <p className="px-12 pb-2 text-form-hint text-warning" role="status">
          {shadowedMessage}
        </p>
      ) : null}
    </div>
  );
}

import { useState } from "react";

import { Input, cn } from "@agh/ui";

import type { WindowManagerLayoutDesktop } from "../../lib/window-manager-layout-types";

interface LayoutDesktopTabsProps {
  desktops: readonly WindowManagerLayoutDesktop[];
  activeIndex: number;
  onSelect: (index: number) => void;
  onRename: (index: number, name: string) => void;
}

/**
 * Desktops are switched here and renamed here — but not created or removed:
 * those are semantic commands the workspace owns, not edits to a layout
 * document, and the strip says so rather than offering a button that would fail.
 */
export function LayoutDesktopTabs({
  desktops,
  activeIndex,
  onSelect,
  onRename,
}: LayoutDesktopTabsProps) {
  const [renaming, setRenaming] = useState<number | null>(null);

  return (
    <div
      aria-label="Desktops"
      className="flex flex-wrap items-center gap-1 border-b border-line-soft bg-canvas-tint px-2.5 py-2"
      data-testid="layout-desktop-tabs"
      role="tablist"
    >
      {desktops.map((desktop, index) => {
        const active = index === activeIndex;
        if (renaming === index) {
          return (
            <DesktopNameInput
              key={desktop.id}
              name={desktop.name}
              onCommit={name => {
                setRenaming(null);
                if (name !== desktop.name) onRename(index, name);
              }}
            />
          );
        }
        return (
          <button
            aria-selected={active}
            className={cn(
              "inline-flex h-7 items-center gap-1.5 rounded-md border border-transparent px-2.5",
              "text-small-body font-medium text-muted transition-colors duration-base ease-out",
              "hover:bg-row-hover hover:text-fg",
              "focus-visible:outline-none focus-visible:shadow-focus-ring",
              active && "border-line bg-elevated text-fg-strong shadow-highlight"
            )}
            data-testid={`layout-desktop-tab-${desktop.id}`}
            key={desktop.id}
            role="tab"
            type="button"
            onClick={() => (active ? setRenaming(index) : onSelect(index))}
            onDoubleClick={() => setRenaming(index)}
          >
            <span className="font-mono text-badge text-faint">{index + 1}</span>
            <span className="truncate">{desktop.name}</span>
            {desktop.purpose === "focus" ? <span className="eyebrow text-info">Focus</span> : null}
          </button>
        );
      })}
      <span className="flex-1" />
      <span className="text-form-hint text-faint">
        Desktops are created and removed in the workspace, not here
      </span>
    </div>
  );
}

/**
 * Uncontrolled on purpose: the field is mounted only while renaming and the name
 * it starts from is the one on screen, so mirroring the prop into state would
 * add a second copy of the truth for no gain.
 */
function DesktopNameInput({ name, onCommit }: { name: string; onCommit: (name: string) => void }) {
  return (
    <Input
      aria-label="Desktop name"
      autoFocus
      className="h-7 w-32"
      data-testid="layout-desktop-rename"
      defaultValue={name}
      onBlur={event => onCommit(event.target.value.trim() || name)}
      onKeyDown={event => {
        if (event.key === "Enter") {
          event.preventDefault();
          event.currentTarget.blur();
        }
        if (event.key === "Escape") {
          event.preventDefault();
          onCommit(name);
        }
      }}
    />
  );
}

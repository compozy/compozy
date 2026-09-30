import { AppWindow, Plus } from "lucide-react";

import { Button, Card, CardDescription, CardTitle, KindIcon } from "@compozy/ui";

import { OsShortcutChords } from "./os-shortcut-chords";

export interface OsEmptyDesktopProps {
  desktopName: string;
  /** Live palette chord ("⌘K"); omitted until the keymap is known. */
  paletteShortcutLabel: string | null;
  onNewSession: () => void;
}

/**
 * VC-10 — an empty desktop (shell-rail v2 `.empty-card`): identity well, the
 * desktop's name, how to open something, and New session as the one primary.
 */
export function OsEmptyDesktop({
  desktopName,
  paletteShortcutLabel,
  onNewSession,
}: OsEmptyDesktopProps) {
  return (
    <div className="absolute inset-0 grid place-items-center px-6">
      <Card data-testid="os-desk-hint" className="w-95 max-w-full items-start gap-2.5 px-4">
        <KindIcon icon={AppWindow} tone="well" />
        <CardTitle
          role="heading"
          aria-level={2}
          className="mt-1 text-heading font-medium tracking-tight text-fg"
        >
          {desktopName} is empty
        </CardTitle>
        <CardDescription className="mb-1 text-body text-pretty">
          Open an app from the dock
          {paletteShortcutLabel ? (
            <>
              , or press <OsShortcutChords label={paletteShortcutLabel} /> to open anything.
            </>
          ) : (
            "."
          )}
        </CardDescription>
        <Button onClick={onNewSession}>
          <Plus aria-hidden="true" data-icon="inline-start" />
          New session
        </Button>
      </Card>
    </div>
  );
}

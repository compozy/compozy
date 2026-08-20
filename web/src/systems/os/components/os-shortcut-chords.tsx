import { Fragment } from "react";

import { Kbd, KbdGroup } from "@compozy/ui";

/**
 * Renders a " / "-joined chord label from the window-manager shortcut lib
 * ("⌘K / ⌘⇧P") as one Kbd cap per chord. The slash separates caps — it
 * never sits inside one.
 */
export function OsShortcutChords({ label }: { label: string }) {
  const chords = label.split(" / ");
  return (
    <KbdGroup>
      {chords.map((chord, index) => (
        <Fragment key={chord}>
          {index > 0 ? (
            <span aria-hidden="true" className="text-faint">
              /
            </span>
          ) : null}
          <Kbd>{chord}</Kbd>
        </Fragment>
      ))}
    </KbdGroup>
  );
}

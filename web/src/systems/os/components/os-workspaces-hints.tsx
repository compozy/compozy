import { Kbd } from "@compozy/ui";

function Hint({ children }: { children: React.ReactNode }) {
  return <span className="inline-flex items-center gap-1">{children}</span>;
}

export interface OsWorkspacesHintsProps {
  layer: "strip" | "menu";
  /** The focused workspace has arrow-reachable worktree rows. */
  hasMenuRows: boolean;
  /** Empty state renders only the hints that still apply. */
  empty: boolean;
  pickerShortcutLabel: string | null;
  globalScopeShortcutLabel: string | null;
}

/**
 * Bottom-pinned shortcut hints. Every configurable hint comes from the live
 * effective keymap. Hidden on compact (<960px) chrome.
 */
export function OsWorkspacesHints({
  layer,
  hasMenuRows,
  empty,
  pickerShortcutLabel,
  globalScopeShortcutLabel,
}: OsWorkspacesHintsProps) {
  return (
    <p
      data-slot="os-workspaces-hints"
      data-testid="os-workspaces-hints"
      className="pointer-events-none absolute inset-x-4 bottom-6 hidden flex-wrap items-center justify-center gap-2 text-eyebrow text-muted min-[960px]:flex"
    >
      {layer === "menu" ? (
        <>
          <Hint>
            <Kbd>↑</Kbd>
            <Kbd>↓</Kbd> move
          </Hint>
          <Hint>
            <Kbd>↵</Kbd> scope
          </Hint>
          <Hint>
            <Kbd>esc</Kbd> back
          </Hint>
        </>
      ) : (
        <>
          {empty ? null : (
            <>
              <Hint>
                scroll or <Kbd>←</Kbd>
                <Kbd>→</Kbd>
              </Hint>
              <Hint>
                <Kbd>a–z</Kbd> jump
              </Hint>
              {hasMenuRows ? (
                <Hint>
                  <Kbd>↓</Kbd> worktrees
                </Hint>
              ) : null}
              <Hint>
                <Kbd>↵</Kbd> switch
              </Hint>
            </>
          )}
          <Hint>
            <Kbd>esc</Kbd> close
          </Hint>
          {pickerShortcutLabel ? (
            <Hint>
              <Kbd>{pickerShortcutLabel}</Kbd> open
            </Hint>
          ) : null}
          {globalScopeShortcutLabel ? (
            <Hint>
              <Kbd>{globalScopeShortcutLabel}</Kbd> global
            </Hint>
          ) : null}
        </>
      )}
    </p>
  );
}

import { useMemo, useState } from "react";

import {
  cn,
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
  Dialog,
  DialogContent,
  Pill,
  StateGlyph,
  type StateGlyphState,
} from "@compozy/ui";

import { ComputerMetaLabel } from "@/systems/computers/components/computer-chip";
import { COMPUTERS_PRO } from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  DEVBOX_TRANSCRIPT,
  REMOTE_SESSION_ENTRIES,
  type RemoteSessionRowEntry,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { RemoteSessionsWindow } from "../remote-computers-shared-sessions/remote-sessions-window";

function glyphFor(entry: RemoteSessionRowEntry): StateGlyphState {
  if (entry.computerOffline) return "idle";
  if (entry.state === "running") return "running";
  if (entry.state === "attention") return "attention";
  if (entry.state === "done") return "done";
  return "idle";
}

/**
 * Cross-computer session Search (`⌘K`, `_uiux.md` U5):
 * searches across every connected computer plus cached offline sessions,
 * labeling each hit with `<project> · <computer>` provenance.
 */
export function RemoteSearchOverlayScene({ initialQuery = "" }: { initialQuery?: string }) {
  const [query, setQuery] = useState(initialQuery);
  const [selectedId, setSelectedId] = useState<string>("sess-devbox-1");

  const connectedHits = useMemo(
    () => REMOTE_SESSION_ENTRIES.filter(entry => !entry.computerOffline),
    []
  );
  const offlineHits = useMemo(
    () => REMOTE_SESSION_ENTRIES.filter(entry => entry.computerOffline),
    []
  );

  const searchDialog = (
    <Dialog onOpenChange={() => undefined} open>
      <DialogContent
        className="w-full max-w-xl overflow-hidden p-0 text-fg"
        data-testid="remote-cross-computer-search"
        showCloseButton={false}
        unframed
      >
        <Command className="bg-popover">
          <CommandInput
            onValueChange={setQuery}
            placeholder="Search sessions across all computers…"
            value={query}
          />
          <CommandList className="max-h-96">
            <CommandEmpty>No sessions match “{query}”.</CommandEmpty>

            <CommandGroup heading="Sessions across computers">
              {connectedHits.map(entry => (
                <CommandItem
                  className="items-center gap-2.5 py-2.5"
                  key={entry.id}
                  onSelect={() => setSelectedId(entry.id)}
                  value={`${entry.title} ${entry.project} ${entry.computerName} ${entry.model}`}
                >
                  <StateGlyph size="sm" state={glyphFor(entry)} />
                  <span className="min-w-0 flex-1 truncate text-small-body font-medium text-fg">
                    {entry.title}
                  </span>
                  <span className="flex shrink-0 items-center gap-1.5 text-micro text-muted-foreground">
                    <WorkspaceBadge identity={{ kind: "project", name: entry.project }} />
                    <span aria-hidden="true" className="text-faint">
                      ·
                    </span>
                    {entry.isLocal ? (
                      <span className="text-subtle">This computer</span>
                    ) : (
                      <ComputerMetaLabel
                        iconKind={entry.iconKind}
                        name={entry.computerName}
                        platform={entry.platform}
                      />
                    )}
                    <span className="ml-1 font-mono text-badge text-faint tabular-nums">
                      {entry.updatedLabel}
                    </span>
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>

            {offlineHits.length > 0 ? (
              <>
                <CommandSeparator />
                <CommandGroup heading="Cached from offline computers">
                  {offlineHits.map(entry => (
                    <CommandItem
                      className={cn("items-center gap-2.5 py-2.5 opacity-70")}
                      key={entry.id}
                      onSelect={() => setSelectedId(entry.id)}
                      value={`${entry.title} ${entry.project} ${entry.computerName} offline`}
                    >
                      <StateGlyph size="sm" state="idle" />
                      <span className="min-w-0 flex-1 truncate text-small-body text-fg-2">
                        {entry.title}
                      </span>
                      <span className="flex shrink-0 items-center gap-1.5 text-micro text-faint">
                        <WorkspaceBadge identity={{ kind: "project", name: entry.project }} />
                        <span aria-hidden="true">·</span>
                        <ComputerMetaLabel
                          iconKind={entry.iconKind}
                          name={entry.computerName}
                          offline
                          platform={entry.platform}
                        />
                        <Pill form="tint" size="sm" tone="neutral">
                          Read-only
                        </Pill>
                      </span>
                    </CommandItem>
                  ))}
                </CommandGroup>
              </>
            ) : null}
          </CommandList>

          <div className="flex items-center justify-between border-t border-line px-3 py-2 text-badge text-subtle">
            <span>Searching This computer, studio, devbox, and 1 offline computer</span>
            <span className="font-mono text-faint">↵ Open · Esc Close</span>
          </div>
        </Command>
      </DialogContent>
    </Dialog>
  );

  const activeSession =
    REMOTE_SESSION_ENTRIES.find(s => s.id === selectedId) ?? REMOTE_SESSION_ENTRIES[2]!;

  return (
    <RemoteSessionsWindow
      activeComputer={COMPUTERS_PRO[2]!}
      activeSession={activeSession}
      isWorking
      overlay={searchDialog}
      sessions={REMOTE_SESSION_ENTRIES}
      transcript={DEVBOX_TRANSCRIPT}
    />
  );
}

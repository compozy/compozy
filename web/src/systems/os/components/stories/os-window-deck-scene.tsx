import { Button, Input, Pill, type TopbarSlotStore } from "@compozy/ui";
import { useState, type ReactNode } from "react";
import { fn } from "storybook/test";

import { OsShellContext } from "../../contexts/os-shell-context";
import type { OsWindowFrameModel } from "../../lib/group-projection";
import type { OsWindow } from "../../lib/os-types";
import { OsWindowChrome, OsWindowSurface } from "../os-window-frame";
import { OsWindowDeck } from "../os-window-deck";
import { buildDeskItems, DesktopShell } from "./_desktop";
import { createLiveStoryShell } from "./_shell-fixture";

export function ListBody() {
  return (
    <ul className="flex flex-col py-1">
      {[
        ["Nightly triage", "Running · 12m"],
        ["Rotate vault keys", "Needs you"],
        ["Sync marketplace index", "2h ago"],
      ].map(([title, detail]) => (
        <li
          key={title}
          className="flex items-center gap-3 border-b border-line-soft px-4 py-2.5 last:border-0"
        >
          <Pill.Dot size="sm" tone={detail === "Needs you" ? "accent" : "success"} />
          <span className="min-w-0 flex-1 truncate text-small-body font-medium text-fg">
            {title}
          </span>
          <span className="shrink-0 font-mono text-micro text-subtle">{detail}</span>
        </li>
      ))}
    </ul>
  );
}

export function RunBody() {
  return (
    <div className="space-y-1.5 p-4 font-mono text-micro leading-relaxed text-muted">
      <p>
        <span className="text-subtle">12:41:22</span> <span className="text-fg">step 4/6</span>
        {" · apply fixes — 3 files changed"}
      </p>
      <p>
        <span className="text-subtle">12:40:57</span> tool <span className="text-fg">Edit</span>
        {" · internal/loop/action.go"}
      </p>
      <p>
        <span className="text-subtle">12:40:31</span> tool <span className="text-fg">Read</span>
        {" · internal/loop/registry.go"}
      </p>
    </div>
  );
}

export function SessionBody({ agentName = "cto-agent" }: { agentName?: string }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="min-h-0 flex-1 space-y-4 p-4 text-small-body text-fg">
        <div className="max-w-[52ch] rounded-md bg-chat-fill-user px-3 py-2">
          Tighten the release summary before the review.
        </div>
        <div className="max-w-[60ch]">
          <p className="mb-1 font-medium text-muted">{agentName}</p>
          The review is open. I am checking the release notes and permission changes.
        </div>
      </div>
      <form className="flex items-center gap-2 border-t border-line px-3 py-2.5">
        <Input aria-label={`Message ${agentName}`} placeholder={`Message ${agentName}`} />
        <Button size="sm" type="button">
          Send
        </Button>
      </form>
    </div>
  );
}

export function WindowDeckScene({
  frame,
  windows,
  activeTitle,
  children,
  single = false,
  slotStores,
}: {
  frame: OsWindowFrameModel;
  windows: Record<string, OsWindow>;
  activeTitle: string;
  children: ReactNode;
  single?: boolean;
  slotStores: ReadonlyMap<string, TopbarSlotStore>;
}) {
  const [shell] = useState(() =>
    createLiveStoryShell({ windows, focusedWindowId: frame.activeWindowId })
  );
  const activeSlot = slotStores.get(frame.activeWindowId);

  return (
    <OsShellContext.Provider value={shell}>
      <DesktopShell dockItems={buildDeskItems({ open: ["tasks", "sessions", "marketplace"] })}>
        <OsWindowChrome
          className="absolute top-[118px] left-1/2 h-[450px] w-[960px] -translate-x-1/2"
          focused
        >
          {!single ? (
            <OsWindowDeck
              dragHandleClassName="deck-drag-handle"
              frame={frame}
              onTrafficLight={fn()}
              slotStores={slotStores}
            />
          ) : null}
          <OsWindowSurface
            controls={single ? "head" : "deck"}
            focused
            onTrafficLight={fn()}
            slotStore={activeSlot}
            title={activeTitle}
          >
            {children}
          </OsWindowSurface>
        </OsWindowChrome>
      </DesktopShell>
    </OsShellContext.Provider>
  );
}

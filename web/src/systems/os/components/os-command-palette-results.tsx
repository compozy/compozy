import { ArrowRightLeft } from "lucide-react";

import { CommandGroup, CommandItem } from "@compozy/ui";

import type { OsCommandPaletteModel } from "../hooks/use-os-command-palette";
import { OS_APP_DESCRIPTORS } from "../lib/app-catalog";

const PALETTE_APPS = (() => {
  const apps = [];
  for (const app of Object.values(OS_APP_DESCRIPTORS)) {
    if (app.id !== "new-tab" && app.id !== "session") apps.push(app);
  }
  return apps;
})();

interface OsCommandPaletteResultsProps {
  destination: boolean;
  model: OsCommandPaletteModel;
}

/** App, session, and cross-desktop tab search results for the command palette. */
export function OsCommandPaletteResults({ destination, model }: OsCommandPaletteResultsProps) {
  return (
    <>
      <CommandGroup heading={destination ? "Open in this tab" : "Apps"}>
        {PALETTE_APPS.map(app => (
          <CommandItem
            key={app.id}
            value={`open ${app.title}`}
            data-testid={`os-palette-app-${app.id}`}
            onSelect={() =>
              destination ? model.pickDestination({ app: app.id }) : model.openApp(app.id)
            }
            disabled={!model.commandsAvailable}
          >
            <app.icon className="size-3.5 text-muted" />
            Open {app.title}
          </CommandItem>
        ))}
      </CommandGroup>
      {model.paletteSessions.length > 0 ? (
        <CommandGroup heading="Sessions">
          {model.paletteSessions.map(session => {
            const agentName = session.agent_name?.trim() ?? "";
            const available = model.commandsAvailable && agentName.length > 0;
            return (
              <CommandItem
                key={session.id}
                value={`session ${session.name ?? session.id} ${agentName}`}
                data-testid={`os-palette-session-${session.id}`}
                onSelect={() => {
                  if (!available) return;
                  if (destination) {
                    model.pickDestination({
                      app: "session",
                      instanceKey: session.id,
                      route: {
                        pathname: `/agents/${encodeURIComponent(agentName)}/sessions/${encodeURIComponent(session.id)}`,
                        search: {},
                      },
                    });
                  } else {
                    model.jumpToSession(session.id, agentName);
                  }
                }}
                disabled={!available}
              >
                <OS_APP_DESCRIPTORS.session.icon className="size-3.5 text-muted" />
                <span className="min-w-0 truncate">{session.name?.trim() || session.id}</span>
                <span className="ml-auto text-micro text-subtle">
                  {agentName || "Agent unavailable"}
                </span>
              </CommandItem>
            );
          })}
        </CommandGroup>
      ) : null}
      {!destination && model.tabResults.length > 0 ? (
        <CommandGroup heading="Go to tab">
          {model.tabResults.map(tab => (
            <CommandItem
              key={tab.windowId}
              value={`go to tab ${tab.label} ${tab.desktopName}`}
              data-testid={`os-palette-tab-${tab.windowId}`}
              onSelect={() => model.goToTab(tab.windowId)}
              disabled={!model.commandsAvailable}
            >
              <ArrowRightLeft className="size-3.5 text-muted" />
              <span className="min-w-0 truncate">{tab.label}</span>
              {tab.needsInput ? (
                <span
                  data-slot="os-palette-tab-attention"
                  className="inline-flex h-3.5 min-w-deck-badge shrink-0 items-center justify-center rounded-full bg-accent px-1 font-mono text-[9px] leading-none font-bold text-accent-ink"
                >
                  1
                </span>
              ) : null}
              <span className="ml-auto text-micro text-subtle">
                {tab.minimized ? "minimized · " : ""}
                {tab.desktopName}
              </span>
            </CommandItem>
          ))}
        </CommandGroup>
      ) : null}
    </>
  );
}

"use client";

/** Interactive Run on showcase for the landing Computers section. */
import { Button, Pill, StatusDot, cn } from "@compozy/ui";
import { Check, ChevronDown, Laptop, Monitor, Scale } from "@compozy/ui/icons";
import { ClaudeLogo } from "@compozy/ui/logos";
import { useState } from "react";

import {
  FLEET_CANDIDATES,
  LANDING_COMPUTERS,
  type LandingComputer,
} from "@/components/landing/computers-data";
import { LoadMeter } from "@/components/landing/computers-shots";
import { FOCUS } from "@/components/site/layout";

type RunOnSelection = "auto" | "local" | LandingComputer["id"];

export function RunOnShot() {
  const [selected, setSelected] = useState<RunOnSelection>("auto");
  const selectedComputer = LANDING_COMPUTERS.find(item => item.id === selected);
  const selectedLabel =
    selected === "auto"
      ? "Auto → studio"
      : selected === "local"
        ? "This computer · MacBook Pro"
        : (selectedComputer?.name ?? "devbox");

  return (
    <div className="flex h-full flex-col bg-[#121212] text-fg">
      <div className="flex h-11 shrink-0 items-center justify-between border-b border-site-light-080 bg-[#0d0d0d] px-4">
        <div className="flex items-center gap-3">
          <div aria-hidden className="flex items-center gap-1.5">
            <span className="size-2.5 rounded-full bg-site-light-150" />
            <span className="size-2.5 rounded-full bg-site-light-150" />
            <span className="size-2.5 rounded-full bg-site-light-150" />
          </div>
          <span className="text-[13px] font-medium text-fg">New session</span>
        </div>
        <span className="font-mono text-[11.5px] text-subtle">4 computers online</span>
      </div>

      <div className="grid flex-1 gap-5 overflow-y-auto p-5 md:grid-cols-[minmax(0,1.15fr)_minmax(0,0.85fr)] md:p-6">
        <div className="flex flex-col gap-3.5 rounded-[14px] border border-site-light-080 bg-[#171717] p-5">
          <div>
            <p className="m-0 mb-1.5 text-[12px] font-medium text-muted-foreground">Run on</p>
            <div className="flex h-9 items-center justify-between rounded-[10px] border border-site-light-120 bg-[#101010] px-3 text-[13.5px] text-fg">
              <span className="inline-flex items-center gap-2 truncate">
                <Scale aria-hidden className="size-3.5 shrink-0 text-brand-strong" />
                <span className="truncate">{selectedLabel}</span>
              </span>
              <ChevronDown aria-hidden className="size-3.5 shrink-0 text-subtle" />
            </div>
          </div>

          <div
            role="listbox"
            aria-label="Run on computer"
            className="divide-y divide-site-light-060 overflow-hidden rounded-[12px] border border-site-light-100 bg-[#101010]"
          >
            <button
              type="button"
              role="option"
              aria-selected={selected === "auto"}
              onClick={() => setSelected("auto")}
              className={cn(
                "flex w-full cursor-pointer items-center justify-between gap-3 px-3.5 py-2.5 text-left transition-colors",
                FOCUS,
                selected === "auto" ? "bg-site-light-080 text-fg" : "hover:bg-site-light-040"
              )}
            >
              <span className="flex min-w-0 items-center gap-2.5">
                <Scale aria-hidden className="size-4 shrink-0 text-brand-strong" />
                <span className="min-w-0">
                  <span className="block text-[13px] font-medium text-fg">
                    Auto <span className="font-normal text-muted-foreground">→ studio</span>
                  </span>
                  <span className="block truncate text-[11.5px] text-subtle">
                    Picks the least busy computer
                  </span>
                </span>
              </span>
              {selected === "auto" ? (
                <Check aria-hidden className="size-3.5 text-brand-strong" />
              ) : null}
            </button>

            <button
              type="button"
              role="option"
              aria-selected={selected === "local"}
              onClick={() => setSelected("local")}
              className={cn(
                "flex w-full cursor-pointer items-center justify-between gap-3 px-3.5 py-2.5 text-left transition-colors",
                FOCUS,
                selected === "local" ? "bg-site-light-080 text-fg" : "hover:bg-site-light-040"
              )}
            >
              <span className="flex min-w-0 items-center gap-2.5">
                <Laptop aria-hidden className="size-4 shrink-0 text-muted-foreground" />
                <span className="min-w-0">
                  <span className="block text-[13px] font-medium text-fg">This computer</span>
                  <span className="flex items-center gap-1.5 text-[11.5px] text-subtle">
                    <StatusDot size="sm" tone="success" />
                    MacBook Pro
                  </span>
                </span>
              </span>
              <LoadMeter cpu={32} mem={58} />
            </button>

            {LANDING_COMPUTERS.slice(0, 3).map(computer => (
              <button
                key={computer.id}
                type="button"
                role="option"
                aria-selected={selected === computer.id}
                onClick={() => setSelected(computer.id)}
                className={cn(
                  "flex w-full cursor-pointer items-center justify-between gap-3 px-3.5 py-2.5 text-left transition-colors",
                  FOCUS,
                  selected === computer.id ? "bg-site-light-080 text-fg" : "hover:bg-site-light-040"
                )}
              >
                <span className="flex min-w-0 items-center gap-2.5">
                  <Monitor aria-hidden className="size-4 shrink-0 text-muted-foreground" />
                  <span className="min-w-0">
                    <span className="block text-[13px] font-medium text-fg">{computer.name}</span>
                    <span className="block truncate font-mono text-[11.5px] text-subtle">
                      {computer.routeLabel}
                    </span>
                  </span>
                </span>
                <LoadMeter cpu={computer.cpuPercent} mem={computer.memPercent} />
              </button>
            ))}
          </div>

          <div className="mt-auto flex items-center justify-between border-t border-site-light-080 pt-3">
            <span className="text-[12px] text-muted-foreground">
              {selected === "auto"
                ? "Auto resolves to studio"
                : selected === "local"
                  ? "Runs on MacBook Pro"
                  : `Runs on ${selectedComputer?.name ?? "devbox"}`}
            </span>
            <Button size="sm" type="button">
              Start session
            </Button>
          </div>
        </div>

        <div className="flex flex-col justify-between gap-4 rounded-[14px] border border-site-light-080 bg-[#0d0d0d] p-5">
          <div>
            <p className="m-0 text-[12.5px] font-medium text-fg">Live fleet capacity</p>
            <p className="m-0 mt-1 text-[13px]/[1.5] text-muted-foreground">
              Auto scores every connected computer that holds the project and picks the one with the
              most free CPU and memory.
            </p>

            <div className="mt-4 grid grid-cols-2 gap-3 border-y border-site-light-080 py-3">
              <div>
                <p className="m-0 text-[11px] text-subtle">Project</p>
                <p className="m-0 mt-0.5 text-[13.5px] font-medium text-fg">api</p>
              </div>
              <div>
                <p className="m-0 text-[11px] text-subtle">Agent</p>
                <p className="m-0 mt-0.5 inline-flex items-center gap-1.5 text-[13.5px] font-medium text-fg">
                  <ClaudeLogo className="size-3.5" aria-hidden />
                  Claude Code
                </p>
              </div>
            </div>

            <ul className="m-0 mt-4 list-none divide-y divide-site-light-060 p-0">
              {FLEET_CANDIDATES.map(candidate => (
                <li
                  key={candidate.id}
                  className="flex items-center justify-between gap-3 py-2.5 first:pt-0 last:pb-0"
                >
                  <div className="flex min-w-0 items-center gap-2">
                    <Laptop aria-hidden className="size-3.5 shrink-0 text-subtle" />
                    <span
                      className={cn(
                        "truncate text-[13px]",
                        candidate.winner ? "font-medium text-fg" : "text-muted-foreground"
                      )}
                    >
                      {candidate.name}
                    </span>
                    {candidate.winner ? (
                      <Pill form="tint" size="xs" tone="success">
                        Chosen
                      </Pill>
                    ) : null}
                  </div>
                  <LoadMeter cpu={candidate.cpuPercent} mem={candidate.memPercent} />
                </li>
              ))}
            </ul>
          </div>
          <p className="m-0 border-t border-site-light-080 pt-3 text-[12px] text-subtle">
            Works in Session create, Kanban New session, and scheduled automations.
          </p>
        </div>
      </div>
    </div>
  );
}

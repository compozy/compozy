"use client";

/** Interactive Computers list and Phone access showcases for the landing Computers section. */
import { Button, Pill, Spinner, StatusDot, Switch, cn } from "@compozy/ui";
import { Check, ChevronDown, Laptop, Lock, Monitor, Plus } from "@compozy/ui/icons";
import { useState } from "react";

import {
  LANDING_COMPUTERS,
  PHONE_HOSTED_URL,
  PHONE_POINTS,
  PHONE_SESSIONS,
  type ComputerStatus,
  type LandingComputer,
} from "@/components/landing/computers-data";
import { FOCUS } from "@/components/site/layout";

function StatusBadge({ status }: { status: ComputerStatus }) {
  if (status === "connected") {
    return (
      <Pill form="tint" size="xs" tone="success">
        <StatusDot size="sm" tone="success" />
        Connected
      </Pill>
    );
  }
  if (status === "needs_attention") {
    return (
      <Pill form="tint" size="xs" tone="warning">
        <StatusDot size="sm" tone="warning" />
        Needs attention
      </Pill>
    );
  }
  if (status === "connecting") {
    return (
      <Pill form="tint" size="xs" tone="neutral">
        <Spinner className="size-3" />
        Connecting
      </Pill>
    );
  }
  return (
    <Pill form="tint" size="xs" tone="neutral">
      Disabled
    </Pill>
  );
}

export function LoadMeter({ cpu, mem }: { cpu: number; mem: number }) {
  return (
    <span className="inline-flex items-center gap-2 font-mono text-[11.5px] text-subtle tabular-nums">
      <span>CPU {cpu}%</span>
      <span aria-hidden className="text-site-light-150">
        ·
      </span>
      <span>RAM {mem}%</span>
    </span>
  );
}

export function ComputersListShot() {
  const [computers, setComputers] = useState<LandingComputer[]>([...LANDING_COMPUTERS]);
  const connectedCount = computers.filter(
    item => item.status === "connected" || item.status === "needs_attention"
  ).length;
  const connectingCount = computers.filter(item => item.status === "connecting").length;

  const handleToggle = (id: string, nextEnabled: boolean) => {
    setComputers(current =>
      current.map(item =>
        item.id === id
          ? { ...item, enabled: nextEnabled, status: nextEnabled ? "connected" : "disabled" }
          : item
      )
    );
  };

  return (
    <div className="flex h-full flex-col bg-[#121212] text-fg">
      <div className="flex h-11 shrink-0 items-center justify-between border-b border-site-light-080 bg-[#0d0d0d] px-4">
        <div className="flex items-center gap-3">
          <div aria-hidden className="flex items-center gap-1.5">
            <span className="size-2.5 rounded-full bg-site-light-150" />
            <span className="size-2.5 rounded-full bg-site-light-150" />
            <span className="size-2.5 rounded-full bg-site-light-150" />
          </div>
          <span className="text-[13px] font-medium text-fg">Settings</span>
          <span className="text-subtle">/</span>
          <span className="text-[13px] text-muted-foreground">Computers</span>
        </div>
        <span className="inline-flex h-7 items-center gap-1.5 rounded-[8px] border border-site-light-120 bg-site-light-050 px-2.5 text-[12px] text-fg">
          <Plus aria-hidden className="size-3.5" />
          Add computer
        </span>
      </div>

      <div className="flex-1 overflow-y-auto p-5 md:p-6">
        <div className="mx-auto max-w-[900px] space-y-4">
          <div className="flex flex-wrap items-baseline justify-between gap-3 border-b border-site-light-080 pb-3">
            <div>
              <p className="m-0 text-[17px] font-medium text-fg">Computers</p>
              <p className="m-0 mt-0.5 text-[13px] text-muted-foreground">
                {connectedCount} connected
                {connectingCount > 0 ? ` · ${connectingCount} connecting` : ""} · Pro plan
              </p>
            </div>
            <div className="flex items-center gap-2.5 font-mono text-[12px] text-muted-foreground tabular-nums">
              <span>Hosted routes</span>
              <span className="h-1.5 w-20 overflow-hidden rounded-full bg-site-light-100">
                <span className="block h-full w-[30%] rounded-full bg-brand" />
              </span>
              <span className="text-fg">3 / 10</span>
            </div>
          </div>

          <div className="overflow-hidden rounded-[14px] border border-site-light-080 bg-[#171717]">
            <div className="border-b border-site-light-080 px-4 py-2.5 font-mono text-[11.5px] text-subtle">
              Computers ({computers.length})
            </div>
            <ul className="m-0 list-none divide-y divide-site-light-060 p-0">
              {computers.map(computer => (
                <li
                  key={computer.id}
                  className="flex flex-wrap items-center justify-between gap-3 px-4 py-3.5"
                >
                  <div className="flex min-w-0 items-center gap-3">
                    <span className="grid size-8 shrink-0 place-items-center rounded-[9px] border border-site-light-080 bg-[#101010] text-muted-foreground">
                      {computer.platform === "macos" ? (
                        <Laptop aria-hidden className="size-4" />
                      ) : (
                        <Monitor aria-hidden className="size-4" />
                      )}
                    </span>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-[14px] font-medium text-fg">{computer.name}</span>
                        <StatusBadge status={computer.status} />
                      </div>
                      <p className="m-0 mt-0.5 truncate font-mono text-[12px] text-subtle">
                        {computer.attentionHint ?? computer.routeLabel}
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-4">
                    <span className="hidden font-mono text-[12px] text-subtle sm:inline">
                      {computer.routesCount} {computer.routesCount === 1 ? "route" : "routes"}
                    </span>
                    <Switch
                      aria-label={`Enable ${computer.name}`}
                      checked={computer.enabled}
                      onCheckedChange={checked => handleToggle(computer.id, checked)}
                    />
                  </div>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}

export function PhoneShot() {
  const [phoneTab, setPhoneTab] = useState<"sessions" | "approvals" | "terminal">("sessions");

  return (
    <div className="grid h-full items-center gap-6 bg-[#121212] p-5 md:p-7 lg:grid-cols-[minmax(0,1fr)_390px]">
      <div className="flex flex-col justify-center gap-4 px-2 lg:px-5">
        <div className="inline-flex w-fit items-center gap-2 rounded-pill border border-site-light-100 bg-[#0d0d0d] px-3 py-1 font-mono text-[12px] text-muted-foreground">
          <Lock aria-hidden className="size-3 text-success" />
          <span>{PHONE_HOSTED_URL}</span>
        </div>
        <h3 className="m-0 font-display text-[24px]/[1.15] font-normal tracking-[-0.02em] text-fg md:text-[30px]">
          Every session on your computer, in your pocket.
        </h3>
        <p className="m-0 text-[15px]/[1.55] text-muted-foreground">
          Turn on Hosted route for any machine on Pro. Sign in from your phone browser with Google
          or GitHub to read live transcripts, approve waiting commands, or open a remote terminal.
        </p>
        <ul className="m-0 list-none space-y-2.5 p-0 pt-1">
          {PHONE_POINTS.map(point => (
            <li key={point} className="flex items-start gap-2.5 text-[14.5px] text-fg/85">
              <Check aria-hidden className="mt-0.5 size-4 shrink-0 text-brand-strong" />
              <span>{point}</span>
            </li>
          ))}
        </ul>
      </div>

      <div className="mx-auto flex h-[470px] w-full max-w-[390px] flex-col overflow-hidden rounded-[24px] border border-site-light-150 bg-[#121212] shadow-[0_24px_70px_-20px_rgba(0,0,0,0.9)]">
        <div className="flex h-9 shrink-0 items-center justify-center gap-1.5 border-b border-site-light-080 bg-[#0d0d0d] px-3">
          <Lock aria-hidden className="size-3 text-subtle" />
          <span className="truncate font-mono text-[11.5px] text-muted-foreground">
            {PHONE_HOSTED_URL}
          </span>
        </div>

        <div className="flex items-center justify-between border-b border-site-light-080 bg-[#151515] px-4 py-2.5">
          <span className="inline-flex items-center gap-2 text-[13px] font-medium text-fg">
            <StatusDot size="sm" tone="success" />
            studio
            <ChevronDown aria-hidden className="size-3.5 text-subtle" />
          </span>
          <span className="font-mono text-[11px] text-subtle">2 peers</span>
        </div>

        <div className="flex-1 overflow-y-auto p-3.5">
          {phoneTab === "sessions" ? (
            <ul className="m-0 list-none space-y-2.5 p-0">
              {PHONE_SESSIONS.map(session => (
                <li
                  key={session.id}
                  className="rounded-[12px] border border-site-light-080 bg-[#171717] p-3"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-[11px] text-subtle">{session.project}</span>
                    <Pill
                      form="tint"
                      size="xs"
                      tone={
                        session.status === "Needs you"
                          ? "warning"
                          : session.status === "Working"
                            ? "brand"
                            : "success"
                      }
                    >
                      {session.status}
                    </Pill>
                  </div>
                  <p className="m-0 mt-1 text-[13.5px] font-medium text-fg">{session.title}</p>
                  <p className="m-0 mt-1 text-[12px] text-muted-foreground">{session.detail}</p>
                </li>
              ))}
            </ul>
          ) : null}

          {phoneTab === "approvals" ? (
            <div className="rounded-[12px] border border-warning/30 bg-[#171717] p-3.5">
              <p className="m-0 text-[12px] font-medium text-warning">Waiting for approval</p>
              <p className="m-0 mt-1.5 text-[13.5px] font-medium text-fg">
                Refactor checkout webhook retries
              </p>
              <code className="mt-2 block rounded-[8px] bg-[#0d0d0d] px-2.5 py-2 font-mono text-[12px] text-brand-strong">
                git push origin fix/retries
              </code>
              <div className="mt-3 flex gap-2">
                <Button size="xs" type="button">
                  Approve
                </Button>
                <Button size="xs" variant="outline" type="button">
                  Deny
                </Button>
              </div>
            </div>
          ) : null}

          {phoneTab === "terminal" ? (
            <div className="rounded-[12px] border border-site-light-080 bg-[#0a0a0a] p-3.5 font-mono text-[12px]/[1.6] text-fg/90">
              <p className="m-0 text-subtle">pedro@studio ~/code/api</p>
              <p className="m-0 mt-1 text-brand-strong">$ go test ./internal/billing/...</p>
              <p className="m-0 mt-1 text-success">ok github.com/compozy/api/internal/billing</p>
            </div>
          ) : null}
        </div>

        <div className="grid shrink-0 grid-cols-3 border-t border-site-light-080 bg-[#0d0d0d] p-1.5">
          {(
            [
              ["sessions", "Sessions"],
              ["approvals", "Approvals"],
              ["terminal", "Terminal"],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              aria-pressed={phoneTab === id}
              onClick={() => setPhoneTab(id)}
              className={cn(
                "cursor-pointer rounded-[8px] py-1.5 text-[12px] transition-colors",
                FOCUS,
                phoneTab === id
                  ? "bg-site-light-100 font-medium text-fg"
                  : "text-muted-foreground hover:text-fg"
              )}
            >
              {label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

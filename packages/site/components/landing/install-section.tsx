"use client";

import { useState, type KeyboardEvent } from "react";
import { buttonVariants } from "@agh/ui";
import { cn } from "@agh/ui/lib/utils";
import { LandingCodeBlock } from "./primitives/code-block";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";

type TabId = "homebrew" | "npm" | "go";

const INSTALL_TABS: { id: TabId; label: string; command: string; note: string }[] = [
  {
    id: "homebrew",
    label: "Homebrew",
    command: "brew install compozy/compozy/agh",
    note: "Managed updates · macOS + Linux · Compozy tap",
  },
  {
    id: "npm",
    label: "npm",
    command: "npm install -g @compozy/agh",
    note: "Managed updates · Node package · downloads the AGH release archive",
  },
  {
    id: "go",
    label: "Go",
    command: "go install github.com/compozy/agh@latest",
    note: "Requires Go · builds the current release from the public module",
  },
];

const STEPS = [
  {
    step: "01",
    title: "Bootstrap your AGH home",
    description:
      "Create ~/.agh/config.toml and the default general agent before you start the daemon.",
    code: "agh install",
  },
  {
    step: "02",
    title: "Start the daemon",
    description: "One local process, detached by default, exposing CLI, HTTP/SSE, and the web UI.",
    code: "agh daemon start",
  },
  {
    step: "03",
    title: "Launch a real session",
    description:
      "Create the session from the repository you want AGH to manage so workspace resolution is explicit.",
    code: 'agh workspace add "$PWD" --name current\nagh session new --workspace current --agent general',
  },
];

function getTabId(id: TabId) {
  return `install-tab-${id}`;
}

function getPanelId(id: TabId) {
  return `install-panel-${id}`;
}

export function InstallSection() {
  const [tab, setTab] = useState<TabId>("homebrew");

  function selectTab(next: TabId) {
    setTab(next);
    document.getElementById(getTabId(next))?.focus();
  }

  function handleTabKeyDown(event: KeyboardEvent<HTMLButtonElement>, current: TabId) {
    const index = INSTALL_TABS.findIndex(item => item.id === current);
    if (index === -1) {
      return;
    }

    switch (event.key) {
      case "ArrowRight": {
        event.preventDefault();
        const next = INSTALL_TABS[(index + 1) % INSTALL_TABS.length];
        selectTab(next.id);
        return;
      }
      case "ArrowLeft": {
        event.preventDefault();
        const next = INSTALL_TABS[(index - 1 + INSTALL_TABS.length) % INSTALL_TABS.length];
        selectTab(next.id);
        return;
      }
      case "Home":
        event.preventDefault();
        selectTab(INSTALL_TABS[0].id);
        return;
      case "End":
        event.preventDefault();
        selectTab(INSTALL_TABS[INSTALL_TABS.length - 1].id);
        return;
      default:
        return;
    }
  }

  return (
    <SectionFrame background="surface" padY="lg" className="border-b border-line">
      <SectionHeader
        align="center"
        eyebrow="Getting started"
        title="Three commands. First session in under a minute."
        description="macOS and Linux. Install with Homebrew, npm, or Go. The full installation guide also covers the verified binary installer, Linux packages, and source builds."
      />

      <div className="mx-auto mt-10 w-full max-w-190">
        <div
          role="tablist"
          aria-label="Install methods"
          className="flex flex-wrap gap-1 rounded-md border border-line bg-canvas p-1"
        >
          {INSTALL_TABS.map(t => (
            <button
              key={t.id}
              type="button"
              role="tab"
              id={getTabId(t.id)}
              aria-controls={getPanelId(t.id)}
              aria-selected={t.id === tab}
              tabIndex={t.id === tab ? 0 : -1}
              onClick={() => setTab(t.id)}
              onKeyDown={event => handleTabKeyDown(event, t.id)}
              className={cn(
                buttonVariants({
                  variant: t.id === tab ? "secondary" : "ghost",
                  size: "sm",
                }),
                "flex-1 font-mono text-xs tracking-mono",
                t.id === tab && "bg-accent-tint text-accent hover:bg-accent-tint"
              )}
            >
              {t.label}
            </button>
          ))}
        </div>

        {INSTALL_TABS.map(t => (
          <div
            key={t.id}
            id={getPanelId(t.id)}
            role="tabpanel"
            aria-labelledby={getTabId(t.id)}
            hidden={t.id !== tab}
            className="mt-4"
          >
            <LandingCodeBlock code={t.command} caption={t.note} shell />
          </div>
        ))}
      </div>

      <div className="mx-auto mt-14 max-w-190">
        <div className="flex flex-col gap-5">
          {STEPS.map(item => (
            <div
              key={item.step}
              className="flex flex-col gap-4 rounded-diagram border border-line bg-canvas p-6"
            >
              <div className="flex items-start gap-4">
                <span className="mt-0.5 font-mono text-lg font-medium text-accent">
                  {item.step}
                </span>
                <div className="flex-1">
                  <h3 className="text-lg font-medium text-fg">{item.title}</h3>
                  <p className="mt-2 max-w-[52ch] text-sm leading-relaxed text-muted">
                    {item.description}
                  </p>
                </div>
              </div>
              <div className="ml-11">
                <LandingCodeBlock code={item.code} copyable caption="shell" shell />
              </div>
            </div>
          ))}
        </div>
      </div>
    </SectionFrame>
  );
}

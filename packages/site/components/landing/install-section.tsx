"use client";

import { useState, type KeyboardEvent } from "react";
import { buttonVariants } from "@compozy/ui";
import { cn } from "@compozy/ui/lib/utils";
import { LandingCodeBlock } from "./primitives/code-block";
import { SectionFrame } from "./primitives/section-frame";
import { SectionHeader } from "./primitives/section-header";

type TabId = "installer" | "npm" | "go";

const INSTALL_TABS: { id: TabId; label: string; command: string; note: string }[] = [
  {
    id: "installer",
    label: "Installer",
    command: "curl -fsSL https://compozy.com/install.sh | sh",
    note: "Verified beta · Sigstore provenance · opens bootstrap on an interactive terminal",
  },
  {
    id: "npm",
    label: "npm",
    command: "npm install -g @compozy/cli@beta",
    note: "Beta channel · Node package · managed updates",
  },
  {
    id: "go",
    label: "Go",
    command: "go install github.com/compozy/compozy@v0.3.0-beta.2",
    note: "Pinned beta · requires Go · builds from the public module",
  },
];

type InstallStep = {
  title: string;
  description: string;
  code: string;
};

const BOOTSTRAP_STEP: InstallStep = {
  title: "Bootstrap your CompozyOS home",
  description:
    "npm and Go install the binary only. Create ~/.compozy/config.toml and the default general agent once.",
  code: "compozy install",
};

const COMMON_STEPS: InstallStep[] = [
  {
    title: "Start the daemon",
    description: "One local process, detached by default, exposing CLI, HTTP/SSE, and the web UI.",
    code: "compozy daemon start",
  },
  {
    title: "Launch a real session",
    description:
      "Run from the repository you want to manage. CompozyOS infers and registers the workspace from your current directory.",
    code: "compozy session new --agent general --name first-run",
  },
];

function nextSteps(tab: TabId): InstallStep[] {
  return tab === "installer" ? COMMON_STEPS : [BOOTSTRAP_STEP, ...COMMON_STEPS];
}

function getTabId(id: TabId) {
  return `install-tab-${id}`;
}

function getPanelId(id: TabId) {
  return `install-panel-${id}`;
}

export function InstallSection() {
  const [tab, setTab] = useState<TabId>("installer");

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
    <SectionFrame id="install" background="surface" padY="lg" className="border-b border-line">
      <SectionHeader
        align="center"
        eyebrow="Getting started"
        title="Install the beta. Start one durable session."
        description="Use the verified installer on macOS or Linux, the npm beta channel, or the pinned Go release. Homebrew returns with the stable release."
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
          {nextSteps(tab).map((item, index) => (
            <div
              key={item.title}
              className="flex flex-col gap-4 rounded-diagram border border-line bg-canvas p-6"
            >
              <div className="flex items-start gap-4">
                <span className="mt-0.5 font-mono text-lg font-medium text-accent">
                  {String(index + 1).padStart(2, "0")}
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

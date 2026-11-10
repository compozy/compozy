import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DownloadHero } from "@/components/download/download-hero";
import { AllDownloads } from "@/components/download/sections";
import {
  COMPARE_CHECKED_ON,
  COMPARE_COLUMNS,
  COMPARE_ROWS,
  COMPARE_SOURCES,
  COMPARE_VERDICTS,
  INSTALL_STEPS,
} from "@/components/landing/content";
import { SiteMotionProvider } from "@/components/site/motion-provider";
import { InstallCommand } from "@/components/site/primitives";
import { Automations, Community, Comparison, FinalCta, Hero, Install } from "@/components/landing";
import { DESKTOP_TARGETS, type DesktopRelease } from "@/lib/release/desktop-downloads";
import { siteConfig } from "@/lib/site-config";
import {
  NPM_INSTALL_COMMAND,
  VERIFIED_INSTALLER_COMMAND,
  goInstallCommand,
} from "@/lib/release/install-commands";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: { href: string; children: ReactNode } & Omit<
    AnchorHTMLAttributes<HTMLAnchorElement>,
    "href"
  >) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("@/components/site/github-stars-api", () => ({
  loadGithubStars: async () => null,
}));

const copyPath = resolve(process.env.COMPOZY_SITE_ROOT ?? process.cwd(), "../../COPY.md");

/** The Hero Lock block of COPY.md §2: headline, subhead and category label. */
function heroLock() {
  const copy = readFileSync(copyPath, "utf8");
  const section = copy.slice(copy.indexOf("### Hero Lock"), copy.indexOf("### Primary Promise"));
  const quoted = section
    .split("\n")
    .filter(line => line.startsWith("> ") && line.length > 2)
    .map(line => line.slice(2).trim());
  const category = section.match(/with `([^`]+)` as a small category label/)?.[1];
  if (quoted.length !== 2 || !category) throw new Error("COPY.md §2 Hero Lock is not parseable");
  return { headline: quoted[0]!.replace(/^\*\*|\*\*$/g, ""), subhead: quoted[1]!, category };
}

afterEach(() => cleanup());

/** The href of a call to action, found as a link by its accessible name. */
function ctaHref(name: string) {
  return screen.getByRole("link", { name }).getAttribute("href");
}

describe("Hero", () => {
  it("ships the COPY.md Hero Lock verbatim with the download and docs actions", () => {
    const lock = heroLock();
    render(<Hero />);

    expect(screen.getByRole("heading", { level: 1, name: lock.headline })).toBeDefined();
    expect(screen.getByText(lock.subhead)).toBeDefined();
    expect(screen.getByText(lock.category)).toBeDefined();
    expect(ctaHref("Download for macOS")).toBe("/download");
    expect(screen.getByRole("link", { name: /Open the runtime docs/ }).getAttribute("href")).toBe(
      "/docs"
    );
    expect(screen.getByText(VERIFIED_INSTALLER_COMMAND)).toBeDefined();
  });
});

/** An inline style that hides an element: what the server serves before motion takes over. */
const HIDDEN_STYLE = /opacity:\s*0(?![.\d])|scale\(0\)/;

/** The server HTML of `node`, parsed into a detached container. */
function served(node: ReactNode) {
  const container = document.createElement("div");
  container.innerHTML = renderToString(node);
  return container;
}

/** The nearest element, from `node` up, whose inline style hides it. */
function hiddenAncestor(node: Element) {
  for (let el: Element | null = node; el; el = el.parentElement) {
    if (HIDDEN_STYLE.test(el.getAttribute("style") ?? "")) return el;
  }
  return null;
}

/** The visible (aria-hidden) copy a `Typed` line draws next to its screen-reader text. */
function typedText(container: Element, text: string) {
  const reader = within(container as HTMLElement).getByText(text, { selector: ".sr-only" });
  return reader.nextElementSibling?.textContent;
}

describe("Served motion", () => {
  it("serves the hero visible: only decorative, textless frames start hidden", () => {
    const hero = served(<Hero />);
    const hidden = Array.from(hero.querySelectorAll("[style]")).filter(el =>
      HIDDEN_STYLE.test(el.getAttribute("style") ?? "")
    );
    for (const el of hidden) {
      expect(el.getAttribute("aria-hidden")).toBe("true");
      expect(el.textContent).toBe("");
    }
  });

  it("serves every comparison check and typed line in its final state", () => {
    const checks = within(served(<Comparison />)).getAllByLabelText("Yes");
    expect(checks.length).toBeGreaterThan(0);
    expect(checks.map(hiddenAncestor)).toEqual(checks.map(() => null));

    const command = INSTALL_STEPS[0]!.command;
    expect(typedText(served(<Install releaseTag="v9.9.9" />), command)).toBe(command);
    const briefing = "A briefing waiting for you every weekday at 08:30.";
    expect(typedText(served(<Automations />), briefing)).toBe(briefing);
  });

  it("hydrates without a mismatch and replays only what starts out of view", async () => {
    const command = INSTALL_STEPS[0]!.command;
    const errors = vi.spyOn(console, "error").mockImplementation(() => undefined);
    const hydrate = async (onScreen: boolean) => {
      const page = (
        <SiteMotionProvider>
          <Install releaseTag="v9.9.9" />
        </SiteMotionProvider>
      );
      const container = served(page);
      document.body.append(container);
      const rect = vi
        .spyOn(Element.prototype, "getBoundingClientRect")
        .mockReturnValue(DOMRect.fromRect({ y: onScreen ? 100 : 5000, height: 20 }));
      const root = await act(async () => hydrateRoot(container, page));
      // Motion values reach the DOM on the next animation frame.
      await act(() => new Promise(resolve => requestAnimationFrame(() => resolve(undefined))));
      const shown = typedText(container, command);
      act(() => root.unmount());
      rect.mockRestore();
      container.remove();
      return shown;
    };

    // On screen at hydration: stays as served. Below the fold: clears out of sight to type later.
    expect(await hydrate(true)).toBe(command);
    expect(await hydrate(false)).toBe("");
    expect(errors).not.toHaveBeenCalled();
    errors.mockRestore();
  });
});

describe("Comparison", () => {
  it("names the comparison set as compression, never as a ranking (COPY.md §10)", () => {
    render(<Comparison />);
    const table = screen.getByRole("table");
    const headers = within(table)
      .getAllByRole("columnheader")
      .map(header => header.textContent?.trim())
      .filter(Boolean);

    expect(headers).toEqual(["CompozyOS", ...COMPARE_COLUMNS]);
    expect(headers).toContain("DIY stack");
    expect(screen.getByText(/each prove demand for part of the system/)).toBeDefined();

    const text = document.body.textContent ?? "";
    expect(text).not.toMatch(/\b(best|better than|beats|the only|#1|number one)\b/i);
    expect(text).not.toMatch(/Source:|\.resources\//);
  });

  it("states each documented verdict, with the date and the docs it was checked against", () => {
    render(<Comparison />);
    const table = screen.getByRole("table");
    const rows = within(table).getAllByRole("row").slice(1);
    expect(rows).toHaveLength(COMPARE_ROWS.length);
    rows.forEach((row, index) => {
      const cells = within(row).getAllByRole("cell").slice(2);
      const labels = cells.map(cell =>
        cell.querySelector("[aria-label]")?.getAttribute("aria-label")
      );
      expect(labels).toEqual(
        COMPARE_COLUMNS.map(
          column =>
            ({ yes: "Yes", partial: "Partial", no: "No" })[COMPARE_VERDICTS[column][index] ?? "no"]
        )
      );
    });

    const text = document.body.textContent ?? "";
    expect(text).not.toMatch(/to verify|<date>/);
    expect(text).toContain(COMPARE_CHECKED_ON);
    for (const [name, href] of Object.entries(COMPARE_SOURCES)) {
      expect(screen.getByRole("link", { name }).getAttribute("href")).toBe(href);
    }
  });
});

describe("Community", () => {
  it("serves the release count in the HTML, so it reads right before the count-up runs", () => {
    const container = document.createElement("div");
    container.innerHTML = renderToString(<Community releases={1234} />);
    const releases = within(container).getByText("releases").closest("p");
    expect(releases?.textContent).toBe("1,234releases");
  });

  it("shows the live release count's absence instead of inventing a number", () => {
    render(<Community releases={null} />);
    const releases = screen.getByText("releases").closest("p");
    expect(releases?.textContent).toContain("—");
    expect(within(releases as HTMLElement).getByLabelText("unavailable")).toBeDefined();
  });
});

describe("Install", () => {
  it("offers the desktop download and every terminal method, with Go pinned to the release", () => {
    const tag = "v9.9.9-beta.9";
    render(<Install releaseTag={tag} />);

    expect(ctaHref("Download for macOS")).toBe("/download");
    expect(screen.getByText("Start the daemon")).toBeDefined();
    // Homebrew, npm and Go install only the binary: provider setup stays ahead of the daemon.
    const steps = screen.getAllByRole("listitem").map(item => item.textContent ?? "");
    expect(steps.findIndex(text => text.includes("compozy install"))).toBe(0);
    expect(steps.findIndex(text => text.includes("compozy daemon start"))).toBe(1);

    fireEvent.click(screen.getByRole("tab", { name: "Installer" }));
    expect(screen.getByText(VERIFIED_INSTALLER_COMMAND)).toBeDefined();
    fireEvent.click(screen.getByRole("tab", { name: "npm" }));
    expect(screen.getByText(NPM_INSTALL_COMMAND)).toBeDefined();
    fireEvent.click(screen.getByRole("tab", { name: "Go" }));
    expect(screen.getByText(goInstallCommand(tag))).toBeDefined();
  });
});

describe("FinalCta", () => {
  it("links the final actions to the download page and the repository", () => {
    render(<FinalCta />);
    expect(ctaHref("Download CompozyOS")).toBe("/download");
    expect(ctaHref("Star on GitHub")).toBe(siteConfig.repoUrl);
  });
});

/** The landing's install command and the download page's labelled copy control, by their idle names. */
const COPY_CONTROLS = [
  {
    name: "install command",
    render: () => render(<InstallCommand command="compozy install" />),
    idle: "Copy command",
    copied: "Copied",
    failed: "Copy failed",
  },
  {
    name: "download link",
    render: () =>
      render(
        <DownloadHero
          mode="mobile"
          release={{ status: "fallback", releaseUrl: siteConfig.repoUrl }}
          choice={{ platform: "macos", arch: "arm64" }}
          onPlatform={() => undefined}
          onArch={() => undefined}
        />
      ),
    idle: "Copy the download link",
    copied: "Link copied",
    failed: "Couldn't copy the link",
  },
];

function stubClipboard(writeText: (value: string) => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
}

async function clickCopy(name: string) {
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name }));
    await Promise.resolve();
    await Promise.resolve();
  });
}

describe("Copy controls", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it.each(COPY_CONTROLS)("$name shows copy success and then returns to idle", async control => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    stubClipboard(writeText);
    control.render();

    await clickCopy(control.idle);
    expect(writeText).toHaveBeenCalledOnce();
    expect(screen.getByRole("button", { name: control.copied })).toBeDefined();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });
    expect(screen.getByRole("button", { name: control.idle })).toBeDefined();
  });

  it.each(COPY_CONTROLS)(
    "$name shows copy failure instead of swallowing clipboard errors",
    async control => {
      stubClipboard(vi.fn().mockRejectedValue(new Error("blocked")));
      control.render();

      await clickCopy(control.idle);
      expect(screen.getByRole("button", { name: control.failed })).toBeDefined();
    }
  );
});

describe("DownloadHero", () => {
  const releaseUrl = "https://github.com/compozy/compozy/releases/tag/v9.9.9";
  const dmg =
    "https://github.com/compozy/compozy/releases/download/v9.9.9/CompozyOS-9.9.9-mac-arm64.dmg";
  const ready: DesktopRelease = {
    status: "ready",
    tag: "v9.9.9",
    releaseUrl,
    publishedAt: "2026-11-06T02:25:39Z",
    downloads: [
      { target: DESKTOP_TARGETS[0]!, fileName: "CompozyOS-9.9.9-mac-arm64.dmg", url: dmg, size: 1 },
    ],
    checksums: null,
    signature: null,
    windowsCommand: null,
    commandPackages: [],
  };
  const hero = (release: DesktopRelease, platform: "macos" | "windows") =>
    render(
      <DownloadHero
        mode="ready"
        release={release}
        choice={{ platform, arch: "arm64" }}
        onPlatform={() => undefined}
        onArch={() => undefined}
      />
    );

  it("downloads a listed file in this tab", () => {
    hero(ready, "macos");
    const link = screen.getByRole("link", { name: "Download for macOS" });
    expect(link.getAttribute("href")).toBe(dmg);
    expect(link.getAttribute("target")).toBeNull();
  });

  it("serves the All downloads header visible, since it can open in the first viewport", () => {
    const container = served(<AllDownloads release={ready} releaseDate="Nov 6, 2026" />);
    const heading = within(container).getByRole("heading", { name: "All downloads" });
    expect(hiddenAncestor(heading)).toBeNull();
  });

  it("opens the release on GitHub in a new tab when the file is not listed", () => {
    hero(ready, "windows");
    const link = screen.getByRole("link", { name: "Download the command" });
    expect(link.getAttribute("href")).toBe(releaseUrl);
    expect(link.getAttribute("target")).toBe("_blank");
  });
});

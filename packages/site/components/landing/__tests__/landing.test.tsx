import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { act, cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DownloadHero } from "@/components/download/download-hero";
import { COMPARE_COLUMNS } from "@/components/landing/content";
import { InstallCommand } from "@/components/site/primitives";
import { Community, Comparison, FinalCta, Hero, Install } from "@/components/landing";
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
});

describe("Community", () => {
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

  it("opens the release on GitHub in a new tab when the file is not listed", () => {
    hero(ready, "windows");
    const link = screen.getByRole("link", { name: "Download the command" });
    expect(link.getAttribute("href")).toBe(releaseUrl);
    expect(link.getAttribute("target")).toBe("_blank");
  });
});

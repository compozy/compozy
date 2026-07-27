import { render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { baseOptions } from "@/lib/layout.shared";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    className,
  }: {
    href: string;
    children: ReactNode;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}));

import { Comparison } from "../comparison";
import { ExtensibilitySection } from "../extensibility-section";
import { FeatureWall } from "../feature-wall";
import { FinalCta } from "../final-cta";
import { Hero } from "../hero";
import { ProofSection } from "../proof-section";

function resolveImageAsset(src: string | null): string | null {
  if (!src?.startsWith("/_next/image")) return src;
  return new URL(src, "http://localhost").searchParams.get("url") ?? src;
}

describe("Hero", () => {
  it("renders the locked OS definition next to the static shipped-shell proof", () => {
    render(<Hero />);

    expect(
      screen.getByRole("heading", { level: 1, name: "The only true OS for AI agents." })
    ).toBeDefined();
    expect(
      screen.getByText(
        "A window on top of an agent isn't an OS. An OS runs the work, keeps the memory, sets the permissions, connects agents to each other — and lets you build on it. That's the test, and Compozy is the only one built to pass it."
      )
    ).toBeDefined();

    const shell = screen.getByRole("img", {
      name: "Compozy OS shell with multiple agent workspaces, a task board, and an active loop run.",
    });
    expect(resolveImageAsset(shell.getAttribute("src"))).toBe("/images/landing/os-shell-hero.png");
    expect(screen.getByText(/windows for agent work, a task board/)).toBeDefined();
  });

  it("links the beta install and system exploration paths", () => {
    render(<Hero />);

    expect(screen.getByRole("link", { name: "Install the beta" }).getAttribute("href")).toBe(
      "/runtime/core/getting-started/installation"
    );
    expect(screen.getByRole("link", { name: "Explore the system" }).getAttribute("href")).toBe(
      "/runtime/core/extensions"
    );
  });
});

describe("ExtensibilitySection", () => {
  it("presents extensibility as the second OS criterion", () => {
    render(<ExtensibilitySection />);

    expect(screen.getByRole("heading", { name: "Built to be built on." })).toBeDefined();
    for (const surface of ["Hooks", "Skills", "Automation", "Extensions"]) {
      expect(screen.getByText(surface)).toBeDefined();
    }
    expect(
      screen.getByRole("link", { name: /Read the extensions reference/ }).getAttribute("href")
    ).toBe("/runtime/core/extensions");
  });

  it("uses the shipped skill-contract illustration", () => {
    render(<ExtensibilitySection />);

    const illustration = screen.getByRole("img", {
      name: "A Compozy skill contract shown as a Markdown file with frontmatter and an execution trace.",
    });
    expect(resolveImageAsset(illustration.getAttribute("src"))).toBe(
      "/images/extensibility-skill-contract-v1.png"
    );
  });
});

describe("FeatureWall", () => {
  it("connects the four named responsibilities instead of presenting a feature count", () => {
    render(<FeatureWall />);

    const articles = screen.getAllByRole("article");
    expect(articles).toHaveLength(4);
    for (const [title, label] of [
      ["Run the work", "Sessions"],
      ["Keep the memory", "Memory"],
      ["Set the permissions", "Control plane"],
      ["Connect the agents", "Network"],
    ]) {
      const article = articles.find(candidate =>
        within(candidate).queryByRole("heading", { name: title })
      );
      expect(article, title).toBeDefined();
      if (!article) throw new Error(`Missing feature wall article: ${title}`);
      expect(within(article).getByText(label)).toBeDefined();
    }
  });
});

describe("Comparison", () => {
  it("states only positive product scopes with their approved source paths", () => {
    render(<Comparison />);

    const expectedSources = new Map([
      ["Paperclip", ".resources/paperclip/README.md"],
      ["Smithers", ".resources/smithers/README.md"],
      ["OpenClaw", ".resources/openclaw/README.md"],
      ["T3 Code", ".resources/t3code/README.md"],
    ]);
    for (const [name, source] of expectedSources) {
      expect(screen.getByRole("heading", { name })).toBeDefined();
      expect(screen.getByText(`Source: ${source}`)).toBeDefined();
    }
    expect(screen.queryByText("Letta")).toBeNull();
    expect(screen.queryByText(/None, single agent/)).toBeNull();
  });
});

describe("ProofSection", () => {
  it("offers exactly the three documented beta install paths", () => {
    render(<ProofSection />);

    for (const command of [
      "curl -fsSL https://compozy.com/install.sh | sh",
      "npm install -g @compozy/cli@beta",
      "go install github.com/compozy/compozy@v0.3.0-beta.1",
    ]) {
      expect(screen.getByText(command)).toBeDefined();
    }
    expect(screen.queryByText(/Homebrew/i)).toBeNull();
    expect(screen.getByRole("link", { name: "Install Compozy beta" }).getAttribute("href")).toBe(
      "/runtime/core/getting-started/installation"
    );
  });
});

describe("FinalCta", () => {
  it("ends with install, protocol, and source paths", () => {
    render(<FinalCta />);

    expect(screen.getByRole("link", { name: "Install the beta" }).getAttribute("href")).toBe(
      "/runtime/core/getting-started/installation"
    );
    expect(screen.getByRole("link", { name: "Read the protocol" }).getAttribute("href")).toBe(
      "/protocol"
    );
    expect(
      screen.getByRole("link", { name: "View the source on GitHub" }).getAttribute("href")
    ).toBe(baseOptions.githubUrl);
  });
});

import { baseOptions } from "@/lib/layout.shared";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

// Mock next/link to render as a plain anchor
vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    className,
  }: {
    href: string;
    children: React.ReactNode;
    className?: string;
  }) => (
    <a href={href} className={className}>
      {children}
    </a>
  ),
}));

// Mock next/navigation
vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}));

import { Pill } from "@compozy/ui";
import { AutonomyKernelSection } from "../autonomy-kernel-section";
import { BentoSection } from "../bento-section";
import { BridgesSection } from "../bridges-section";
import { Comparison } from "../comparison";
import { ExtensibilitySection } from "../extensibility-section";
import { FeaturesSection } from "../features-section";
import { FinalCta } from "../final-cta";
import { Hero } from "../hero";
import { InstallSection } from "../install-section";
import { MemoryDreamSection } from "../memory-dream-section";
import { NetworkSection } from "../network-section";
import { BUILTIN_PROVIDER_COUNT, BUILTIN_PROVIDER_INTEGRATIONS } from "../provider-data";
import { SupportedAgents } from "../supported-agents";

import { KIND_MEANING, type NetworkKind } from "../primitives/network-kinds";

// next/image optimization is enabled, so an <img> src is a `/_next/image?url=…`
// URL. The invariant under test is which source asset each section references,
// so resolve the underlying asset from the optimizer URL before asserting.
function resolveImageAsset(src: string | null): string | null {
  if (!src) return src;
  if (!src.startsWith("/_next/image")) return src;
  const url = new URL(src, "http://localhost").searchParams.get("url");
  return url ?? src;
}

function assetSources(): (string | null)[] {
  return screen.getAllByRole("img").map(image => resolveImageAsset(image.getAttribute("src")));
}

describe("Hero", () => {
  it("leads with the locked headline, subhead, and CompozyOS overview CTA", () => {
    render(<Hero />);
    // Locked hero pair (COPY.md §2 Hero Lock). The headline and the subhead ship verbatim, together.
    expect(
      screen.getByRole("heading", { level: 1, name: "The system around the agent, already built." })
    ).toBeDefined();
    expect(
      screen.getByText(
        "One complete environment to create, automate, and supervise agent work, without scripts, plugin chains, or orchestration frameworks."
      )
    ).toBeDefined();
    const install = screen.getByText("Install the beta");
    expect(install.closest("a")?.getAttribute("href")).toBe("/docs/getting-started/installation");
    const overview = screen.getByText("See how CompozyOS works");
    expect(overview.closest("a")?.getAttribute("href")).toBe("/docs");
  });

  it("renders four proof-of-life signal tiles", () => {
    render(<Hero />);
    expect(screen.getByText("Create")).toBeDefined();
    expect(screen.getByText("Automate")).toBeDefined();
    expect(screen.getByText("Supervise")).toBeDefined();
    expect(screen.getByText(`${BUILTIN_PROVIDER_COUNT} built-in providers`)).toBeDefined();
  });

  it("renders the OS shell capture as the hero visual", () => {
    render(<Hero />);

    expect(
      resolveImageAsset(
        screen
          .getByAltText(
            "CompozyOS workspace capture: the Loops window lists three workspace loops beside a Tasks board tracking launch work across Blocked, Queued, and Done."
          )
          .getAttribute("src")
      )
    ).toBe("/images/hero/os-shell-capture-v1.png");
  });
});

describe("FeaturesSection", () => {
  it("renders four illustrated runtime capabilities in a 2x2 grid", () => {
    render(<FeaturesSection />);
    const eyebrows = ["Memory", "Sessions", "Observability", "Automation"];
    for (const label of eyebrows) {
      expect(screen.getByText(label)).toBeDefined();
    }

    expect(screen.getAllByTestId("feature-card")).toHaveLength(4);
    expect(screen.queryByText("Hooks")).toBeNull();
    expect(screen.queryByText("Bridges")).toBeNull();
    expect(screen.queryByText("Skills")).toBeNull();
    expect(screen.getByText("Comes with what you would otherwise build.")).toBeDefined();
  });

  it("uses the four everything illustration assets", () => {
    render(<FeaturesSection />);

    const expectedSources = [
      "/images/everything/illustration_01.png",
      "/images/everything/illustration_02.png",
      "/images/everything/illustration_03.png",
      "/images/everything/illustration_06.png",
    ];

    const sources = assetSources();

    for (const source of expectedSources) {
      expect(sources).toContain(source);
    }
    expect(sources).not.toContain("/images/everything/illustration_04.png");
    expect(sources).not.toContain("/images/everything/illustration_05.png");
  });
});

describe("BentoSection", () => {
  it("renders the five-tile runtime bento with the Extensibility tile", () => {
    render(<BentoSection />);

    expect(screen.getByTestId("bento-grid")).toBeDefined();
    expect(screen.getAllByRole("article")).toHaveLength(5);
    expect(screen.queryByText("The runtime surface in five parts.")).toBeNull();

    for (const label of ["OS Shell", "Network", "Bridges", "Memory", "Extensibility"]) {
      expect(screen.getByText(label)).toBeDefined();
    }
    expect(screen.queryByText("Trace")).toBeNull();
    expect(screen.queryByText("Tool Registry")).toBeNull();

    for (const title of [
      "Batteries included. Every window managed.",
      "Built-in network. Delegate. Deliver. Done.",
      "From anywhere. Into a session.",
      "Memory that compounds.",
      "Every layer. Pluggable.",
    ]) {
      expect(screen.getByRole("heading", { name: title })).toBeDefined();
    }
  });

  it("uses the five active bento illustration assets including extensibility-v2", () => {
    render(<BentoSection />);

    const expectedSources = [
      "/images/bento-illustrations/os-v2.png",
      "/images/bento-illustrations/network-v2.png",
      "/images/bento-illustrations/bridges-v2.png",
      "/images/bento-illustrations/memory-v2.png",
      "/images/bento-illustrations/extensibility-v2.png",
    ];

    const sources = assetSources();

    for (const source of expectedSources) {
      expect(sources).toContain(source);
    }
    expect(sources).not.toContain("/images/bento-illustrations/trace-v2.png");
    expect(sources).not.toContain("/images/bento-illustrations/runtime-v2.png");
  });
});

describe("SupportedAgents", () => {
  it("renders as a compact support strip, not a hero section", () => {
    render(<SupportedAgents />);
    const list = screen.getByRole("list", { name: "Built-in provider integrations" });
    const items = within(list).getAllByRole("listitem");

    expect(items.map(item => item.getAttribute("aria-label"))).toEqual(
      BUILTIN_PROVIDER_INTEGRATIONS.map(provider => provider.name)
    );
    expect(items).toHaveLength(BUILTIN_PROVIDER_COUNT);
  }, 15_000);
});

describe("BridgesSection", () => {
  it("renders the bridge catalog with brand logos and release-safe copy", () => {
    render(<BridgesSection />);
    const expected = [
      "Slack",
      "Discord",
      "Telegram",
      "WhatsApp",
      "Microsoft Teams",
      "Google Chat",
      "GitHub",
      "Linear",
    ];
    for (const name of expected) {
      expect(screen.getByText(name)).toBeDefined();
    }
    expect(
      screen.getByText("Your users work in these channels. Your agents can meet them there.")
    ).toBeDefined();
    expect(
      screen.getByText(
        "Webhooks in, sessions out. Responses stream back to the original thread. No serverless glue, no second runtime, the bridge adapter runs inside the daemon."
      )
    ).toBeDefined();
    expect(screen.queryByText("Your users live on these. Now so do your agents.")).toBeNull();
  });

  it("marks every provider in-tree and states the source-checkout caveat", () => {
    render(<BridgesSection />);
    // All eight providers exist under extensions/bridges/; none is embedded in the released
    // binary. content/docs/bridges/index.mdx is the source of record for both facts.
    expect(screen.getAllByText("in-tree").length).toBe(8);
    expect(screen.queryByText("alpha")).toBeNull();
    expect(screen.queryByText("planned")).toBeNull();
    expect(
      screen.getByText(/build and install the provider from a trusted source checkout/i)
    ).toBeDefined();
  });

  it("links bridge readers to setup and adapter docs", () => {
    render(<BridgesSection />);

    expect(screen.getByRole("link", { name: "Set up a bridge" }).getAttribute("href")).toBe(
      "/docs/bridges/setup"
    );
    expect(screen.getByRole("link", { name: "Build a bridge adapter" }).getAttribute("href")).toBe(
      "/docs/bridges/adding-a-bridge"
    );
  });
});

describe("ExtensibilitySection", () => {
  it("renders six extensibility cards including sandbox and docs link", () => {
    render(<ExtensibilitySection />);
    expect(screen.getAllByRole("article")).toHaveLength(6);
    const eyebrows = ["Hooks", "Skills", "Automation", "Sandbox", "Extensions"];
    for (const label of eyebrows) {
      expect(screen.getByText(label)).toBeDefined();
    }
    expect(screen.getByRole("link", { name: "Read extensions docs" }).getAttribute("href")).toBe(
      "/docs/extensions"
    );
  });

  it("uses the dedicated skill contract illustration for the lower section", () => {
    render(<ExtensibilitySection />);

    expect(
      resolveImageAsset(
        screen
          .getByAltText(
            "deploy-staging.skill.md shown as a Markdown skill contract with frontmatter, deployment capabilities, and a staged execution trace."
          )
          .getAttribute("src")
      )
    ).toBe("/images/extensibility-skill-contract-v1.png");
  });
});

describe("NetworkSection", () => {
  it("renders the protocol walkthrough and supporting cards", () => {
    render(<NetworkSection />);
    expect(screen.getByText("Implemented commands")).toBeDefined();
    expect(screen.getByText("Commit first, dispatch in-process")).toBeDefined();
    expect(screen.getByText("Explicit receipts, durable history")).toBeDefined();
    expect(screen.getByLabelText(/Pause walkthrough|Play walkthrough/)).toBeDefined();
  });
});

describe("InstallSection", () => {
  const goInstallCommand = "go install github.com/compozy/compozy@v9.9.9-beta.9";

  it("shows method-specific bootstrap steps instead of repeating hosted bootstrap", () => {
    render(<InstallSection goInstallCommand={goInstallCommand} />);
    const installer = screen.getByRole("tab", { name: "Installer" });
    const npm = screen.getByRole("tab", { name: "npm" });
    expect(installer).toBeDefined();
    expect(npm).toBeDefined();
    expect(screen.getByRole("tab", { name: "Go" })).toBeDefined();
    expect(screen.getByText("curl -fsSL https://compozy.com/install.sh | sh")).toBeDefined();
    expect(screen.queryByText("Bootstrap your CompozyOS home")).toBeNull();
    expect(screen.getByText("Start the daemon")).toBeDefined();
    expect(screen.getByText("Launch a real session")).toBeDefined();

    fireEvent.click(npm);

    expect(screen.getByText("Bootstrap your CompozyOS home")).toBeDefined();
  });

  it("renders the resolved pinned Go command in the Go panel", () => {
    render(<InstallSection goInstallCommand={goInstallCommand} />);

    fireEvent.click(screen.getByRole("tab", { name: "Go" }));

    expect(screen.getByText(goInstallCommand)).toBeDefined();
  });

  it("wires tab roles, panels, and keyboard navigation", () => {
    render(<InstallSection goInstallCommand={goInstallCommand} />);

    const installer = screen.getByRole("tab", { name: "Installer" });
    const npm = screen.getByRole("tab", { name: "npm" });
    const go = screen.getByRole("tab", { name: "Go" });

    expect(installer.getAttribute("id")).toBe("install-tab-installer");
    expect(installer.getAttribute("aria-controls")).toBe("install-panel-installer");
    expect(installer.getAttribute("tabindex")).toBe("0");
    expect(npm.getAttribute("tabindex")).toBe("-1");
    expect(go.getAttribute("tabindex")).toBe("-1");

    fireEvent.keyDown(installer, { key: "ArrowRight" });

    expect(npm.getAttribute("aria-selected")).toBe("true");
    let panel = screen.getByRole("tabpanel");
    expect(panel.getAttribute("id")).toBe("install-panel-npm");
    expect(panel.getAttribute("aria-labelledby")).toBe("install-tab-npm");

    fireEvent.keyDown(npm, { key: "End" });

    expect(go.getAttribute("aria-selected")).toBe("true");
    panel = screen.getByRole("tabpanel");
    expect(panel.getAttribute("id")).toBe("install-panel-go");

    fireEvent.keyDown(go, { key: "Home" });

    expect(installer.getAttribute("aria-selected")).toBe("true");
    panel = screen.getByRole("tabpanel");
    expect(panel.getAttribute("id")).toBe("install-panel-installer");
  });
});

describe("Comparison", () => {
  it("frames the DIY agent stack against what comes built in, with no named rivals", () => {
    render(<Comparison />);
    expect(screen.getByText("Every piece you would otherwise assemble.")).toBeDefined();
    for (const name of [
      "Loops",
      "Triggers and schedules",
      "Memory",
      "Permissions and approvals",
      "Observability",
      "Agent integration",
    ]) {
      expect(screen.getByText(name)).toBeDefined();
    }
    expect(screen.queryByText("Paperclip")).toBeNull();
    expect(screen.queryByText("Smithers")).toBeNull();
    expect(screen.queryByText("Mastra Factory")).toBeNull();
  });

  it("keeps internal research paths out of the rendered page", () => {
    render(<Comparison />);
    expect(screen.queryByText(/^Source:/)).toBeNull();
    expect(screen.queryByText(/\.resources\//)).toBeNull();
  });
});

describe("MemoryDreamSection", () => {
  it("renders the sticky-rail headline and the numbered consolidation steps", () => {
    render(<MemoryDreamSection />);
    expect(screen.getByText("Memory that compounds")).toBeDefined();
    expect(screen.getByText("while you sleep.")).toBeDefined();
    for (const title of [
      "Memory as scoped Markdown",
      "Time → Sessions → Lock → Signal cascade",
      "Same surface for you and the agent",
    ]) {
      expect(screen.getByText(title)).toBeDefined();
    }
    expect(screen.getByText("01")).toBeDefined();
    expect(screen.getByText("02")).toBeDefined();
    expect(screen.getByText("03")).toBeDefined();
  });

  it("withholds the memory storyboard while its artwork carries the retired wordmark", () => {
    render(<MemoryDreamSection />);
    expect(screen.queryAllByRole("img")).toHaveLength(0);
  });
});

describe("AutonomyKernelSection", () => {
  it("renders the autonomy kernel header and the storyboard image", () => {
    render(<AutonomyKernelSection />);
    expect(screen.getByText("A real autonomy kernel, not a fork-and-pray loop.")).toBeDefined();
    expect(
      resolveImageAsset(
        screen
          .getByAltText(
            "CompozyOS autonomy storyboard, task_runs queue, an agent claiming a run with a claim_token and heartbeat, and lease recovery on daemon restart."
          )
          .getAttribute("src")
      )
    ).toBe("/images/runtime/autonomy-overview-storyboard-v1.png");
  });

  it("renders the asymmetric narrative card and the side-list invariants", () => {
    render(<AutonomyKernelSection />);
    expect(screen.getByText("No double-execution, ever.")).toBeDefined();
    for (const heading of [
      "Daemon crashes don't orphan work.",
      "Operators and agents hit task_runs.",
      "Children cannot widen parents.",
    ]) {
      expect(screen.getByText(heading)).toBeDefined();
    }
  });

  it("links the autonomy guide CTA to the autonomy documentation", () => {
    render(<AutonomyKernelSection />);

    expect(
      screen.getByRole("link", { name: "Read the autonomy kernel guide" }).getAttribute("href")
    ).toBe("/docs/autonomy");
  });
});

describe("FinalCta", () => {
  it("links the final actions to installation, protocol, and the repository", () => {
    render(<FinalCta />);
    const install = screen.getByText("Install the beta");
    expect(install.closest("a")?.getAttribute("href")).toBe("/docs/getting-started/installation");
    const spec = screen.getByText("Read compozy-network/v0 spec");
    expect(spec.closest("a")?.getAttribute("href")).toBe("/docs/network/protocol");
    const star = screen.getByText("Star on GitHub");
    expect(star.closest("a")?.getAttribute("href")).toBe(baseOptions.githubUrl);
  });
});

describe("Network kind pill", () => {
  it("has a meaning string for every NetworkKind and renders inside Pill", () => {
    const kinds: NetworkKind[] = ["greet", "whois", "say", "capability", "receipt", "trace"];
    for (const kind of kinds) {
      expect(KIND_MEANING[kind]).toBeDefined();
      render(
        <Pill mono size="xs" tone="accent" title={KIND_MEANING[kind]}>
          {kind}
        </Pill>
      );
      expect(screen.getAllByText(kind)).toBeDefined();
    }
  });

  it("does not advertise direct as a wire kind", () => {
    const meaningKeys = Object.keys(KIND_MEANING);
    expect(meaningKeys).not.toContain("direct");
  });
});

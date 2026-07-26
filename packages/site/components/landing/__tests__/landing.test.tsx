import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { baseOptions } from "@/lib/layout.shared";

const remotionPlayerMocks = vi.hoisted(() => ({
  player: vi.fn(),
  thumbnail: vi.fn(),
}));

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

vi.mock("@remotion/player", () => ({
  Player: (props: Record<string, unknown>) => {
    remotionPlayerMocks.player(props);
    return <div data-testid="remotion-player" />;
  },
  Thumbnail: (props: Record<string, unknown>) => {
    remotionPlayerMocks.thumbnail(props);
    return <div data-testid="remotion-thumbnail" />;
  },
}));

import { Hero } from "../hero";
import { FeaturesSection } from "../features-section";
import { BentoSection } from "../bento-section";
import { SUPPORTED_AGENT_COUNT, SUPPORTED_AGENT_PROVIDERS } from "../provider-data";
import { SupportedAgents } from "../supported-agents";
import { RuntimeMicroDiagram } from "../runtime-micro-diagram";
import { RuntimeSection } from "../runtime-section";
import { SandboxSection } from "../sandbox-section";
import { BridgesSection } from "../bridges-section";
import { ExtensibilitySection } from "../extensibility-section";
import { NetworkSection } from "../network-section";
import { MemoryDreamSection } from "../memory-dream-section";
import { AutonomyKernelSection } from "../autonomy-kernel-section";
import { InstallSection } from "../install-section";
import { Comparison } from "../comparison";
import { FinalCta } from "../final-cta";
import { Pill } from "@agh/ui";

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
  it("leads with the locked headline, subhead, and agent network protocol CTA", () => {
    render(<Hero />);
    expect(screen.getByText("An open workplace for AI agents.")).toBeDefined();
    expect(
      screen.getByText(
        "AGH runs the agent CLIs you already use as durable sessions, with memory, autonomy, tools, and automation, connected on agh-network/v0 channels where they find each other, share capabilities, and close work with receipts."
      )
    ).toBeDefined();
    expect(screen.getByText(/find each other/)).toBeDefined();
    const install = screen.getByText("Install the runtime");
    expect(install.closest("a")?.getAttribute("href")).toBe(
      "/runtime/core/getting-started/installation"
    );
    const spec = screen.getByText("Read the agh-network/v0 spec");
    expect(spec.closest("a")?.getAttribute("href")).toBe("/protocol");
  });

  it("renders four proof-of-life signal tiles", () => {
    render(<Hero />);
    expect(screen.getByText("agh-network/v0, alpha runtime")).toBeDefined();
    expect(
      screen.getByText("6 message kinds. Commit-first delivery. Audited outcomes.")
    ).toBeDefined();
    expect(screen.getByText(`${SUPPORTED_AGENT_COUNT} ACP drivers supported`)).toBeDefined();
    expect(screen.getByText("Tool registry, one control path")).toBeDefined();
    expect(screen.getByText("Single binary, no infra")).toBeDefined();
  });

  it("starts the Remotion player muted so browser autoplay can advance frames", () => {
    remotionPlayerMocks.player.mockClear();

    render(<Hero />);

    expect(remotionPlayerMocks.player).toHaveBeenCalledWith(
      expect.objectContaining({
        autoPlay: true,
        initiallyMuted: true,
      })
    );
  });
});

describe("FeaturesSection", () => {
  it("renders four illustrated runtime capabilities in a 2x2 grid", () => {
    render(<FeaturesSection />);
    const eyebrows = ["Memory", "Capabilities", "Workspaces", "Automation"];
    for (const label of eyebrows) {
      expect(screen.getByText(label)).toBeDefined();
    }

    expect(screen.getAllByTestId("feature-card")).toHaveLength(4);
    expect(screen.queryByText("Sessions")).toBeNull();
    expect(screen.queryByText("Hooks")).toBeNull();
    expect(screen.queryByText("Bridges")).toBeNull();
    expect(screen.queryByText("Skills")).toBeNull();
    expect(screen.queryByText("Observability")).toBeNull();
    expect(screen.getByText("The runtime your agents already know how to drive.")).toBeDefined();
  });

  it("uses the four everything illustration assets", () => {
    render(<FeaturesSection />);

    const expectedSources = [
      "/images/everything/illustration_02.png",
      "/images/everything/illustration_04.png",
      "/images/everything/illustration_05.png",
      "/images/everything/illustration_06.png",
    ];

    const sources = assetSources();

    for (const source of expectedSources) {
      expect(sources).toContain(source);
    }
    expect(sources).not.toContain("/images/everything/illustration_01.png");
    expect(sources).not.toContain("/images/everything/illustration_03.png");
  });
});

describe("BentoSection", () => {
  it("renders the five-tile runtime bento with the Extensibility tile", () => {
    render(<BentoSection />);

    expect(screen.getByTestId("bento-grid")).toBeDefined();
    expect(screen.getAllByRole("article")).toHaveLength(5);
    expect(screen.queryByText("The runtime surface in five parts.")).toBeNull();

    for (const label of ["Runtime", "Network", "Bridges", "Memory", "Extensibility"]) {
      expect(screen.getByText(label)).toBeDefined();
    }
    expect(screen.queryByText("Trace")).toBeNull();
    expect(screen.queryByText("Tool Registry")).toBeNull();

    for (const title of [
      "Your agents. Under control.",
      "Built-in network. Delegate. Deliver. Done.",
      "From anywhere. Into a session.",
      "Memory that compounds.",
      "Every layer. Pluggable.",
    ]) {
      expect(screen.getByRole("heading", { name: title })).toBeDefined();
    }
  });

  it("uses the five exported bento illustration assets including extensibility-v2", () => {
    render(<BentoSection />);

    const expectedSources = [
      "/images/bento-illustrations/runtime-v2.png",
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
  });
});

describe("SupportedAgents", () => {
  it("renders as a compact support strip, not a hero section", () => {
    render(<SupportedAgents />);
    const list = screen.getByRole("list", { name: "Supported agent CLIs" });
    const items = within(list).getAllByRole("listitem");

    expect(items.map(item => item.getAttribute("aria-label"))).toEqual(
      SUPPORTED_AGENT_PROVIDERS.map(provider => provider.name)
    );
    expect(screen.getByText("Your CLI on the network")).toBeDefined();
  }, 15_000);
});

describe("RuntimeSection", () => {
  it("renders the three runtime feature cards without the replay claim", () => {
    render(<RuntimeSection />);
    const expected = [
      "Durable sessions in SQLite",
      "Three operator surfaces, one daemon",
      "Permission modes with an audit trail",
    ];
    for (const title of expected) {
      expect(screen.getByText(title)).toBeDefined();
    }
    expect(screen.queryByText("Replayable event stream")).toBeNull();
  });

  it("uses the runtime daemon illustration in the sticky rail", () => {
    render(<RuntimeSection />);

    expect(
      resolveImageAsset(
        screen
          .getByAltText(
            "AGH daemon connecting CLI, API, and web UI surfaces to sessions, memory, skills, workspaces, and observability."
          )
          .getAttribute("src")
      )
    ).toBe("/images/runtime/illustration_1.png");
  });

  it("gives the sticky runtime rail a large-screen top inset", () => {
    const { container } = render(<RuntimeSection />);
    const stickyRail = container.querySelector('div[class*="lg:sticky"]');

    expect(stickyRail).toBeTruthy();
    expect(stickyRail?.getAttribute("class")).toContain("lg:top-24");
  });
});

describe("SandboxSection", () => {
  it("renders sandbox positioning and implemented provider copy", () => {
    render(<SandboxSection />);

    expect(screen.getByText("Sandbox")).toBeDefined();
    expect(screen.getByText("Run agents away from the host filesystem.")).toBeDefined();
    expect(screen.getByText("Run on the host when isolation is not needed")).toBeDefined();
    expect(screen.getByText("Move a workspace into a remote sandbox")).toBeDefined();
    expect(screen.getByText("Control how files move")).toBeDefined();
  });

  it("renders the sandbox lifecycle diagram labels", () => {
    render(<SandboxSection />);

    expect(screen.getByLabelText("AGH sandbox lifecycle diagram")).toBeDefined();
    expect(screen.getByText("sandbox_id")).toBeDefined();
    expect(screen.getByText("sandbox_ref")).toBeDefined();
    expect(screen.getByText("sandbox.exec")).toBeDefined();
  });
});

describe("RuntimeMicroDiagram", () => {
  it("injects a prefers-reduced-motion CSS guard for pre-hydration renders", () => {
    const { container } = render(<RuntimeMicroDiagram />);
    const styleText = container.querySelector("style")?.textContent ?? "";

    expect(styleText).toContain("@media (prefers-reduced-motion: reduce)");
    expect(styleText).toContain("animation: none !important;");
  });
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

  it("marks the three alpha bridges separately from the planned batch", () => {
    render(<BridgesSection />);
    expect(screen.getAllByText("alpha").length).toBe(3);
    expect(screen.getAllByText("planned").length).toBe(5);
    expect(screen.queryByText("live")).toBeNull();
    expect(screen.queryByText("next")).toBeNull();
  });

  it("links bridge readers to operator setup and adapter docs", () => {
    render(<BridgesSection />);

    expect(
      screen
        .getByRole("link", { name: "Configure Slack, Discord, or Telegram" })
        .getAttribute("href")
    ).toBe("/runtime/core/bridges/setup");
    expect(screen.getByRole("link", { name: "Build a bridge adapter" }).getAttribute("href")).toBe(
      "/runtime/core/extensions"
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
      "/runtime/core/extensions"
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
    expect(screen.getByText("Receipts are first-class")).toBeDefined();
    expect(screen.getByLabelText(/Pause walkthrough|Play walkthrough/)).toBeDefined();
  });
});

describe("InstallSection", () => {
  it("renders three install tabs and the three CLI steps", () => {
    render(<InstallSection />);
    expect(screen.getByRole("tab", { name: "Homebrew" })).toBeDefined();
    expect(screen.getByRole("tab", { name: "npm" })).toBeDefined();
    expect(screen.getByRole("tab", { name: "Go" })).toBeDefined();
    expect(screen.getByText("brew install compozy/compozy/agh")).toBeDefined();
    expect(screen.getByText("Bootstrap your AGH home")).toBeDefined();
    expect(screen.getByText("Start the daemon")).toBeDefined();
    expect(screen.getByText("Launch a real session")).toBeDefined();
  });

  it("wires tab roles, panels, and keyboard navigation", () => {
    render(<InstallSection />);

    const homebrew = screen.getByRole("tab", { name: "Homebrew" });
    const npm = screen.getByRole("tab", { name: "npm" });
    const go = screen.getByRole("tab", { name: "Go" });

    expect(homebrew.getAttribute("id")).toBe("install-tab-homebrew");
    expect(homebrew.getAttribute("aria-controls")).toBe("install-panel-homebrew");
    expect(homebrew.getAttribute("tabindex")).toBe("0");
    expect(npm.getAttribute("tabindex")).toBe("-1");
    expect(go.getAttribute("tabindex")).toBe("-1");

    fireEvent.keyDown(homebrew, { key: "ArrowRight" });

    expect(npm.getAttribute("aria-selected")).toBe("true");
    let panel = screen.getByRole("tabpanel");
    expect(panel.getAttribute("id")).toBe("install-panel-npm");
    expect(panel.getAttribute("aria-labelledby")).toBe("install-tab-npm");

    fireEvent.keyDown(npm, { key: "End" });

    expect(go.getAttribute("aria-selected")).toBe("true");
    panel = screen.getByRole("tabpanel");
    expect(panel.getAttribute("id")).toBe("install-panel-go");

    fireEvent.keyDown(go, { key: "Home" });

    expect(homebrew.getAttribute("aria-selected")).toBe("true");
    panel = screen.getByRole("tabpanel");
    expect(panel.getAttribute("id")).toBe("install-panel-homebrew");
  });
});

describe("Comparison", () => {
  it("renders the four named approaches and the agent support column", () => {
    render(<Comparison />);
    expect(screen.getByText("Other tools stop at the runtime boundary.")).toBeDefined();
    for (const name of ["Letta", "LangGraph / CrewAI", "OpenAI Assistants / Devin", "AGH"]) {
      expect(screen.getByText(name)).toBeDefined();
    }
    expect(screen.getByText("None, single agent")).toBeDefined();
    expect(screen.getByText("agh-network/v0, implemented")).toBeDefined();
    expect(screen.getByText(`${SUPPORTED_AGENT_COUNT} ACP drivers`)).toBeDefined();
  });
});

describe("MemoryDreamSection", () => {
  it("renders the sticky-rail headline and the numbered consolidation steps", () => {
    render(<MemoryDreamSection />);
    expect(screen.getByText("Memory that compounds")).toBeDefined();
    expect(screen.getByText("while you sleep.")).toBeDefined();
    for (const title of [
      "Memory as scoped Markdown",
      "Time → Sessions → Lock cascade",
      "Same surface for you and the agent",
    ]) {
      expect(screen.getByText(title)).toBeDefined();
    }
    expect(screen.getByText("01")).toBeDefined();
    expect(screen.getByText("02")).toBeDefined();
    expect(screen.getByText("03")).toBeDefined();
  });

  it("renders the memory storyboard illustration in the sticky rail", () => {
    render(<MemoryDreamSection />);
    expect(
      resolveImageAsset(
        screen
          .getByAltText(
            "AGH memory interface diagram showing scoped Markdown files, memory indexing, and dream consolidation into durable memory."
          )
          .getAttribute("src")
      )
    ).toBe("/images/runtime/memory-dream-landing-v1.png");
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
            "AGH autonomy storyboard, task_runs queue, an agent claiming a run with a claim_token and heartbeat, and lease recovery on daemon restart."
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
});

describe("FinalCta", () => {
  it("renders the final CTAs and drops the old hedge copy", () => {
    render(<FinalCta />);
    expect(screen.getByText("Install AGH. Run a session. Join the network.")).toBeDefined();
    const install = screen.getByText("Install AGH");
    expect(install.closest("a")?.getAttribute("href")).toBe(
      "/runtime/core/getting-started/installation"
    );
    const spec = screen.getByText("Read agh-network/v0 spec");
    expect(spec.closest("a")?.getAttribute("href")).toBe("/protocol");
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

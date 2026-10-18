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

import { AutonomyKernelSection } from "../autonomy-kernel-section";
import { Comparison } from "../comparison";
import { ExtensibilitySection } from "../extensibility-section";
import { FinalCta } from "../final-cta";
import { Hero } from "../hero";
import { InstallSection } from "../install-section";
import { BUILTIN_PROVIDER_COUNT, BUILTIN_PROVIDER_INTEGRATIONS } from "../provider-data";
import { SupportedAgents } from "../supported-agents";

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

describe("ExtensibilitySection", () => {
  it("renders extensibility cards and docs link", () => {
    render(<ExtensibilitySection />);
    expect(screen.getAllByRole("article")).toHaveLength(5);
    const eyebrows = ["Hooks", "Skills", "Automation", "Extensions"];
    for (const label of eyebrows) {
      expect(screen.getByText(label)).toBeDefined();
    }
    expect(screen.getByRole("link", { name: "Read extensions docs" }).getAttribute("href")).toBe(
      "/docs/extensions"
    );
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
  it("keeps internal research paths out of the rendered page", () => {
    render(<Comparison />);
    expect(screen.queryByText(/^Source:/)).toBeNull();
    expect(screen.queryByText(/\.resources\//)).toBeNull();
  });
});

describe("AutonomyKernelSection", () => {
  it("links the autonomy guide CTA to the autonomy documentation", () => {
    render(<AutonomyKernelSection />);

    expect(
      screen.getByRole("link", { name: "Read the autonomy kernel guide" }).getAttribute("href")
    ).toBe("/docs/autonomy");
  });
});

describe("FinalCta", () => {
  it("links the final actions to installation guidance and the repository", () => {
    render(<FinalCta />);
    const install = screen.getByText("Install the beta");
    expect(install.closest("a")?.getAttribute("href")).toBe("/docs/getting-started/installation");
    const spec = screen.getByText("Read the installation guide");
    expect(spec.closest("a")?.getAttribute("href")).toBe("/docs/getting-started/installation");
    const star = screen.getByText("Star on GitHub");
    expect(star.closest("a")?.getAttribute("href")).toBe(baseOptions.githubUrl);
  });
});

import { render, screen } from "@testing-library/react";
import { Bot, Code } from "lucide-react";
import { describe, expect, it } from "vitest";

import { KindIcon } from "../kind-icon";
import { providerKindIconRegistry } from "../kind-icon-registry";

const providerKeys = [
  "blackbox",
  "claude",
  "cline",
  "codex",
  "gemini",
  "goose",
  "hermes",
  "junie",
  "kimi-cli",
  "ollama",
  "openai",
  "openclaw",
  "openhands",
  "qoder",
  "qwen-code",
] as const;

describe("KindIcon", () => {
  it("Should expose the shared provider registry with all supported provider keys", () => {
    for (const provider of providerKeys) {
      expect(providerKindIconRegistry).toHaveProperty(provider);
    }
  });

  it("Should render each supported provider kind with normalized metadata", () => {
    for (const provider of providerKeys) {
      const { unmount } = render(<KindIcon kind={provider} data-testid={`kind-${provider}`} />);
      const icon = screen.getByTestId(`kind-${provider}`);
      expect(icon).toHaveAttribute("data-slot", "kind-icon");
      expect(icon).toHaveAttribute("data-kind", provider);
      expect(icon.querySelector("svg")).toBeInTheDocument();
      unmount();
    }
  });

  it("Should normalize casing and whitespace before reading the registry", () => {
    render(<KindIcon kind=" Qwen-Code " data-testid="icon" />);

    expect(screen.getByTestId("icon")).toHaveAttribute("data-kind", "qwen-code");
  });

  it("Should render the fallback icon for unknown kinds", () => {
    render(<KindIcon kind="unknown-provider" fallback={Code} data-testid="icon" />);

    const icon = screen.getByTestId("icon");
    expect(icon).toHaveAttribute("data-kind", "unknown-provider");
    expect(icon.querySelector("svg")).toBeInTheDocument();
  });

  it("Should support caller registries without changing the primitive", () => {
    render(
      <KindIcon
        kind="custom"
        registry={{ custom: Bot }}
        tone="accent"
        size="md"
        className="custom-class"
        data-testid="icon"
      />
    );

    const icon = screen.getByTestId("icon");
    expect(icon.className).toContain("custom-class");
    expect(icon.className).toContain("size-5");
    expect(icon.className).toContain("text-accent");
  });
});

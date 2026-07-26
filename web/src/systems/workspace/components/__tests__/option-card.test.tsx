import { Home } from "lucide-react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Button, Pill } from "@agh/ui";

import { OptionCard } from "../option-card";

function renderCard(action?: React.ReactNode) {
  return render(
    <OptionCard size="comfortable" data-testid="option-card-root">
      <OptionCard.Header eyebrow="Global" right={<Pill tone="accent">HOME</Pill>} />
      <OptionCard.Body>
        <OptionCard.Icon tone="accent" data-testid="option-card-icon">
          <Home className="size-4" />
        </OptionCard.Icon>
        <OptionCard.Content>
          <OptionCard.Title>Use global workspace</OptionCard.Title>
          <OptionCard.Description data-testid="option-card-description">
            Resolve the daemon's home workspace.
          </OptionCard.Description>
          <OptionCard.Meta data-testid="option-card-meta">/Users/pedro</OptionCard.Meta>
        </OptionCard.Content>
      </OptionCard.Body>
      <OptionCard.Action data-testid="option-card-action">{action}</OptionCard.Action>
    </OptionCard>
  );
}

describe("OptionCard", () => {
  it("Should render the eyebrow + right slot through Section chrome", () => {
    renderCard();

    const root = screen.getByTestId("option-card-root");
    expect(root.dataset.size).toBe("comfortable");
    expect(screen.getByText("Global")).toBeInTheDocument();
    expect(screen.getByText("HOME")).toBeInTheDocument();
  });

  it("Should apply the compact size padding when size=compact", () => {
    render(
      <OptionCard size="compact" data-testid="option-card-root">
        <OptionCard.Body>
          <OptionCard.Content>
            <OptionCard.Title>Compact</OptionCard.Title>
          </OptionCard.Content>
        </OptionCard.Body>
      </OptionCard>
    );

    const root = screen.getByTestId("option-card-root");
    expect(root.dataset.size).toBe("compact");
  });

  it("Should expose tone via data attribute on the icon slot", () => {
    renderCard();

    const icon = screen.getByTestId("option-card-icon");
    expect(icon.dataset.tone).toBe("accent");
  });

  it("Should render the action button and trigger its handler when clicked", async () => {
    const onClick = vi.fn();
    const user = userEvent.setup();

    renderCard(<Button onClick={onClick}>Use this workspace</Button>);

    await user.click(screen.getByRole("button", { name: "Use this workspace" }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("Should render the meta paragraph when provided", () => {
    renderCard();

    expect(screen.getByTestId("option-card-meta").textContent).toBe("/Users/pedro");
  });

  it("Should throw when slots are rendered outside the OptionCard root", () => {
    const originalError = console.error;
    try {
      console.error = () => {};
      expect(() =>
        render(
          <OptionCard.Body>
            <OptionCard.Title>orphan</OptionCard.Title>
          </OptionCard.Body>
        )
      ).toThrow(/OptionCard\.Body must be used inside <OptionCard>/);
    } finally {
      console.error = originalError;
    }
  });
});

import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RestartBanner } from "../restart-banner";

describe("RestartBanner", () => {
  it("Should render the warning tone chrome with the default message", () => {
    const { container } = render(<RestartBanner />);
    const root = container.querySelector<HTMLElement>('[data-slot="restart-banner"]');
    expect(root?.dataset.tone).toBe("warning");
    expect(root?.getAttribute("data-variant")).toBe("warning");
    expect(root?.getAttribute("role")).toBe("status");

    const message = container.querySelector<HTMLElement>('[data-slot="restart-banner-message"]');
    expect(message?.textContent).toContain("Restart required to apply.");
    expect(container.querySelector('[data-slot="restart-banner-action"]')).not.toBeInTheDocument();
  });

  it("Should render the action button and invoke restartNow on click", () => {
    const restartNow = vi.fn();
    render(<RestartBanner restartNow={restartNow} />);
    const action = screen.getByRole("button", { name: "Restart daemon" });
    expect(action).toBeEnabled();
    fireEvent.click(action);
    expect(restartNow).toHaveBeenCalledTimes(1);
  });

  it("Should swap the action label to Starting... and disable it while pending", () => {
    render(<RestartBanner restartNow={vi.fn()} isPending />);
    const action = screen.getByRole("button", { name: "Starting..." });
    expect(action).toBeDisabled();
    const root = action.closest('[data-slot="restart-banner"]');
    expect(root?.getAttribute("data-pending")).toBe("true");
  });

  it("Should accept a custom message via the message prop", () => {
    render(<RestartBanner message="Provider config changed. Restart to apply." />);
    expect(screen.getByText("Provider config changed. Restart to apply.")).toBeInTheDocument();
  });

  it("Should swap to the info tone with a spinner when busy", () => {
    const { container } = render(<RestartBanner tone="info" busy message="Restarting daemon" />);
    const root = container.querySelector<HTMLElement>('[data-slot="restart-banner"]');
    expect(root?.dataset.tone).toBe("info");
    expect(root?.dataset.busy).toBe("true");
    expect(root?.getAttribute("data-variant")).toBe("info");
    const icon = container.querySelector('[data-slot="restart-banner-icon"]');
    expect(icon?.getAttribute("class") ?? "").toContain("animate-spin");
  });

  it("Should render the danger tone with role=alert and a dismiss button when onDismiss is provided", () => {
    const onDismiss = vi.fn();
    const { container } = render(
      <RestartBanner tone="danger" message="Daemon restart failed" onDismiss={onDismiss} />
    );
    const root = container.querySelector<HTMLElement>('[data-slot="restart-banner"]');
    expect(root?.dataset.tone).toBe("danger");
    expect(root?.getAttribute("role")).toBe("alert");
    const dismiss = screen.getByRole("button", { name: "Dismiss" });
    fireEvent.click(dismiss);
    expect(onDismiss).toHaveBeenCalledTimes(1);
  });

  it("Should render detail content alongside the message", () => {
    const { container } = render(
      <RestartBanner detail={<span data-testid="detail-chip">op_42</span>} />
    );
    expect(screen.getByTestId("detail-chip")).toBeInTheDocument();
    const detail = container.querySelector<HTMLElement>('[data-slot="restart-banner-detail"]');
    expect(detail).not.toBeNull();
  });
});

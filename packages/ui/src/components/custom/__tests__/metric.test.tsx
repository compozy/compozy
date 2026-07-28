import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Metric, type MetricTone } from "../metric";

describe("Metric", () => {
  it("Should render label and value", () => {
    const { container } = render(<Metric label="Sessions" value="12" />);
    const root = container.querySelector<HTMLElement>('[data-slot="metric"]');
    expect(root).not.toBeNull();

    const label = container.querySelector<HTMLElement>('[data-slot="metric-label"]');
    expect(label?.textContent).toBe("Sessions");

    const value = container.querySelector<HTMLElement>('[data-slot="metric-value"]');
    expect(value?.textContent).toBe("12");
  });

  it.each<{ tone: MetricTone; className: string }>([
    { tone: "success", className: "text-success" },
    { tone: "danger", className: "text-danger" },
    { tone: "warning", className: "text-warning" },
    { tone: "accent", className: "text-accent" },
  ])("Should apply the $tone semantic color utility to the value", ({ tone, className }) => {
    const { container } = render(<Metric label="Signal" value="08" tone={tone} />);
    const value = container.querySelector<HTMLElement>('[data-slot="metric-value"]');
    expect(value?.className).toContain(className);
    const root = container.querySelector<HTMLElement>('[data-slot="metric"]');
    expect(root).not.toBeNull();
    expect(container.querySelector('[data-slot="metric"]')?.getAttribute("data-tone")).toBe(tone);
  });

  it("Should default the value color utility to text-fg", () => {
    const { container } = render(<Metric label="Sessions" value="12" />);
    const value = container.querySelector<HTMLElement>('[data-slot="metric-value"]');
    expect(value?.className).toContain("text-fg");
  });

  it("Should render the optional detail slot inline with the value", () => {
    const { container } = render(<Metric label="Credits" value="56.4%" detail="+4.2%" />);
    const detail = container.querySelector<HTMLElement>('[data-slot="metric-detail"]');
    expect(detail?.textContent).toBe("+4.2%");
  });

  it("Should render the optional subtext line", () => {
    const { container } = render(
      <Metric label="Credits" value="56.4%" subtext="Across 8 sessions" />
    );
    const subtext = container.querySelector<HTMLElement>('[data-slot="metric-subtext"]');
    expect(subtext?.textContent).toBe("Across 8 sessions");
  });

  it("Should omit the detail and subtext slots when those props are absent", () => {
    const { container } = render(<Metric label="Credits" value="0" />);
    expect(container.querySelector('[data-slot="metric-detail"]')).toBeNull();
    expect(container.querySelector('[data-slot="metric-subtext"]')).toBeNull();
  });

  it("Should render an eyebrow-cased label distinct from the sentence label", () => {
    const { container } = render(<Metric labelCase="eyebrow" label="Active runs" value="18" />);
    const label = container.querySelector('[data-slot="metric-label"]');
    expect(label?.textContent).toBe("Active runs");
    expect(label?.className).not.toContain("text-form-label");
  });

  it("Should reflect the compact size and value tier via data-size", () => {
    const { container } = render(<Metric size="compact" label="Queue" value="142" />);
    expect(container.querySelector('[data-slot="metric"]')?.getAttribute("data-size")).toBe(
      "compact"
    );
    const value = container.querySelector('[data-slot="metric-value"]');
    expect(value?.className).toContain("text-kpi-compact");
  });

  it("Should render the head icon and trailing slots when provided", () => {
    function Dot() {
      return <svg data-testid="metric-head-icon" />;
    }
    const { container } = render(
      <Metric label="Signal" value="8" icon={Dot} trailing={<span>t</span>} />
    );
    expect(container.querySelector('[data-slot="metric-icon"]')).not.toBeNull();
    expect(container.querySelector('[data-slot="metric-trailing"]')).not.toBeNull();
  });
});

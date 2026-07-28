import { render, waitFor } from "@testing-library/react";
import { cloneElement, isValidElement, type ReactElement, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

// Force recharts ResponsiveContainer to inject a deterministic test size into its child chart.
// jsdom returns 0 for layout boxes, which otherwise yields an empty chart and zero rendered cells.
vi.mock("recharts", async () => {
  const actual = await vi.importActual<typeof import("recharts")>("recharts");
  function MockResponsiveContainer({ children }: { children: ReactNode }) {
    if (isValidElement(children)) {
      const element = children as ReactElement<{
        width?: number | string;
        height?: number | string;
      }>;
      return cloneElement(element, { width: 320, height: 96 });
    }
    return <div style={{ width: 320, height: 96 }}>{children}</div>;
  }
  return { ...actual, ResponsiveContainer: MockResponsiveContainer };
});

import { QueueHealthSparkline, type QueueHealthSparklineBucket } from "../queue-health-sparkline";

const SAMPLE: QueueHealthSparklineBucket[] = [
  { label: "23h", value: 4 },
  { label: "22h", value: 6 },
  { label: "21h", value: 2 },
  { label: "20h", value: 8, stuck: true },
];

describe("QueueHealthSparkline", () => {
  it("Should render the container at the provided height and surface aria metadata", () => {
    const { container } = render(
      <QueueHealthSparkline data={SAMPLE} height={72} ariaLabel="Queue depth last 24 hours" />
    );
    const root = container.querySelector<HTMLElement>('[data-slot="queue-health-sparkline"]');
    expect(root?.style.height).toBe("72px");
    expect(root?.getAttribute("role")).toBe("img");
    expect(root?.getAttribute("aria-label")).toBe("Queue depth last 24 hours");
  });

  it("Should paint default cells with --bar-fill and stuck cells with --accent-tint-strong", async () => {
    const { container } = render(<QueueHealthSparkline data={SAMPLE} />);
    const queryCells = () =>
      Array.from(
        container.querySelectorAll<SVGPathElement>('[data-slot="queue-health-sparkline-cell"]')
      );

    await waitFor(
      () => {
        expect(queryCells()).toHaveLength(SAMPLE.length);
      },
      { timeout: 5_000 }
    );

    const cells = queryCells();
    expect(cells).toHaveLength(SAMPLE.length);
    expect(cells[0]?.getAttribute("fill")).toBe("var(--color-bar-fill)");
    expect(cells[1]?.getAttribute("fill")).toBe("var(--color-bar-fill)");
    expect(cells[2]?.getAttribute("fill")).toBe("var(--color-bar-fill)");
    expect(cells[3]?.getAttribute("fill")).toBe("var(--color-accent-tint-strong)");
    expect(cells[3]?.getAttribute("data-stuck")).toBe("true");
    expect(cells[0]?.getAttribute("data-stuck")).toBeNull();
  });
});

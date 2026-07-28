import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { StatusLine } from "../status-line";

describe("StatusLine", () => {
  it("Should render the ConnectionIndicator plus typed items with tone data attribute", () => {
    const { container } = render(
      <StatusLine
        status="connected"
        items={[
          { key: "sessions", label: "sessions", value: "12", tone: "neutral" },
          { key: "agents", label: "agents", value: "3", tone: "info" },
          { key: "workspace", value: "workspace · launch", tone: "success" },
        ]}
      />
    );

    const root = container.querySelector<HTMLElement>('[data-slot="status-line"]');
    expect(root?.dataset.status).toBe("connected");
    expect(root?.querySelector('[data-slot="connection-indicator"]')).not.toBeNull();

    const items = container.querySelectorAll<HTMLElement>('[data-slot="status-line-item"]');
    expect(items).toHaveLength(3);
    expect(items[0]?.dataset.tone).toBe("neutral");
    expect(items[1]?.dataset.tone).toBe("info");
    expect(items[2]?.dataset.tone).toBe("success");

    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("workspace · launch")).toBeInTheDocument();
  });

  it("Should render labels only when present", () => {
    const { container } = render(
      <StatusLine
        status="connected"
        items={[
          { key: "sessions", label: "sessions", value: "12", tone: "neutral" },
          { key: "unlabeled", value: "no-label", tone: "info" },
        ]}
      />
    );

    const labels = container.querySelectorAll<HTMLElement>('[data-slot="status-line-item-label"]');
    expect(labels).toHaveLength(1);
    expect(labels[0]?.textContent).toBe("sessions");
  });

  it("Should default to neutral tone when an item omits the tone field", () => {
    const { container } = render(
      <StatusLine status="connecting" items={[{ key: "sync", value: "resyncing…" }]} />
    );
    const item = container.querySelector<HTMLElement>('[data-slot="status-line-item"]');
    expect(item?.dataset.tone).toBe("neutral");
  });
});

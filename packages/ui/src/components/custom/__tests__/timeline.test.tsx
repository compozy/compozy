import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Timeline } from "../timeline";
import { TimelineEvent } from "../timeline-event";

describe("Timeline", () => {
  it("Should render events in order with markers", () => {
    const { container } = render(
      <Timeline ariaLabel="Run history">
        <TimelineEvent title="Started" tone="info" time="03:00" />
        <TimelineEvent title="Completed" tone="success" time="03:05" />
      </Timeline>
    );
    expect(screen.getByLabelText("Run history")).toBeInTheDocument();
    const markers = container.querySelectorAll('[data-slot="timeline-event-marker"]');
    expect(markers).toHaveLength(2);
    const items = Array.from(container.querySelectorAll('[data-slot="timeline-event"]'));
    expect(items).toHaveLength(2);
    expect(items[0].textContent).toContain("Started");
    expect(items[1].textContent).toContain("Completed");
  });

  it("Should use a flow container for a composed title", () => {
    const { container } = render(
      <TimelineEvent
        title={
          <div>
            <span>Accepted</span>
            <span>Controller</span>
          </div>
        }
      />
    );

    const title = container.querySelector('[data-slot="timeline-event-title"]');
    expect(title?.tagName).toBe("DIV");
    expect(title).toHaveTextContent("AcceptedController");
  });
});

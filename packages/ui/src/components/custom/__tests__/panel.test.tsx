import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Panel } from "../panel";

describe("Panel", () => {
  it("Should render head slots with title, meta, and right content", () => {
    const { container } = render(
      <Panel title="Queue health" meta="14d" right={<button type="button">Open</button>}>
        body
      </Panel>
    );

    expect(container.querySelector('[data-slot="panel-title"]')?.textContent).toBe("Queue health");
    expect(container.querySelector('[data-slot="panel-meta"]')?.textContent).toBe("14d");
    expect(container.querySelector('[data-slot="panel-right"]')?.textContent).toBe("Open");
    expect(container.querySelector('[data-slot="panel-body"]')?.textContent).toBe("body");
  });

  it("Should omit the head entirely when no head slot is provided", () => {
    const { container } = render(<Panel>rows</Panel>);

    expect(container.querySelector('[data-slot="panel-head"]')).toBeNull();
    expect(container.querySelector('[data-slot="panel-body"]')?.textContent).toBe("rows");
  });

  it("Should pin the foot row after the body", () => {
    const { container } = render(<Panel foot={<a href="/tasks">View tasks</a>}>rows</Panel>);

    const body = container.querySelector('[data-slot="panel-body"]');
    const foot = container.querySelector('[data-slot="panel-foot"]');
    expect(foot?.textContent).toBe("View tasks");
    expect(body?.nextElementSibling).toBe(foot);
  });

  it("Should override body padding through bodyClassName", () => {
    const { container } = render(<Panel bodyClassName="p-0">rows</Panel>);

    expect(container.querySelector('[data-slot="panel-body"]')?.className).toContain("p-0");
  });
});

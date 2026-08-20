import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "../../button";
import { Dock } from "../dock";

describe("Dock", () => {
  it("Should render the decision anatomy: eyebrow head, mono subject, actions with key chips", () => {
    const { container } = render(
      <Dock>
        <Dock.Head>
          <Dock.Eyebrow>Permission</Dock.Eyebrow>
          <Dock.Title>Run a command</Dock.Title>
          <Dock.Count>1/2</Dock.Count>
        </Dock.Head>
        <Dock.Body>
          <Dock.Pre>bun install</Dock.Pre>
          <Dock.Meta>
            Bash · workspace <code>compozy</code>
          </Dock.Meta>
        </Dock.Body>
        <Dock.Actions>
          <Button size="sm">
            Allow once
            <Dock.Key>1</Dock.Key>
          </Button>
        </Dock.Actions>
      </Dock>
    );

    expect(container.querySelector('[data-slot="dock"]')).not.toBeNull();
    const eyebrow = screen.getByText("Permission");
    expect(eyebrow).toHaveAttribute("data-slot", "dock-eyebrow");
    expect(eyebrow.className).toContain("eyebrow");
    expect(container.querySelector('[data-slot="dock-pre"]')?.textContent).toBe("bun install");
    expect(container.querySelector('[data-slot="dock-count"]')?.textContent).toBe("1/2");
    const key = container.querySelector<HTMLElement>('[data-slot="dock-key"]');
    expect(key?.tagName).toBe("KBD");
    expect(key?.textContent).toBe("1");
  });

  // KEEP: DESIGN.md §3 — the dock reserves mono for code and key caps; the deadline is prose.
  it("Should render the deadline hint as static sans text with tabular figures", () => {
    const { container } = render(
      <Dock>
        <Dock.Head>
          <Dock.Eyebrow>Question</Dock.Eyebrow>
          <Dock.Deadline>times out 14:32</Dock.Deadline>
        </Dock.Head>
      </Dock>
    );
    const deadline = container.querySelector<HTMLElement>('[data-slot="dock-deadline"]');

    expect(deadline?.textContent).toBe("times out 14:32");
    expect(deadline?.className).not.toContain("font-mono");
    expect(deadline?.className).toContain("tabular-nums");
  });

  it("Should tone the status line danger only when asked", () => {
    const { container, rerender } = render(<Dock.Status>Sending answer…</Dock.Status>);
    let status = container.querySelector<HTMLElement>('[data-slot="dock-status"]');
    expect(status).toHaveAttribute("data-tone", "neutral");
    expect(status?.className).not.toContain("text-danger");

    rerender(<Dock.Status tone="danger">Could not send the answer.</Dock.Status>);
    status = container.querySelector<HTMLElement>('[data-slot="dock-status"]');
    expect(status).toHaveAttribute("data-tone", "danger");
    expect(status?.className).toContain("text-danger");
  });
});

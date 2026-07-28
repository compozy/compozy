import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Progress, ProgressLabel } from "../progress";

function getProgressIndicator(container: HTMLElement): HTMLElement {
  const indicator = container.querySelector('[data-slot="progress-indicator"]');
  if (!(indicator instanceof HTMLElement)) {
    throw new Error("Expected progress indicator to render");
  }
  return indicator;
}

describe("Progress", () => {
  it("Should apply the requested tone without leaking the accent background", () => {
    const { container } = render(
      <Progress tone="success" value={42}>
        <ProgressLabel>Uploading dataset</ProgressLabel>
      </Progress>
    );

    const indicator = getProgressIndicator(container);
    expect(indicator).toHaveClass("bg-success");
    expect(indicator).not.toHaveClass("bg-accent");
  });

  it("Should keep accent as the default indicator tone", () => {
    const { container } = render(
      <Progress value={64}>
        <ProgressLabel>Uploading dataset</ProgressLabel>
      </Progress>
    );

    expect(getProgressIndicator(container)).toHaveClass("bg-accent");
  });
});

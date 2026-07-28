import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Kbd, KbdGroup } from "../kbd";

describe("Kbd", () => {
  it("Should mount a Kbd root with the stable data-slot", () => {
    const { container } = render(<Kbd>K</Kbd>);
    expect(container.querySelector('[data-slot="kbd"]')).not.toBeNull();
  });

  it("Should mount a KbdGroup root with the stable data-slot", () => {
    const { container } = render(
      <KbdGroup>
        <Kbd>⌘</Kbd>
        <Kbd>K</Kbd>
      </KbdGroup>
    );
    expect(container.querySelector('[data-slot="kbd-group"]')).not.toBeNull();
  });
});

import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import userEvent from "@testing-library/user-event";

import {
  ChoiceFree,
  ChoiceFreeBar,
  ChoiceFreeTextarea,
  ChoiceHint,
  ChoiceKey,
  ChoiceList,
  ChoiceRow,
} from "../choice";

describe("Choice", () => {
  it("Should render bounded choices as buttons with numeric key chips and hints", async () => {
    const onSelect = vi.fn();
    render(
      <ChoiceList>
        <ChoiceRow onClick={onSelect}>
          <ChoiceKey>1</ChoiceKey>
          packages/ui (shared primitive)
          <ChoiceHint>recommended</ChoiceHint>
        </ChoiceRow>
      </ChoiceList>
    );

    const row = screen.getByRole("button", {
      name: /packages\/ui \(shared primitive\)/,
    });
    expect(row).toHaveAttribute("data-slot", "choice-row");
    expect(row.getAttribute("type")).toBe("button");
    const key = row.querySelector<HTMLElement>('[data-slot="choice-key"]');
    expect(key?.tagName).toBe("KBD");
    expect(key?.textContent).toBe("1");

    await userEvent.click(row);
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it("Should render the free-text form shell with textarea and bar", () => {
    const { container } = render(
      <ChoiceFree>
        <ChoiceFreeTextarea placeholder="Type your answer…" />
        <ChoiceFreeBar>
          <span className="flex-1" />
          <button type="button">Send answer</button>
        </ChoiceFreeBar>
      </ChoiceFree>
    );

    expect(container.querySelector('[data-slot="choice-free"]')).not.toBeNull();
    const textarea = screen.getByPlaceholderText("Type your answer…");
    expect(textarea).toHaveAttribute("data-slot", "choice-free-textarea");
    expect(screen.getByRole("button", { name: "Send answer" })).not.toBeNull();
  });

  it("Should forward aria-pressed to mark the submitted choice", () => {
    render(
      <ChoiceRow aria-pressed="true">
        <ChoiceKey>2</ChoiceKey>
        web/src/systems/session
      </ChoiceRow>
    );

    expect(screen.getByRole("button")).toHaveAttribute("aria-pressed", "true");
  });
});

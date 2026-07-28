import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SearchInput } from "../search-input";

describe("SearchInput", () => {
  it("Should fire onChange with the accumulated string on each keystroke", async () => {
    const user = userEvent.setup();
    const handle = vi.fn();
    render(<SearchInput onChange={handle} placeholder="Search workspaces" />);

    const input = screen.getByPlaceholderText<HTMLInputElement>("Search workspaces");
    await user.type(input, "ab");

    expect(handle).toHaveBeenCalledTimes(2);
    expect(handle).toHaveBeenNthCalledWith(1, "a");
    expect(handle).toHaveBeenNthCalledWith(2, "ab");
    expect(input.value).toBe("ab");
  });

  it("Should render the kbd slot when provided", () => {
    const { container } = render(
      <SearchInput value="" onChange={() => {}} kbd={<span>⌘K</span>} />
    );
    const kbd = container.querySelector('[data-slot="search-input-kbd"]');
    expect(kbd).not.toBeNull();
    expect(kbd?.textContent).toContain("⌘K");
  });

  it("Should mark the control disabled when disabled is true", () => {
    const { container } = render(<SearchInput value="" onChange={() => {}} disabled />);
    const control = container.querySelector<HTMLInputElement>('[data-slot="search-input-control"]');
    expect(control?.disabled).toBe(true);
    expect(container.querySelector('[data-slot="search-input"]')).toHaveAttribute(
      "data-disabled",
      "true"
    );
  });
});

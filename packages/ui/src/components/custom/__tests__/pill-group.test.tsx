import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { PillGroup } from "../pill-group";

describe("PillGroup", () => {
  const items = [
    { value: "list", label: "List" },
    { value: "kanban", label: "Kanban" },
    { value: "inbox", label: "Inbox", badge: 3 },
  ] as const;

  it("Should fire onChange with the selected value when a non-active item is clicked", async () => {
    const user = userEvent.setup();
    const handle = vi.fn();
    render(<PillGroup value="list" onChange={handle} items={items} />);

    await user.click(screen.getByRole("button", { name: /kanban/i }));

    expect(handle).toHaveBeenCalledWith("kanban");
  });

  it("Should not fire onChange when the active item is re-clicked", async () => {
    const user = userEvent.setup();
    const handle = vi.fn();
    render(<PillGroup value="list" onChange={handle} items={items} />);

    await user.click(screen.getByRole("button", { name: /list/i }));

    expect(handle).not.toHaveBeenCalled();
  });

  it("Should reflect the active item via aria-pressed and data-active", () => {
    render(<PillGroup value="kanban" onChange={() => {}} items={items} />);
    const kanban = screen.getByRole("button", { name: /kanban/i });
    const list = screen.getByRole("button", { name: /list/i });
    expect(kanban).toHaveAttribute("aria-pressed", "true");
    expect(kanban).toHaveAttribute("data-active", "true");
    expect(list).toHaveAttribute("aria-pressed", "false");
    expect(list).toHaveAttribute("data-active", "false");
  });

  it("Should render the badge text for items with badge prop", () => {
    render(<PillGroup value="list" onChange={() => {}} items={items} />);
    const inbox = screen.getByRole("button", { name: /inbox/i });
    const badge = inbox.querySelector('[data-slot="pill-group-badge"]') as HTMLElement | null;
    expect(badge).not.toBeNull();
    expect(badge?.textContent).toBe("3");
  });

  it("Should not fire onChange for a disabled item", async () => {
    const user = userEvent.setup();
    const handle = vi.fn();
    render(
      <PillGroup
        value="list"
        onChange={handle}
        items={[
          { value: "list", label: "List" },
          { value: "kanban", label: "Kanban", disabled: true },
        ]}
      />
    );

    const kanban = screen.getByRole("button", { name: /kanban/i });
    expect(kanban).toBeDisabled();

    await user.click(kanban);
    expect(handle).not.toHaveBeenCalled();
  });

  it("Should expose testId as data-testid when provided", () => {
    render(
      <PillGroup
        value="list"
        onChange={() => {}}
        items={[{ value: "list", label: "List", testId: "mode-list" }]}
      />
    );
    expect(screen.getByRole("button", { name: /list/i })).toHaveAttribute(
      "data-testid",
      "mode-list"
    );
  });
});

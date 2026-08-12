import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { primarySessionFixture } from "../../testing";
import { SessionRenameDialog } from "../session-rename-dialog";

describe("SessionRenameDialog", () => {
  it("Should discard a stale draft when the same session dialog reopens", () => {
    const props = {
      onOpenChange: vi.fn(),
      session: primarySessionFixture,
      isRenaming: false,
      onConfirm: vi.fn(),
    };
    const { rerender } = render(<SessionRenameDialog {...props} open />);

    fireEvent.change(screen.getByTestId("session-rename-name"), {
      target: { value: "Unsaved draft" },
    });
    rerender(<SessionRenameDialog {...props} open={false} />);
    rerender(<SessionRenameDialog {...props} open />);

    expect(screen.getByTestId("session-rename-name")).toHaveValue(primarySessionFixture.name);
  });

  it("Should submit a valid trimmed name", () => {
    const onConfirm = vi.fn();
    render(
      <SessionRenameDialog
        open
        onOpenChange={vi.fn()}
        session={primarySessionFixture}
        isRenaming={false}
        onConfirm={onConfirm}
      />
    );

    fireEvent.change(screen.getByTestId("session-rename-name"), {
      target: { value: "  Release review  " },
    });
    fireEvent.click(screen.getByTestId("session-rename-confirm"));

    expect(onConfirm).toHaveBeenCalledWith("Release review");
  });

  it("Should show a failed rename while allowing the user to retry", () => {
    render(
      <SessionRenameDialog
        open
        onOpenChange={vi.fn()}
        session={primarySessionFixture}
        isRenaming={false}
        requestError="Session rename failed."
        onConfirm={vi.fn()}
      />
    );

    expect(screen.getByTestId("session-rename-error")).toHaveTextContent("Session rename failed.");
    expect(screen.getByTestId("session-rename-confirm")).toBeEnabled();
  });

  it.each([
    ["blank", "   ", "Enter a session name."],
    ["too long", "a".repeat(65), "Use 64 characters or fewer."],
  ])("Should block a %s name", (_case, name, message) => {
    const onConfirm = vi.fn();
    render(
      <SessionRenameDialog
        open
        onOpenChange={vi.fn()}
        session={primarySessionFixture}
        isRenaming={false}
        onConfirm={onConfirm}
      />
    );

    fireEvent.change(screen.getByTestId("session-rename-name"), { target: { value: name } });

    expect(screen.getByText(message)).toBeInTheDocument();
    expect(screen.getByTestId("session-rename-confirm")).toBeDisabled();
    expect(onConfirm).not.toHaveBeenCalled();
  });
});

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { marketingProfileFixture } from "../../mocks/fixtures";
import { ProfileIdentityDialog } from "../profile-identity-dialog";

describe("ProfileIdentityDialog", () => {
  it("Should update color and symbol without changing the profile name", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(
      <ProfileIdentityDialog
        open
        onOpenChange={vi.fn()}
        profile={marketingProfileFixture}
        isPending={false}
        onSave={onSave}
      />
    );

    const picker = screen.getByTestId("profile-identity-symbol-picker");
    await user.click(screen.getByRole("button", { name: "Emojis" }));
    await user.click(screen.getByRole("option", { name: "seedling" }));
    await user.click(screen.getByTestId("profile-identity-confirm"));

    expect(picker).toBeInTheDocument();
    expect(onSave).toHaveBeenCalledWith({ color: "#c26ad6", emoji: "🌱" });
  });

  it("Should prevent submission while the custom color is invalid", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    render(
      <ProfileIdentityDialog
        open
        onOpenChange={vi.fn()}
        profile={marketingProfileFixture}
        isPending={false}
        onSave={onSave}
      />
    );

    const color = screen.getByLabelText("Custom color");
    await user.clear(color);
    await user.paste("12ZZ");

    expect(screen.getByText("Enter a color like #4ea7fc.")).toBeInTheDocument();
    expect(screen.getByTestId("profile-identity-confirm")).toBeDisabled();
    expect(onSave).not.toHaveBeenCalled();
  });
});

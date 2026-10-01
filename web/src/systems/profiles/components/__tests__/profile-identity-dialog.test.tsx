import type React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

function renderWithClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

import { marketingProfileFixture } from "../../mocks/fixtures";
import { ProfileIdentityDialog } from "../profile-identity-dialog";

const catalog = {
  icons: [{ name: "rocket", label: "rocket", keywords: "launch" }],
  loading: false,
} as const;

describe("ProfileIdentityDialog", () => {
  it("Should update color and symbol without changing the profile name", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    renderWithClient(
      <ProfileIdentityDialog
        catalog={catalog}
        open
        onOpenChange={vi.fn()}
        profile={marketingProfileFixture}
        isPending={false}
        onSave={onSave}
      />
    );

    const picker = screen.getByTestId("profile-identity-symbol-picker");
    await user.type(await screen.findByLabelText("Search icons"), "rocket");
    await user.click(await screen.findByRole("option", { name: "rocket" }));
    await user.click(screen.getByTestId("profile-identity-confirm"));

    expect(picker).toBeInTheDocument();
    expect(onSave).toHaveBeenCalledWith({ color: "#c26ad6", icon: "rocket" });
  });

  it("Should prevent submission while the custom color is invalid", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    renderWithClient(
      <ProfileIdentityDialog
        catalog={catalog}
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

  it("Should close from the header's quiet close button, and not while saving", async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    const { rerender } = renderWithClient(
      <ProfileIdentityDialog
        catalog={catalog}
        open
        onOpenChange={onOpenChange}
        profile={marketingProfileFixture}
        isPending={false}
        onSave={vi.fn()}
      />
    );

    await user.click(screen.getByRole("button", { name: "Close" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);

    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <ProfileIdentityDialog
          catalog={catalog}
          open
          onOpenChange={onOpenChange}
          profile={marketingProfileFixture}
          isPending
          onSave={vi.fn()}
        />
      </QueryClientProvider>
    );
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
  });
});

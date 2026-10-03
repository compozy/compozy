// Suite: profile create dialog
// Invariant: creation rejects blank names and activates the new profile in exactly the selected lens.
// Name validation describes the current input, retaining refusals for their submitted name.
// Owning layer: web/src/systems/profiles/components/profile-create-dialog.tsx.
// Boundary OUT: mutation transport and daemon-side name validation.
import type React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

function renderWithClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

import { ProfileCreateDialog } from "../profile-create-dialog";

const catalog = { icons: [], loading: false } as const;

describe("ProfileCreateDialog", () => {
  it("Should reject a blank profile name locally", async () => {
    const onCreate = vi.fn();
    renderWithClient(
      <ProfileCreateDialog
        catalog={catalog}
        open
        onOpenChange={vi.fn()}
        existingCount={1}
        lens={{ scope: "global" }}
        isPending={false}
        onCreate={onCreate}
      />
    );

    await userEvent.click(screen.getByTestId("profile-create-confirm"));

    expect(screen.getByText("Give the profile a name.")).toBeInTheDocument();
    expect(onCreate).not.toHaveBeenCalled();

    const name = screen.getByRole("textbox", { name: /^Name$/ });
    await userEvent.type(name, "research");
    expect(name).toHaveAttribute("aria-invalid", "false");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("Should keep a server refusal attached to its submitted name", async () => {
    const onCreate = vi.fn();
    renderWithClient(
      <ProfileCreateDialog
        catalog={catalog}
        open
        onOpenChange={vi.fn()}
        existingCount={1}
        lens={{ scope: "global" }}
        isPending={false}
        initialName="default"
        nameError={{ name: "default", message: "This name is reserved." }}
        onCreate={onCreate}
      />
    );
    const name = screen.getByRole("textbox", { name: /^Name$/ });
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("alert")).toBeVisible();

    await userEvent.clear(name);
    await userEvent.type(name, "research");
    expect(name).toHaveAttribute("aria-invalid", "false");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();

    await userEvent.clear(name);
    await userEvent.type(name, "default");
    expect(name).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByRole("alert")).toBeVisible();
    expect(onCreate).not.toHaveBeenCalled();

    await userEvent.clear(name);
    await userEvent.type(name, "research");
    await userEvent.click(screen.getByTestId("profile-create-confirm"));
    expect(onCreate).toHaveBeenCalledExactlyOnceWith(expect.objectContaining({ name: "research" }));
  });

  it("Should keep operation failures visible when the name changes", async () => {
    const onCreate = vi.fn();
    renderWithClient(
      <ProfileCreateDialog
        catalog={catalog}
        open
        onOpenChange={vi.fn()}
        existingCount={1}
        lens={{ scope: "global" }}
        isPending={false}
        initialName="research"
        error="Unable to reach the server."
        onCreate={onCreate}
      />
    );

    const name = screen.getByRole("textbox", { name: /^Name$/ });
    await userEvent.clear(name);
    await userEvent.type(name, "editorial");
    expect(name).toHaveAttribute("aria-invalid", "false");
    expect(screen.getByRole("alert")).toHaveTextContent("Unable to reach the server.");
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("Should keep creation disabled while a request is pending after a name edit", async () => {
    const onCreate = vi.fn();
    renderWithClient(
      <ProfileCreateDialog
        catalog={catalog}
        open
        onOpenChange={vi.fn()}
        existingCount={1}
        lens={{ scope: "global" }}
        isPending
        initialName="research"
        onCreate={onCreate}
      />
    );

    await userEvent.type(screen.getByRole("textbox", { name: /^Name$/ }), "-notes");
    const create = screen.getByTestId("profile-create-confirm");
    expect(create).toBeDisabled();
    await userEvent.click(create);
    expect(onCreate).not.toHaveBeenCalled();
  });

  it("Should trim the name and activate a workspace profile", async () => {
    const onCreate = vi.fn();
    renderWithClient(
      <ProfileCreateDialog
        catalog={catalog}
        open
        onOpenChange={vi.fn()}
        existingCount={1}
        lens={{ scope: "workspace", workspaceId: "workspace:alpha" }}
        isPending={false}
        initialName="  marketing  "
        onCreate={onCreate}
      />
    );

    await userEvent.click(screen.getByTestId("profile-create-confirm"));

    expect(onCreate).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        name: "marketing",
        activate: {
          scope: "workspace",
          profile: "marketing",
          workspace_id: "workspace:alpha",
        },
      })
    );
  });

  it("Should activate a global profile without a workspace identifier", async () => {
    const onCreate = vi.fn();
    renderWithClient(
      <ProfileCreateDialog
        catalog={catalog}
        open
        onOpenChange={vi.fn()}
        existingCount={2}
        lens={{ scope: "global" }}
        isPending={false}
        initialName="research"
        onCreate={onCreate}
      />
    );

    await userEvent.click(screen.getByTestId("profile-create-confirm"));

    expect(onCreate).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        name: "research",
        activate: { scope: "global", profile: "research" },
      })
    );
  });
});

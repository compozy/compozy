// Suite: profile create dialog
// Invariant: creation rejects blank names and activates the new profile in exactly the selected lens.
// Owning layer: web/src/systems/profiles/components/profile-create-dialog.tsx.
// Boundary OUT: mutation transport and daemon-side name validation.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ProfileCreateDialog } from "../profile-create-dialog";

describe("ProfileCreateDialog", () => {
  it("Should reject a blank profile name locally", async () => {
    const onCreate = vi.fn();
    render(
      <ProfileCreateDialog
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
  });

  it("Should trim the name and activate a workspace profile", async () => {
    const onCreate = vi.fn();
    render(
      <ProfileCreateDialog
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
    render(
      <ProfileCreateDialog
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

import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Avatar, AvatarBadge, AvatarFallback, AvatarGroup, AvatarImage } from "../avatar";

describe("Avatar", () => {
  it("Should render the fallback initials when no image is provided", () => {
    render(
      <Avatar>
        <AvatarFallback>PN</AvatarFallback>
      </Avatar>
    );
    expect(screen.getByText("PN")).toBeInTheDocument();
  });

  it("Should expose the requested size via data-size", () => {
    const { container } = render(
      <Avatar size="lg">
        <AvatarFallback>AR</AvatarFallback>
      </Avatar>
    );
    const root = container.querySelector('[data-slot="avatar"]');
    expect(root?.getAttribute("data-size")).toBe("lg");
  });

  it("Should render the image slot when the avatar image loads successfully", async () => {
    const OriginalImage = window.Image;

    class LoadedImageMock {
      onload: null | (() => void) = null;
      onerror: null | (() => void) = null;
      complete = false;
      naturalWidth = 0;

      set src(_value: string) {
        this.complete = true;
        this.naturalWidth = 64;
        this.onload?.();
      }
    }

    window.Image = LoadedImageMock as unknown as typeof Image;

    try {
      render(
        <Avatar>
          <AvatarImage src="https://example.test/me.png" alt="me" />
          <AvatarFallback>ME</AvatarFallback>
        </Avatar>
      );

      await waitFor(() =>
        expect(screen.getByRole("img", { name: "me" })).toHaveAttribute("data-slot", "avatar-image")
      );
    } finally {
      window.Image = OriginalImage;
    }
  });

  it("Should render AvatarBadge as a positioned child", () => {
    const { container } = render(
      <Avatar size="lg">
        <AvatarFallback>PN</AvatarFallback>
        <AvatarBadge data-testid="badge" />
      </Avatar>
    );
    const badge = container.querySelector('[data-slot="avatar-badge"]');
    expect(badge).not.toBeNull();
  });

  it("Should render an AvatarGroup container for nested avatars", () => {
    const { container } = render(
      <AvatarGroup>
        <Avatar>
          <AvatarFallback>A</AvatarFallback>
        </Avatar>
        <Avatar>
          <AvatarFallback>B</AvatarFallback>
        </Avatar>
      </AvatarGroup>
    );
    const group = container.querySelector('[data-slot="avatar-group"]');
    expect(group).not.toBeNull();
    expect(group?.querySelectorAll('[data-slot="avatar"]').length).toBe(2);
  });

  it("Should default to circle shape", () => {
    const { container } = render(
      <Avatar>
        <AvatarFallback>PN</AvatarFallback>
      </Avatar>
    );
    const root = container.querySelector('[data-slot="avatar"]');
    expect(root?.getAttribute("data-shape")).toBe("circle");
  });

  it("Should reflect data-shape=square when shape is square", () => {
    const { container } = render(
      <Avatar shape="square">
        <AvatarFallback>AG</AvatarFallback>
      </Avatar>
    );
    const root = container.querySelector('[data-slot="avatar"]');
    expect(root?.getAttribute("data-shape")).toBe("square");
  });
});

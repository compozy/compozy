import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Avatar, AvatarFallback, AvatarImage } from "../avatar";

describe("Avatar", () => {
  it("Should render the fallback initials when no image is provided", () => {
    render(
      <Avatar>
        <AvatarFallback>PN</AvatarFallback>
      </Avatar>
    );
    expect(screen.getByText("PN")).toBeInTheDocument();
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
});

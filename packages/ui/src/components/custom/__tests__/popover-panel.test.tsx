import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { KeyRound } from "../../../icons";
import { Popover } from "../../popover";
import { KindIcon } from "../kind-icon";
import { PopoverPanel, PopoverPanelContent, type PopoverPanelProps } from "../popover-panel";

type PanelOverrides = Omit<Partial<PopoverPanelProps>, "icon" | "mark"> & {
  mark?: React.ReactNode;
};

function renderPanel({ mark, ...props }: PanelOverrides = {}) {
  const identity = mark === undefined ? { icon: KeyRound } : { mark };
  return render(
    <Popover open modal={false}>
      <PopoverPanelContent>
        <PopoverPanel {...identity} title="Sign in to linear" {...props}>
          body
        </PopoverPanel>
      </PopoverPanelContent>
    </Popover>
  );
}

describe("PopoverPanel", () => {
  it("Should name the popup with its title and describe it with the subtitle", () => {
    renderPanel({ subtitle: "linear.app" });

    const dialog = screen.getByRole("dialog", { name: "Sign in to linear" });
    expect(dialog).toHaveAccessibleDescription("linear.app");
  });

  it("Should rule the aside and footer after the body, in order", () => {
    renderPanel({ aside: "fallback", footer: <button type="button">Done</button> });

    const body = document.querySelector('[data-slot="popover-panel-body"]');
    const aside = document.querySelector('[data-slot="popover-panel-aside"]');
    const footer = document.querySelector('[data-slot="popover-panel-footer"]');
    expect(body?.textContent).toBe("body");
    expect(body?.nextElementSibling).toBe(aside);
    expect(aside?.nextElementSibling).toBe(footer);
  });

  it("Should offer Close only when the host can close it", () => {
    const onClose = vi.fn();
    const { unmount } = renderPanel({ onClose });
    fireEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalledOnce();
    unmount();

    renderPanel();
    expect(screen.queryByRole("button", { name: "Close" })).not.toBeInTheDocument();
  });

  it("Should replace the well with a mark when one is given, with no glyph required", () => {
    renderPanel({
      mark: <KindIcon tone="well" glyph={<svg data-testid="mark" />} aria-hidden="true" />,
      tone: "success",
    });
    expect(screen.getByTestId("mark")).toBeInTheDocument();
    expect(document.querySelector('[data-tone="success"]')).toBeNull();
  });
});

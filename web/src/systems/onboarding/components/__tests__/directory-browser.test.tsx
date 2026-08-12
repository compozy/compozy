import { fireEvent, render, screen } from "@testing-library/react";
import { UIProvider } from "@compozy/ui";
import { describe, expect, it, vi } from "vitest";

import { DirectoryBrowser } from "../directory-browser";

describe("DirectoryBrowser", () => {
  it("forwards native div attributes and navigates to an operating-system root", () => {
    const onClick = vi.fn();
    const onNavigate = vi.fn();
    render(
      <UIProvider reducedMotion="always">
        <DirectoryBrowser
          aria-label="Workspace directory browser"
          browseError={null}
          currentPath="/workspace"
          entries={[]}
          homePath="/workspace"
          isBrowsing={false}
          isPicked={() => false}
          onClick={onClick}
          onGoHome={() => undefined}
          onGoParent={() => undefined}
          onNavigate={onNavigate}
          onPick={() => undefined}
          parentPath={null}
          roots={["/"]}
          title="Choose a workspace directory"
        />
      </UIProvider>
    );

    const browser = screen.getByTestId("directory-browser");
    expect(browser).toHaveAccessibleName("Workspace directory browser");
    expect(browser).toHaveAttribute("title", "Choose a workspace directory");
    fireEvent.click(browser);
    expect(onClick).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByRole("button", { name: "Go to location /" }));
    expect(onNavigate).toHaveBeenCalledWith("/");
  });
});

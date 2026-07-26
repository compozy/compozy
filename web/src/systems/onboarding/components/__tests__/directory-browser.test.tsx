import { fireEvent, render, screen } from "@testing-library/react";
import { UIProvider } from "@agh/ui";
import { describe, expect, it, vi } from "vitest";

import { DirectoryBrowser } from "../directory-browser";

describe("DirectoryBrowser", () => {
  it("forwards native div attributes and events from its root wrapper", () => {
    const onClick = vi.fn();
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
          onNavigate={() => undefined}
          onPick={() => undefined}
          parentPath={null}
          title="Choose a workspace directory"
        />
      </UIProvider>
    );

    const browser = screen.getByTestId("directory-browser");
    expect(browser).toHaveAccessibleName("Workspace directory browser");
    expect(browser).toHaveAttribute("title", "Choose a workspace directory");
    fireEvent.click(browser);
    expect(onClick).toHaveBeenCalledOnce();
  });
});

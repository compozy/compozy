import { fireEvent, render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import { UIProvider } from "@compozy/ui";
import { describe, expect, it, vi } from "vitest";

import { browseDirectory } from "../../adapters/onboarding-api";
import { useWorkspaceSetupContent } from "@/systems/workspace/hooks/use-workspace-setup-content";

import { DirectoryBrowser } from "../directory-browser";

vi.mock("../../adapters/onboarding-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/onboarding-api")>()),
  browseDirectory: vi.fn(),
}));

function BrowserHarness() {
  const setup = useWorkspaceSetupContent({ onWorkspaceResolved: () => undefined });
  const { navigateTo, goToParent, goHome, ...view } = setup.browse;
  return (
    <DirectoryBrowser
      {...view}
      isPicked={path => path === setup.draft.rootDir}
      onGoHome={goHome}
      onGoParent={goToParent}
      onNavigate={navigateTo}
      onPick={setup.selectRoot}
    />
  );
}

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

  it.each([
    { root: "/", home: "/Users/operator", separator: "/" },
    { root: "C:\\", home: "C:\\Users\\operator", separator: "\\" },
  ])("keeps known recovery destinations after a failed browse from $home", async paths => {
    const project = `${paths.home}${paths.separator}project`;
    const restricted = `${project}${paths.separator}restricted`;
    vi.mocked(browseDirectory).mockImplementation(async query => {
      const path = query.path ?? paths.home;
      if (path === restricted) throw new Error("permission denied");
      return {
        path,
        home: paths.home,
        parent: path === project ? paths.home : path === paths.root ? undefined : paths.root,
        roots: [paths.root],
        entries:
          path === project
            ? [{ name: "restricted", path: restricted, is_dir: true }]
            : [{ name: "project", path: project, is_dir: true }],
      };
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const user = userEvent.setup();
    render(
      <QueryClientProvider client={client}>
        <UIProvider reducedMotion="always">
          <BrowserHarness />
        </UIProvider>
      </QueryClientProvider>
    );

    await user.click(await screen.findByRole("button", { name: "project" }));
    for (const destination of [
      "Go to parent directory",
      "Go to home directory",
      `Go to location ${paths.root}`,
    ]) {
      await user.click(await screen.findByRole("button", { name: "restricted" }));
      expect(await screen.findByRole("alert")).toHaveTextContent("permission denied");
      expect(screen.getByTitle(restricted)).toBeVisible();
      expect(screen.queryByRole("button", { name: "project" })).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: destination })).toBeEnabled();
      await user.click(screen.getByRole("button", { name: destination }));
      if (destination === "Go to parent directory") {
        expect(await screen.findByRole("button", { name: "restricted" })).toBeVisible();
      } else {
        const projectButton = await screen.findByRole("button", { name: "project" });
        expect(screen.queryByRole("alert")).not.toBeInTheDocument();
        if (destination === "Go to home directory") await user.click(projectButton);
      }
    }
    expect(screen.getByRole("button", { name: "Go to parent directory" })).toBeDisabled();
    expect(screen.getByTitle(paths.root)).toBeVisible();
  });
});

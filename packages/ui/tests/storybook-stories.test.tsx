// @vitest-environment jsdom

import { cleanup, screen } from "@testing-library/react";
import { composeStories, setProjectAnnotations } from "@storybook/react-vite";
import { afterEach, describe, expect, it } from "vitest";

import * as preview from "../.storybook/preview";
import * as appShellStories from "../src/components/stories/app-shell.stories";
import * as buttonStories from "../src/components/stories/button.stories";
import * as sectionHeadingStories from "../src/components/stories/section-heading.stories";
import * as statusBadgeStories from "../src/components/stories/status-badge.stories";
import * as surfaceCardStories from "../src/components/stories/surface-card.stories";

setProjectAnnotations(preview);

const { Primary: ButtonPrimary } = composeStories(buttonStories);
const { Default: AppShellDefault } = composeStories(appShellStories);
const { Full: SectionHeadingFull } = composeStories(sectionHeadingStories);
const { ToneMatrix: StatusBadgeToneMatrix } = composeStories(statusBadgeStories);
const { Default: SurfaceCardDefault } = composeStories(surfaceCardStories);

afterEach(() => {
  cleanup();
  document.body.innerHTML = "";
});

describe("packages/ui portable stories", () => {
  it("renders the public button story", async () => {
    await ButtonPrimary.run();

    expect(await screen.findByText("Sync all workflows")).toBeInTheDocument();
  });

  it("renders the shared shell story", async () => {
    await AppShellDefault.run();

    expect(await screen.findByText("Compozy")).toBeInTheDocument();
    expect(screen.getByText("Operator dashboard")).toBeInTheDocument();
  });

  it("renders the section heading story", async () => {
    await SectionHeadingFull.run();

    expect(await screen.findByText("Operator dashboard")).toBeInTheDocument();
    expect(
      screen.getByText("Inspect queue health, runs, and review pressure from one workspace.")
    ).toBeInTheDocument();
  });

  it("renders the status badge tone matrix story", async () => {
    await StatusBadgeToneMatrix.run();

    expect(await screen.findByText("running")).toBeInTheDocument();
    expect(screen.getByText("failed")).toBeInTheDocument();
  });

  it("renders the surface card story", async () => {
    await SurfaceCardDefault.run();

    expect(await screen.findByText("Run queue")).toBeInTheDocument();
    expect(screen.getByText("updated just now")).toBeInTheDocument();
  });
});

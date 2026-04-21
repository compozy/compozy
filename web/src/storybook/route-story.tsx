import type { Meta } from "@storybook/react-vite";
import { useEffect } from "react";

import { resetActiveWorkspaceStoreForTests, useActiveWorkspaceStore } from "@/systems/app-shell";
import { setRunStreamFactoryOverrideForTests, type RunStreamFactory } from "@/systems/runs";

export function StorybookRouteCanvas() {
  return null;
}

export function createRouteStoryMeta(
  title: string,
  description: string
): Meta<typeof StorybookRouteCanvas> {
  return {
    title,
    component: StorybookRouteCanvas,
    parameters: {
      layout: "fullscreen",
      docs: {
        description: {
          component: description,
        },
      },
    },
  };
}

export function appRouteParameters(path: string) {
  return {
    layout: "fullscreen" as const,
    router: {
      kind: "app" as const,
      initialEntries: [path],
    },
  };
}

export function StorybookWorkspaceSetup({
  workspaceId = "ws-storybook",
}: {
  workspaceId?: string;
}) {
  useEffect(() => {
    useActiveWorkspaceStore.setState({ selectedWorkspaceId: workspaceId });
    return () => {
      resetActiveWorkspaceStoreForTests();
    };
  }, [workspaceId]);

  return null;
}

export function StorybookRunStreamSetup({ factory }: { factory: RunStreamFactory | null }) {
  useEffect(() => {
    setRunStreamFactoryOverrideForTests(factory);
    return () => {
      setRunStreamFactoryOverrideForTests(null);
    };
  }, [factory]);

  return null;
}
